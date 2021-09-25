package notifiarr

import (
	"fmt"
	"reflect"
	"time"

	"github.com/Notifiarr/notifiarr/pkg/ui"
	"golift.io/cnfg"
)

const (
	stuckDur = 5*time.Minute + 1327*time.Millisecond
	pollDur  = 4*time.Minute + 47*time.Second + 977*time.Millisecond
)

// timerConfig defines a custom GET timer from the website.
// Used to offload crons to clients.
type timerConfig struct {
	Name     string        `json:"name"`     // name of action.
	Interval cnfg.Duration `json:"interval"` // how often to GET this URI.
	URI      string        `json:"endpoint"` // endpoint for the URI.
	Desc     string        `json:"description"`
	ch       chan EventType
	getdata  func(string) (*Response, error)
	errorf   func(string, ...interface{})
}

type action struct {
	Fn  func(EventType)           // most actions use this
	SFn func(map[string]struct{}) // this is just for plex sessions.
	Msg string                    // msg is printed if provided, otherwise ignored.
	C   chan EventType            // if provided, T is optional
	T   *time.Ticker              // if provided, C is optional.
}

// Run fires a custom cron timer (GET).
func (t *timerConfig) Run(event EventType) {
	if t.ch == nil {
		return
	}

	t.ch <- event
}

func (t *timerConfig) run(event EventType) {
	if _, err := t.getdata(t.URI); err != nil {
		t.errorf("[%s requested] Custom Timer Request for %s failed: %v", event, t.URI, err)
	}
}

func (t *timerConfig) setup(c *Config) {
	t.URI, t.errorf, t.getdata = c.BaseURL+"/"+t.URI, c.Errorf, c.GetData
	t.ch = make(chan EventType)
}

func (c *Config) startTimers() {
	var (
		_, err  = c.GetClientInfo(EventStart)
		actions = c.getClientInfoTimers(err == nil) // sync, gaps, dashboard, custom
		plex    = c.getPlexTimers()
		cases   = []reflect.SelectCase{}
		combine = []*action{}
		timer   int
		trigger int
	)

	if c.Snap.Interval.Duration > 0 {
		c.Trigger.snap.T = time.NewTicker(c.Snap.Interval.Duration)
		c.logSnapshotStartup()
	}

	if c.ClientInfo == nil || c.ClientInfo.Actions.Poll {
		c.Printf("==> Started Notifiarr Poller, (nil=%v) interval: %v, timeout: %v", c.ClientInfo == nil, pollDur, c.Timeout)
		actions = append(actions, //nolint:wsl
			&action{Msg: "Polling Notifiarr for new settings.", Fn: c.pollForReload, T: time.NewTicker(pollDur)})
	}

	for _, t := range append(actions,
		plex, c.Trigger.snap, c.Trigger.gaps, c.Trigger.plex,
		c.Trigger.dash, c.Trigger.stop, c.Trigger.sync, c.Trigger.stuck) {
		if t == nil {
			continue
		}

		// Since we may add up to 2 actions per list item, duplicate the pointer in a new combined list.
		if t.C != nil {
			cases = append(cases, reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(t.C)})
			combine = append(combine, t)
			trigger++
		}

		if t.T != nil {
			cases = append(cases, reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(t.T.C)})
			combine = append(combine, t)
			timer++
		}
	}

	go c.runTimerLoop(combine, cases)
	c.Printf("==> Started %d Notifiarr Timers with %d Triggers", timer, trigger)
}

