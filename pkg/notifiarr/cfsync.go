package notifiarr

import (
	"encoding/json"
	"fmt"

	"github.com/Notifiarr/notifiarr/pkg/apps"
	"golift.io/cnfg"
	"golift.io/starr/radarr"
	"golift.io/starr/sonarr"
)

// syncConfig is the configuration returned from the notifiarr website.
type syncConfig struct {
	Interval        cnfg.Duration `json:"interval"`        // how often to fire in minutes.
	Radarr          int64         `json:"radarr"`          // items in sync
	RadarrInstances intList       `json:"radarrInstances"` // which instance IDs we sync
	Sonarr          int64         `json:"sonarr"`          // items in sync
	SonarrInstances intList       `json:"sonarrInstances"` // which instance IDs we sync
}

// cfMapIDpayload is used to post-back ID changes for profiles and formats.
type cfMapIDpayload struct {
	Instance int     `json:"instance"`
	RP       []idMap `json:"releaseProfiles,omitempty"`
	QP       []idMap `json:"qualityProfiles,omitempty"`
	CF       []idMap `json:"customFormats,omitempty"`
}

// idMap is used a mapping list from old ID to new ID. Part of cfMapIDpayload.
type idMap struct {
	OldID int64 `json:"oldId"`
	NewID int64 `json:"newId"`
}

/*//*****/ // Radarr /*//*****///

// RadarrCustomFormatPayload is the payload sent and received
// to/from notifarr.com when updating custom formats for Radarr.
type RadarrCustomFormatPayload struct {
	Instance        int                      `json:"instance"`
	Name            string                   `json:"name"`
	CustomFormats   []*radarr.CustomFormat   `json:"customFormats,omitempty"`
	QualityProfiles []*radarr.QualityProfile `json:"qualityProfiles,omitempty"`
	NewMaps         *cfMapIDpayload          `json:"newMaps,omitempty"`
}

func (t *Triggers) SyncCF(event EventType) {
	if t.stop == nil {
		return
	}

	t.sync.C <- event
}

func (c *Config) syncCF(event EventType) {
	c.Debugf("Running CF Sync via event: %s", event)
	c.syncRadarr(event)
	c.syncSonarr(event)
}

// syncRadarr triggers a custom format sync for Radarr.
func (c *Config) syncRadarr(event EventType) {
	if c.ClientInfo == nil || len(c.Actions.Sync.RadarrInstances) < 1 {
		c.Debugf("Cannot sync Radarr Custom Formats. Website provided 0 instances.")
		return
	} else if len(c.Apps.Radarr) < 1 {
		c.Debugf("Cannot sync Radarr Custom Formats. No Radarr instances configured.")
		return
	}

	for i, app := range c.Apps.Radarr {
		instance := i + 1
		if app.URL == "" || app.APIKey == "" || !c.Actions.Sync.RadarrInstances.Has(instance) {
			c.Debugf("CF Sync Skipping Radarr instance %d. Not in sync list: %v", instance, c.Actions.Sync.RadarrInstances)
			continue
		}

		if err := c.syncRadarrCF(instance, app); err != nil {
			c.Errorf("[%s requested] Radarr Custom Formats sync request for '%d:%s' failed: %v", event, instance, app.URL, err)
			return
		}

		c.Printf("[%s requested] Synced Custom Formats from Notifiarr for Radarr: %d:%s", event, instance, app.URL)
	}
}

func (c *Config) syncRadarrCF(instance int, app *apps.RadarrConfig) error { //nolint:dupl
	var (
		err     error
		payload = RadarrCustomFormatPayload{Instance: instance, Name: app.Name, NewMaps: c.radarrCF[instance]}
	)

	payload.QualityProfiles, err = app.GetQualityProfiles()
	if err != nil {
		return fmt.Errorf("getting quality profiles: %w", err)
	}

	payload.CustomFormats, err = app.GetCustomFormats()
	if err != nil {
		return fmt.Errorf("getting custom formats: %w", err)
	}

	body, err := c.SendData(CFSyncRoute.Path("", "app=radarr"), payload, false)
	if err != nil {
		return fmt.Errorf("sending current formats: %w", err)
	}

	delete(c.radarrCF, instance)

	if body.Status != success {
		return fmt.Errorf("%w: %s", ErrInvalidResponse, body.Status)
	}

	if err := c.updateRadarrCF(instance, app, body.Message.Response); err != nil {
		return fmt.Errorf("updating application: %w", err)
	}

	return nil
}

