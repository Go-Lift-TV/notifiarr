package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Notifiarr/notifiarr/pkg/mnd"
	"github.com/Notifiarr/notifiarr/pkg/notifiarr"
	"github.com/Notifiarr/notifiarr/pkg/plex"
)

func (c *Client) plexIncoming(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	if err := r.ParseMultipartForm(mnd.KB100); err != nil {
		c.Errorf("Parsing Multipart Form (plex): %v", err)
		c.Config.Respond(w, http.StatusBadRequest, "form parse error")

		return
	}

	payload := r.Form.Get("payload")
	c.Debugf("Plex Webhook Payload: %s", payload)
	r.Header.Set("X-Request-Time", fmt.Sprintf("%dms", time.Since(start).Milliseconds()))

	var v plex.Webhook

	switch err := json.Unmarshal([]byte(payload), &v); {
	case err != nil:
		c.Config.Respond(w, http.StatusInternalServerError, "payload error")
		c.Errorf("Unmarshalling Plex payload: %v", err)
	case v.Event == "media.play":
		go c.collectSessions(&v, plex.WaitTime)
		c.Config.Respond(w, http.StatusAccepted, "processing")
	case (v.Event == "media.resume" || v.Event == "media.pause") && c.plex.Active(c.Config.Plex.Cooldown.Duration):
		c.Printf("Plex Incoming Webhook IGNORED (cooldown): %s, %s '%s' => %s",
			v.Server.Title, v.Account.Title, v.Event, v.Metadata.Title)
		c.Config.Respond(w, http.StatusAlreadyReported, "ignored, cooldown")
	case strings.HasPrefix(v.Event, "media"):
		c.collectSessions(&v, 0)
		r.Header.Set("X-Request-Time", fmt.Sprintf("%dms", time.Since(start).Milliseconds()))
		c.Config.Respond(w, http.StatusAccepted, "processed")
	default:
		c.Config.Respond(w, http.StatusAlreadyReported, "ignored, non-media")
		c.Printf("Plex Incoming Webhook IGNORED (non-media): %s, %s '%s' => %s",
			v.Server.Title, v.Account.Title, v.Event, v.Metadata.Title)
	}
}

func (c *Client) collectSessions(v *plex.Webhook, wait time.Duration) {
	c.Printf("Plex Incoming Webhook: %s, %s '%s' => %s (collecting sessions)",
		v.Server.Title, v.Account.Title, v.Event, v.Metadata.Title)

	reply, err := c.notify.SendMeta(notifiarr.PlexHook, c.notify.URL, v, wait)
	if err != nil {
		c.Errorf("Sending Plex Session to Notifiarr: %v", err)
		return
	}

	if fields := strings.Split(string(reply), `"`); len(fields) > 3 { // nolint:gomnd
		c.Printf("Plex => Notifiarr: %s '%s' => %s (%s)", v.Account.Title, v.Event, v.Metadata.Title, fields[3])
	} else {
		c.Printf("Plex => Notifiarr: %s '%s' => %s", v.Account.Title, v.Event, v.Metadata.Title)
	}
}

// logSnaps writes a full snapshot payload to the log file.
func (c *Client) logSnaps() {
	c.Printf("[user requested] Collecting Snapshot from Plex and the System (for log file).")

	snaps, errs, debug := c.Config.Snapshot.GetSnapshot()
	for _, err := range errs {
		if err != nil {
			c.Errorf("[user requested] %v", err)
		}
	}

	for _, err := range debug {
		if err != nil {
			c.Errorf("[user requested] %v", err)
		}
	}

	var (
		plex *plex.Sessions
		err  error
	)

	if c.Config.Plex.Configured() {
		if plex, err = c.Config.Plex.GetXMLSessions(); err != nil {
			c.Errorf("[user requested] %v", err)
		}
	}

	b, _ := json.MarshalIndent(&notifiarr.Payload{
		Type: notifiarr.LogLocal,
		Snap: snaps,
		Plex: plex,
	}, "", "  ")
	c.Printf("[user requested] Snapshot Data:\n%s", string(b))
}

// sendSystemSnapshot is triggered from a menu-bar item, and from --send cli arg.
func (c *Client) sendSystemSnapshot(url string) string {
	c.Printf("[user requested] Sending System Snapshot to %s", url)

	snaps, errs, debug := c.Config.Snapshot.GetSnapshot()
	for _, err := range errs {
		if err != nil {
			c.Errorf("[user requested] %v", err)
		}
	}

	for _, err := range debug {
		if err != nil {
			c.Errorf("[user requested] %v", err)
		}
	}

	b, _ := json.MarshalIndent(&notifiarr.Payload{Type: notifiarr.SnapCron, Snap: snaps}, "", "  ")
	if _, body, err := c.notify.SendJSON(url, b); err != nil { //nolint:bodyclose // body already closed
		c.Errorf("[user requested] Sending System Snapshot to %s: %v", url, err)
	} else if fields := strings.Split(string(body), `"`); len(fields) > 3 { //nolint:gomnd
		c.Printf("[user requested] Sent System Snapshot to %s, reply: %s", url, fields[3])
	} else {
		c.Printf("[user requested] Sent System Snapshot to %s", url)
	}

	return string(b)
}
