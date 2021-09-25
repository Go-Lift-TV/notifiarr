package notifiarr

import (
	"fmt"

	"github.com/Notifiarr/notifiarr/pkg/apps"
	"golift.io/cnfg"
	"golift.io/starr/radarr"
)

/* Gaps allows filling gaps in Radarr collections. */

// gapsConfig is the configuration returned from the notifiarr website.
type gapsConfig struct {
	Instances intList       `json:"instances"`
	Interval  cnfg.Duration `json:"interval"`
}

func (t *Triggers) SendGaps(event EventType) {
	if t.stop == nil {
		return
	}

	t.gaps.C <- event
}

func (c *Config) sendGaps(event EventType) {
	if c.ClientInfo == nil || len(c.Actions.Gaps.Instances) == 0 || len(c.Apps.Radarr) == 0 {
		c.Errorf("[%s requested] Cannot send Radarr Collection Gaps: instances or configured Radarrs (%d) are zero.",
			event, len(c.Apps.Radarr))
		return
	}

	for i, r := range c.Apps.Radarr {
		instance := i + 1
		if r.URL == "" || r.APIKey == "" || !c.Actions.Gaps.Instances.Has(instance) {
			continue
		}

		if resp, err := c.sendInstanceGaps(event, instance, r); err != nil {
			c.Errorf("[%s requested] Radarr Collection Gaps request for '%d:%s' failed: %v", event, instance, r.URL, err)
		} else {
			c.Printf("[%s requested] Sent Collection Gaps to Notifiarr for Radarr: %d:%s. "+
				"Website took %s and replied with: %s, %s",
				event, instance, r.URL, resp.Message.Elapsed, resp.Status, resp.Message.Response)
		}
	}
}

func (c *Config) sendInstanceGaps(event EventType, instance int, app *apps.RadarrConfig) (*Response, error) {
	type radarrGapsPayload struct {
		Instance int             `json:"instance"`
		Name     string          `json:"name"`
		Movies   []*radarr.Movie `json:"movies"`
	}

	movies, err := app.GetMovie(0)
	if err != nil {
		return nil, fmt.Errorf("getting movies: %w", err)
	}

	resp, err := c.SendData(GapsRoute.Path(event, "app=radarr"), &radarrGapsPayload{
		Movies:   movies,
		Name:     app.Name,
		Instance: instance,
	}, false)
	if err != nil {
		return nil, fmt.Errorf("sending collection gaps: %w", err)
	}

	return resp, nil
}