func (c *Config) updateRadarrCF(instance int, app *apps.RadarrConfig, data []byte) error {
	reply := &RadarrCustomFormatPayload{}
	if err := json.Unmarshal(data, &reply); err != nil {
		return fmt.Errorf("bad json response: %w", err)
	}

	c.Debugf("Received %d quality profiles and %d custom formats for Radarr: %d:%s",
		len(reply.QualityProfiles), len(reply.CustomFormats), instance, app.URL)

	maps := &cfMapIDpayload{QP: []idMap{}, CF: []idMap{}, Instance: instance}

	for i, profile := range reply.CustomFormats {
		id := profile.ID
		if _, err := app.UpdateCustomFormat(profile, id); err != nil {
			profile.ID = 0

			c.Debugf("Error Updating custom format [%d/%d] (attempting to ADD %d): %v",
				i, len(reply.CustomFormats), id, err)

			newID, err2 := app.AddCustomFormat(profile)
			if err2 != nil {
				return fmt.Errorf("[%d/%d] updating custom format: %d: (update) %v, (add) %w",
					i, len(reply.CustomFormats), id, err, err2)
			}

			maps.CF = append(maps.CF, idMap{int64(id), int64(newID.ID)})
		}
	}

	for i, profile := range reply.QualityProfiles {
		if err := app.UpdateQualityProfile(profile); err != nil {
			id := profile.ID
			profile.ID = 0

			c.Debugf("Error Updating quality profile [%d/%d] (attempting to ADD %d): %v",
				i, len(reply.QualityProfiles), id, err)

			newID, err2 := app.AddQualityProfile(profile)
			if err2 != nil {
				return fmt.Errorf("[%d/%d] updating quality profile: %d: (update) %v, (add) %w",
					i, len(reply.QualityProfiles), id, err, err2)
			}

			maps.QP = append(maps.QP, idMap{id, newID})
		}
	}

	return c.postbackRadarrCF(instance, maps)
}

// postbackRadarrCF sends the changes back to notifiarr.com.
func (c *Config) postbackRadarrCF(instance int, maps *cfMapIDpayload) error {
	if len(maps.CF) < 1 && len(maps.QP) < 1 {
		return nil
	}

	_, err := c.SendData(CFSyncRoute.Path("", "app=radarr", "updateIDs=true"), &RadarrCustomFormatPayload{
		Instance: instance,
		NewMaps:  maps,
	}, false)
	if err != nil {
		c.radarrCF[instance] = maps
		return fmt.Errorf("updating custom format ID map: %w", err)
	}

	delete(c.radarrCF, instance)

	return nil
}

/*//*****/ // Sonarr /*//*****///

// SonarrCustomFormatPayload is the payload sent and received
// to/from notifarr.com when updating custom formats for Sonarr.
type SonarrCustomFormatPayload struct {
	Instance        int                      `json:"instance"`
	Name            string                   `json:"name"`
	ReleaseProfiles []*sonarr.ReleaseProfile `json:"releaseProfiles,omitempty"`
	QualityProfiles []*sonarr.QualityProfile `json:"qualityProfiles,omitempty"`
	NewMaps         *cfMapIDpayload          `json:"newMaps,omitempty"`
}

// syncSonarr triggers a custom format sync for Sonarr.
func (c *Config) syncSonarr(event EventType) {
	if c.ClientInfo == nil || len(c.Actions.Sync.SonarrInstances) < 1 {
		c.Debugf("Cannot sync Sonarr Release Profiles. Website provided 0 instances.")
		return
	} else if len(c.Apps.Sonarr) < 1 {
		c.Debugf("Cannot sync Sonarr Release Profiles. No Sonarr instances configured.")
		return
	}

	for i, app := range c.Apps.Sonarr {
		instance := i + 1
		if app.URL == "" || app.APIKey == "" || !c.Actions.Sync.SonarrInstances.Has(instance) {
			c.Debugf("CF Sync Skipping Sonarr instance %d. Not in sync list: %v", instance, c.Actions.Sync.SonarrInstances)
			continue
		}

		if err := c.syncSonarrRP(instance, app); err != nil {
			c.Errorf("[%s requested] Sonarr Release Profiles sync for '%d:%s' failed: %v", event, instance, app.URL, err)
			return
		}

		c.Printf("[%s requested] Synced Sonarr Release Profiles from Notifiarr: %d:%s", event, instance, app.URL)
	}
}

