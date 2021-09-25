// Package notifiarr provides a standard interface for sending data to notifiarr.com.
// Several methods are exported to make POSTing data to notifarr easier. This package
// also handles the incoming Plex webhook as well as the "crontab" timers for plex
// sessions, snapshots, dashboard state, custom format sync for Radarr and release
// profile sync for Sonarr.
// This package's cofiguration is provided by the configfile  package.
package notifiarr

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Notifiarr/notifiarr/pkg/apps"
	"github.com/Notifiarr/notifiarr/pkg/logs"
	"github.com/Notifiarr/notifiarr/pkg/plex"
	"github.com/Notifiarr/notifiarr/pkg/snapshot"
	"github.com/shirou/gopsutil/v3/host"
)

// Errors returned by this library.
var (
	ErrNon200          = fmt.Errorf("return code was not 200")
	ErrInvalidResponse = fmt.Errorf("invalid response")
)

// Route is used to give us methods on our route paths.
type Route string

// Notifiarr URLs. Data sent to these URLs:
/*
api/v1/notification/plex?event=...
  api (was plexcron)
  user (was plexcron)
  cron (was plexcron)
  webhook (was plexhook)
  movie
  episode

api/v1/notification/services?event=...
  api
  user
  cron
  start (only fires on startup)

api/v1/notification/snapshot?event=...
  api
  user
  cron

api/v1/notification/dashboard?event=... (requires interval from website/client endpoint)
  api
  user
  cron

api/v1/notification/stuck?event=...
  api
  user
  cron

api/v1/user/gaps?app=radarr&event=...
  api
  user
  cron

api/v2/user/client?event=start
  see description https://github.com/Notifiarr/notifiarr/pull/115

api/v1/user/trash?app=...
  radarr
  sonarr
*/
const (
	BaseURL           = "https://notifiarr.com"
	DevBaseURL        = "https://dev.notifiarr.com"
	userRoute1  Route = "/api/v1/user"
	userRoute2  Route = "/api/v2/user"
	ClientRoute Route = userRoute2 + "/client"
	CFSyncRoute Route = userRoute1 + "/trash"
	GapsRoute   Route = userRoute1 + "/gaps"
	notifiRoute Route = "/api/v1/notification"
	DashRoute   Route = notifiRoute + "/dashboard"
	StuckRoute  Route = notifiRoute + "/stuck"
	PlexRoute   Route = notifiRoute + "/plex"
	SnapRoute   Route = notifiRoute + "/snapshot"
	SvcRoute    Route = notifiRoute + "/services"
)

const (
	ModeDev  = "development"
	ModeProd = "production"
)

// EventType identifies the type of event that sent a paylaod to notifiarr.
type EventType string

// These are all our known event types.
const (
	EventCron    EventType = "cron"
	EventUser    EventType = "user"
	EventAPI     EventType = "api"
	EventHook    EventType = "webhook"
	EventStart   EventType = "start"
	EventMovie   EventType = "movie"
	EventEpisode EventType = "episode"
	EventPoll    EventType = "poll"
	EventReload  EventType = "reload"
)

// Path adds parameter to a route path and turns it into a string.
func (r Route) Path(event EventType, params ...string) string {
	switch {
	case len(params) == 0 && event == "":
		return string(r)
	case len(params) == 0:
		return string(r) + "?event=" + string(event)
	case event == "":
		return string(r) + "?" + strings.Join(params, "&")
	default:
		return string(r) + "?" + strings.Join(append(params, "event="+string(event)), "&")
	}
}

const (
	// DefaultRetries is the number of times to attempt a request to notifiarr.com.
	// 4 means 5 total tries: 1 try + 4 retries.
	DefaultRetries = 4
	// RetryDelay is how long to Sleep between retries.
	RetryDelay = 222 * time.Millisecond
)

// success is a ssuccessful tatus message from notifiarr.com.
const success = "success"

// Payload is the outbound payload structure that is sent to Notifiarr for Plex and system snapshot data.
type Payload struct {
	Plex *plex.Sessions       `json:"plex,omitempty"`
	Snap *snapshot.Snapshot   `json:"snapshot,omitempty"`
	Load *plexIncomingWebhook `json:"payload,omitempty"`
}