func (c *Config) getClientInfoTimers(haveInfo bool) []*action {
	if !haveInfo {
		return nil
	}

	if c.Actions.Gaps.Interval.Duration > 0 {
		c.Trigger.gaps.T = time.NewTicker(c.Actions.Gaps.Interval.Duration)
		c.Printf("==> Collection Gaps Timer Enabled, interval: %s", c.Actions.Gaps.Interval)
	}

	if c.Actions.Sync.Interval.Duration > 0 {
		c.Trigger.sync.T = time.NewTicker(c.Actions.Sync.Interval.Duration)
		c.Printf("==> Keeping %d Radarr Custom Formats and %d Sonarr Release Profiles synced, interval: %s",
			c.Actions.Sync.Radarr, c.Actions.Sync.Sonarr, c.Actions.Sync.Interval)
	}

	if c.Actions.Dashboard.Interval.Duration > 0 {
		c.Trigger.dash.T = time.NewTicker(c.Actions.Dashboard.Interval.Duration)
		c.Printf("==> Sending Current State Data for Dashboard every %s", c.Actions.Dashboard.Interval)
	}

	if len(c.Actions.Custom) > 0 { // This is not directly triggable.
		c.Printf("==> Custom Timers Enabled: %d timers provided", len(c.Actions.Custom))
	}

	customActions := []*action{}

	for _, custom := range c.Actions.Custom {
		custom.setup(c)

		var ticker *time.Ticker

		if custom.Interval.Duration < time.Minute {
			c.Errorf("Website provided custom cron interval under 1 minute. Ignored! Interval: %s Name: %s, URI: %s",
				custom.Interval, custom.Name, custom.URI)
		} else {
			ticker = time.NewTicker(custom.Interval.Duration)
		}

		customActions = append(customActions, &action{
			Fn:  custom.run,
			C:   custom.ch,
			Msg: fmt.Sprintf("Running Custom Cron Timer '%s' GET %s", custom.Name, custom.URI),
			T:   ticker,
		})
	}

	return customActions
}

func (c *Config) getPlexTimers() *action {
	if !c.Plex.Configured() {
		return nil
	}

	if c.Plex.Interval.Duration > 0 {
		// Add a little splay to the timers to not hit plex at the same time too often.
		c.Printf("==> Plex Sessions Collection Started, URL: %s, interval: %v, timeout: %v, webhook cooldown: %v",
			c.Plex.URL, c.Plex.Interval, c.Plex.Timeout, c.Plex.Cooldown)
		c.Trigger.plex.T = time.NewTicker(c.Plex.Interval.Duration + 139*time.Millisecond) // nolint:wsl
	}

	if c.Plex.MoviesPC != 0 || c.Plex.SeriesPC != 0 {
		c.Printf("==> Plex Completed Items Started, URL: %s, interval: 1m, timeout: %v movies: %d%%, series: %d%%",
			c.Plex.URL, c.Plex.Timeout, c.Plex.MoviesPC, c.Plex.SeriesPC)

		return &action{SFn: c.checkPlexFinishedItems, T: time.NewTicker(time.Minute + 179*time.Millisecond)}
	}

	return nil
}

// runTimerLoop does all of the timer/cron routines for starr apps and plex.
// Many of the menu items and trigger handlers feed into this routine too.
func (c *Config) runTimerLoop(actions []*action, cases []reflect.SelectCase) {
	defer c.stopTimerLoop(actions)

	for sent := make(map[string]struct{}); ; {
		index, val, _ := reflect.Select(cases)

		action := actions[index]
		if action.Fn == nil && action.SFn == nil { // stop channel has no Functions
			return
		}

		var event EventType
		if _, ok := val.Interface().(time.Time); ok {
			event = EventCron
		} else if event, ok = val.Interface().(EventType); !ok {
			event = "unknown"
		}

		if event == EventUser {
			if err := ui.Notify(action.Msg); err != nil {
				c.Errorf("Displaying toast notification: %v", err)
			}
		}

		if action.Msg != "" {
			c.Printf("[%s requested] %s", event, action.Msg)
		}

		if action.Fn != nil {
			action.Fn(event)
		} else {
			action.SFn(sent)
		}
	}
}

// stopTimerLoop is defered by runTimerLoop.
func (c *Config) stopTimerLoop(actions []*action) {
	defer c.CapturePanic()
	c.Trigger.stop = nil

	for _, action := range actions {
		if action.C != nil {
			close(action.C)
			action.C = nil
		}

		if action.T != nil {
			action.T.Stop()
			action.T = nil
		}
	}
}
