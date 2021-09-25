package notifiarr

import (
	"strings"
	"sync"
	"time"

	"golift.io/cnfg"
	"golift.io/starr/sonarr"
)

/* This file contains the procedures to send stuck download queue items to notifiarr. */

const (
	errorstr  = "error"
	failed    = "failed"
	warning   = "warning"
	completed = "completed"
)

type appConfig struct {
	Instance int           `json:"instance"`
	Name     string        `json:"name"`
	Stuck    bool          `json:"stuck"`
	Interval cnfg.Duration `json:"interval"`
}

// appConfigs is the configuration returned from the notifiarr website.
type appConfigs struct {
	Lidarr  []*appConfig `json:"lidarr"`
	Radarr  []*appConfig `json:"radarr"`
	Readarr []*appConfig `json:"readarr"`
	Sonarr  []*appConfig `json:"sonarr"`
}

type ListItem struct {
	Elapsed time.Duration `json:"elapsed"`
	Name    string        `json:"name"`
	Queue   []interface{} `json:"queue"`
}

type ItemList map[int]ListItem

type QueuePayload struct {
	Lidarr  ItemList `json:"lidarr,omitempty"`
	Radarr  ItemList `json:"radarr,omitempty"`
	Readarr ItemList `json:"readarr,omitempty"`
	Sonarr  ItemList `json:"sonarr,omitempty"`
}

const getItemsMax = 100

func (i ItemList) Len() (count int) {
	for _, v := range i {
		count += len(v.Queue)
	}

	return count
}

func (i ItemList) Empty() bool {
	return i.Len() < 1
}

func (t *Triggers) SendStuckQueueItems(event EventType) {
	if t.stop == nil {
		return
	}

	t.stuck.C <- event
}

func (c *Config) sendStuckQueueItems(event EventType) {
	start := time.Now()
	q := c.getQueues()
	apps := time.Since(start).Round(time.Millisecond)

	if q == nil || (q.Lidarr.Empty() && q.Radarr.Empty() && q.Readarr.Empty() && q.Sonarr.Empty()) {
		c.Printf("[%s requested] No stuck items found to send to Notifiarr.", event)
		return
	}

	resp, err := c.SendData(StuckRoute.Path(event), q, true)
	elapsed := time.Since(start).Round(time.Millisecond)

	if err != nil {
		c.Errorf("[%s requested] Sending Stuck Queue Items "+
			"(apps:%s total:%s) (Lidarr: %d, Radarr: %d, Readarr: %d, Sonarr: %d): %v",
			event, apps, elapsed, q.Lidarr.Len(), q.Radarr.Len(), q.Readarr.Len(), q.Sonarr.Len(), err)
	} else {
		c.Printf("[%s requested] Sent Stuck Items to Notifiarr "+
			"(apps:%s total:%s): Lidarr: %d, Radarr: %d, Readarr: %d, Sonarr: %d."+
			"Website took %s and replied with: %s, %s",
			event, apps, elapsed, q.Lidarr.Len(), q.Radarr.Len(), q.Readarr.Len(), q.Sonarr.Len(),
			resp.Message.Elapsed, resp.Status, resp.Message.Response)
	}
}

// getQueues fires a routine for each app type and tries to get a lot of data fast!
func (c *Config) getQueues() *QueuePayload {
	q := &QueuePayload{}

	var wg sync.WaitGroup

	wg.Add(4) //nolint:gomnd // 4 is 1 for each app polled.

	go func() {
		q.Lidarr = c.getFinishedItemsLidarr()
		wg.Done() //nolint:wsl
	}()
	go func() {
		q.Radarr = c.getFinishedItemsRadarr()
		wg.Done() //nolint:wsl
	}()
	go func() {
		q.Readarr = c.getFinishedItemsReadarr()
		wg.Done() //nolint:wsl
	}()
	go func() {
		q.Sonarr = c.getFinishedItemsSonarr()
		wg.Done() //nolint:wsl
	}()
	wg.Wait()

	return q
}