// Config is the input data needed to send payloads to notifiarr.
type Config struct {
	Apps     *apps.Apps       // has API key
	Plex     *plex.Server     // plex sessions
	Snap     *snapshot.Config // system snapshot data
	Services *ServiceConfig
	Retries  int
	BaseURL  string
	Timeout  time.Duration
	Trigger  Triggers
	MaxBody  int
	Sighup   chan os.Signal

	*logs.Logger // log file writer
	extras
}

type extras struct {
	ciMutex sync.Mutex
	*ClientInfo
	client    *httpClient
	radarrCF  map[int]*cfMapIDpayload
	sonarrRP  map[int]*cfMapIDpayload
	plexTimer *Timer
	hostInfo  *host.InfoStat
}

// Triggers allow trigger actions in the timer routine.
type Triggers struct {
	stop  *action        // Triggered by calling Stop()
	sync  *action        // Sync Radarr CF and Sonarr RP
	gaps  *action        // Send Radarr Collection Gaps
	stuck *action        // Stuck Items
	plex  *action        // Send Plex Sessions
	dash  *action        // Dashboard State
	snap  *action        // Snapshot
	sess  chan time.Time // Return Plex Sessions
	sessr chan *holder   // Session Return Channel
}

// Start (and log) snapshot and plex cron jobs if they're configured.
func (c *Config) Setup(mode string) string {
	switch strings.ToLower(mode) {
	default:
		c.BaseURL = BaseURL
	case "prod", ModeProd:
		c.BaseURL = BaseURL
		mode = ModeProd
	case "dev", "devel", ModeDev, "test", "testing":
		c.BaseURL = DevBaseURL
		mode = ModeDev
	}

	if c.Retries < 0 {
		c.Retries = 0
	} else if c.Retries == 0 {
		c.Retries = DefaultRetries
	}

	if c.extras.client == nil {
		c.extras.client = &httpClient{
			Retries: c.Retries,
			Logger:  c.ErrorLog,
			Client:  &http.Client{},
		}
	}

	return mode
}

// Start runs the timers.
func (c *Config) Start() {
	if c.Trigger.stop != nil {
		panic("notifiarr timers cannot run twice")
	}

	c.Trigger.stuck = &action{
		Fn:  c.sendStuckQueueItems,
		Msg: "Checking app queues and sending stuck items.",
		C:   make(chan EventType, 1),
		T:   time.NewTicker(stuckDur),
	}
	c.Trigger.plex = &action{
		Fn:  c.sendPlexSessions,
		Msg: "Gathering and sending Plex Sessions.",
		C:   make(chan EventType, 1),
	}
	c.Trigger.gaps = &action{
		Fn:  c.sendGaps,
		Msg: "Sending Radarr Collection Gaps.",
		C:   make(chan EventType, 1),
	}
	c.Trigger.sync = &action{
		Fn:  c.syncCF,
		Msg: "Starting Custom Formats and Quality Profiles Sync for Radarr and Sonarr.",
		C:   make(chan EventType, 1),
	}
	c.Trigger.dash = &action{
		Fn:  c.sendDashboardState,
		Msg: "Initiating State Collection for Dashboard.",
		C:   make(chan EventType, 1),
	}
	c.Trigger.snap = &action{
		Fn:  c.sendSnapshot,
		Msg: "Gathering and sending System Snapshot.",
		C:   make(chan EventType, 1),
	}
	c.Trigger.stop = &action{
		Msg: "Stop Channel is used for reloads and must not have a function.",
		C:   make(chan EventType),
	}
	c.Trigger.sess = make(chan time.Time, 1)
	c.extras.radarrCF = make(map[int]*cfMapIDpayload)
	c.extras.sonarrRP = make(map[int]*cfMapIDpayload)
	c.extras.plexTimer = &Timer{}

	go c.runSessionHolder()
	c.startTimers()
}

// Stop all internal cron timers and Triggers.
func (c *Config) Stop(event EventType) {
	if c == nil {
		return
	}

	c.Print("==> Stopping Notifiarr Timers.")

	if c.Trigger.stop == nil {
		c.Error("==> Notifiarr Timers cannot be stopped: not running!")
		return
	}

	c.Trigger.stop.C <- event
	defer close(c.Trigger.sess)
	c.Trigger.sess = nil
}