func (c *Config) syncSonarrRP(instance int, app *apps.SonarrConfig) error { //nolint:dupl
	var (
		err     error
		payload = SonarrCustomFormatPayload{Instance: instance, Name: app.Name, NewMaps: c.sonarrRP[instance]}
	)

	payload.QualityProfiles, err = app.GetQualityProfiles()
	if err != nil {
		return fmt.Errorf("getting quality profiles: %w", err)
	}

	payload.ReleaseProfiles, err = app.GetReleaseProfiles()
	if err != nil {
		return fmt.Errorf("getting release profiles: %w", err)
	}

	body, err := c.SendData(CFSyncRoute.Path("", "app=sonarr"), payload, false)
	if err != nil {
		return fmt.Errorf("sending current profiles: %w", err)
	}

	delete(c.sonarrRP, instance)

	if body.Status != success {
		return fmt.Errorf("%w: %s", ErrInvalidResponse, body.Status)
	}

	if err := c.updateSonarrRP(instance, app, body.Message.Response); err != nil {
		return fmt.Errorf("updating application: %w", err)
	}

	return nil
}

func (c *Config) updateSonarrRP(instance int, app *apps.SonarrConfig, data []byte) error {
	reply := &SonarrCustomFormatPayload{}
	if err := json.Unmarshal(data, &reply); err != nil {
		return fmt.Errorf("bad json response: %w", err)
	}

	c.Debugf("Received %d quality profiles and %d release profiles for Sonarr: %d:%s",
		len(reply.QualityProfiles), len(reply.ReleaseProfiles), instance, app.URL)

	maps := &cfMapIDpayload{RP: []idMap{}, QP: []idMap{}, Instance: instance}

	for i, profile := range reply.ReleaseProfiles {
		if err := app.UpdateReleaseProfile(profile); err != nil {
			id := profile.ID
			profile.ID = 0

			c.Debugf("Error Updating release profile [%d/%d] (attempting to ADD %d): %v",
				i, len(reply.ReleaseProfiles), id, err)

			newID, err2 := app.AddReleaseProfile(profile)
			if err2 != nil {
				return fmt.Errorf("[%d/%d] updating release profiles: %d: (update) %v, (add) %w",
					i, len(reply.ReleaseProfiles), id, err, err2)
			}

			maps.RP = append(maps.RP, idMap{id, newID})
		}
	}

	for i, profile := range reply.QualityProfiles {
		if err := app.UpdateQualityProfile(profile); err != nil {
			id := profile.ID
			profile.ID = 0

			c.Debugf("Error Updating quality format [%d/%d] (attempting to ADD %d): %v",
				i, len(reply.QualityProfiles), id, err)

			newID, err2 := app.AddQualityProfile(profile)
			if err2 != nil {
				return fmt.Errorf("[%d/%d] updating quality profile: %d: (update) %v, (add) %w",
					i, len(reply.QualityProfiles), id, err, err2)
			}

			maps.QP = append(maps.QP, idMap{id, newID})
		}
	}

	return c.postbackSonarrRP(instance, maps)
}

// postbackSonarrRP sends the changes back to notifiarr.com.
func (c *Config) postbackSonarrRP(instance int, maps *cfMapIDpayload) error {
	if len(maps.QP) < 1 && len(maps.RP) < 1 {
		return nil
	}

	_, err := c.SendData(CFSyncRoute.Path("", "app=sonarr", "updateIDs=true"), &SonarrCustomFormatPayload{
		Instance: instance,
		NewMaps:  maps,
	}, false)
	if err != nil {
		c.sonarrRP[instance] = maps
		return fmt.Errorf("updating quality release ID map: %w", err)
	}

	delete(c.sonarrRP, instance)

	return nil
}