func (c *Config) getFinishedItemsLidarr() ItemList {
	stuck := make(ItemList)

	for i, l := range c.Apps.Lidarr {
		instance := i + 1

		if !l.StuckItem {
			continue
		}

		start := time.Now()

		queue, err := l.GetQueue(getItemsMax)
		if err != nil {
			c.Errorf("Getting Lidarr Queue (%d): %v", instance, err)
			continue
		}

		app := stuck[instance]

		for _, item := range queue.Records {
			if s := strings.ToLower(item.Status); s != completed && s != warning &&
				s != failed && s != errorstr && item.ErrorMessage == "" && len(item.StatusMessages) == 0 {
				continue
			}

			item.Quality = nil
			app.Queue = append(app.Queue, item)
		}

		app.Name = l.Name
		app.Elapsed = time.Since(start)
		stuck[instance] = app

		c.Debugf("Checking Lidarr (%d) Queue for Stuck Items, queue size: %d, stuck: %d",
			instance, len(queue.Records), len(stuck[instance].Queue))
	}

	return stuck
}

func (c *Config) getFinishedItemsRadarr() ItemList {
	stuck := make(ItemList)

	for i, l := range c.Apps.Radarr {
		instance := i + 1

		if !l.StuckItem {
			continue
		}

		start := time.Now()

		queue, err := l.GetQueue(getItemsMax, 1)
		if err != nil {
			c.Errorf("Getting Radarr Queue (%d): %v", instance, err)
			continue
		}

		app := stuck[instance]

		for _, item := range queue.Records {
			if s := strings.ToLower(item.Status); s != completed && s != warning &&
				s != failed && s != errorstr && item.ErrorMessage == "" && len(item.StatusMessages) == 0 {
				continue
			}

			item.Quality = nil
			item.CustomFormats = nil
			item.Languages = nil
			app.Queue = append(app.Queue, item)
		}

		app.Name = l.Name
		app.Elapsed = time.Since(start)
		stuck[instance] = app

		c.Debugf("Checking Radarr (%d) Queue for Stuck Items, queue size: %d, stuck: %d",
			instance, len(queue.Records), len(stuck[instance].Queue))
	}

	return stuck
}

func (c *Config) getFinishedItemsReadarr() ItemList {
	stuck := make(ItemList)

	for i, l := range c.Apps.Readarr {
		instance := i + 1

		if !l.StuckItem {
			continue
		}

		start := time.Now()

		queue, err := l.GetQueue(getItemsMax)
		if err != nil {
			c.Errorf("Getting Readarr Queue (%d): %v", instance, err)
			continue
		}

		app := stuck[instance]

		for _, item := range queue.Records {
			if s := strings.ToLower(item.Status); s != completed && s != warning &&
				s != failed && s != errorstr && item.ErrorMessage == "" && len(item.StatusMessages) == 0 {
				continue
			}

			item.Quality = nil
			app.Queue = append(app.Queue, item)
		}

		app.Name = l.Name
		app.Elapsed = time.Since(start)
		stuck[instance] = app

		c.Debugf("Checking Readarr (%d) Queue for Stuck Items, queue size: %d, stuck: %d",
			instance, len(queue.Records), len(stuck[instance].Queue))
	}

	return stuck
}

func (c *Config) getFinishedItemsSonarr() ItemList {
	stuck := make(ItemList)

	for i, l := range c.Apps.Sonarr {
		instance := i + 1

		if !l.StuckItem {
			continue
		}

		start := time.Now()

		queue, err := l.GetQueue(getItemsMax, 1)
		if err != nil {
			c.Errorf("Getting Sonarr Queue (%d): %v", instance, err)
			continue
		}

		// repeatStomper is used to collapse duplicate download IDs.
		repeatStomper := make(map[string]*sonarr.QueueRecord)
		app := stuck[instance]

		for _, item := range queue.Records {
			if s := strings.ToLower(item.Status); s != completed && s != warning &&
				s != failed && s != errorstr && item.ErrorMessage == "" && len(item.StatusMessages) == 0 {
				continue
			} else if repeatStomper[item.DownloadID] != nil {
				continue
			}

			item.Quality = nil
			item.Language = nil
			repeatStomper[item.DownloadID] = item
			app.Queue = append(app.Queue, item)
		}

		app.Name = l.Name
		app.Elapsed = time.Since(start)
		stuck[instance] = app

		c.Debugf("Checking Sonarr (%d) Queue for Stuck Items, queue size: %d, stuck: %d",
			instance, len(queue.Records), len(stuck[instance].Queue))
	}

	return stuck
}
