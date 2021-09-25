package client

import (
	"bytes"
	"fmt"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/Notifiarr/notifiarr/pkg/bindata"
	"github.com/Notifiarr/notifiarr/pkg/notifiarr"
	"github.com/Notifiarr/notifiarr/pkg/ui"
	"github.com/Notifiarr/notifiarr/pkg/update"
	"github.com/gorilla/mux"
	"golift.io/starr"
)

// internalHandlers initializes "special" internal API paths.
func (c *Client) internalHandlers() {
	c.Config.HandleAPIpath("", "version", c.website.VersionHandler, "GET", "HEAD")
	c.Config.HandleAPIpath("", "trigger/{trigger:[0-9a-z-]+}", c.handleTrigger, "GET")
	c.Config.HandleAPIpath("", "trigger/{trigger:[0-9a-z-]+}/{content}", c.handleTrigger, "GET")

	if c.Config.Plex.Configured() {
		c.Config.HandleAPIpath(starr.Plex, "sessions", c.Config.Plex.HandleSessions, "GET")
		c.Config.HandleAPIpath(starr.Plex, "kill", c.Config.Plex.HandleKillSession, "GET").
			Queries("reason", "{reason:.*}", "sessionId", "{sessionId:[0-9a-z-]+}")

		tokens := fmt.Sprintf("{token:%s|%s}", c.Config.Plex.Token, c.Config.Apps.APIKey)
		c.Config.Router.Handle("/plex",
			http.HandlerFunc(c.website.PlexHandler)).Methods("POST").Queries("token", tokens)

		if c.Config.URLBase != "/" {
			// Allow plex to use the base url too.
			c.Config.Router.Handle(path.Join(c.Config.URLBase, "plex"),
				http.HandlerFunc(c.website.PlexHandler)).Methods("POST").Queries("token", tokens)
		}
	}

	// Initialize internal-only paths.
	c.Config.Router.Handle("/favicon.ico", http.HandlerFunc(c.favIcon)) // built-in icon.
	c.Config.Router.Handle("/", http.HandlerFunc(c.slash))              // "hi" page on /

	if base := c.Config.URLBase; !strings.EqualFold(base, "/") {
		// Handle the same URLs on the different base URL too.
		c.Config.Router.Handle(path.Join(base, "favicon.ico"), http.HandlerFunc(c.favIcon))
		c.Config.Router.Handle(base, http.HandlerFunc(c.slash))     // "hi" page on /urlbase
		c.Config.Router.Handle(base+"/", http.HandlerFunc(c.slash)) // "hi" page on /urlbase/
	}

	c.Config.Router.PathPrefix("/").Handler(http.HandlerFunc(c.notFound)) // 404 everything
}

// notFound is the handler for paths that are not found: 404s.
func (c *Client) notFound(w http.ResponseWriter, r *http.Request) {
	c.Config.Respond(w, http.StatusNotFound, "Check your request parameters and try again.")
}

// slash is the handler for /.
func (c *Client) slash(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte("<p>" + c.Flags.Name() + ": <strong>working</strong></p>\n"))
}

func (c *Client) favIcon(w http.ResponseWriter, r *http.Request) {
	b, err := bindata.Asset("files/favicon.ico")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	http.ServeContent(w, r, r.URL.Path, time.Now(), bytes.NewReader(b))
}

// stripSecrets runs first to save a redacted URI in a special request header.
// The logger uses this special value to save a redacted URI in the log file.
func (c *Client) stripSecrets(next http.Handler) http.Handler {
	s := []string{c.Config.Apps.APIKey}
	// gather configured/known secrets.
	if c.Config.Plex != nil {
		s = append(s, c.Config.Plex.Token)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uri := r.RequestURI
		// then redact secrets from request.
		for _, s := range s {
			if s != "" {
				uri = strings.ReplaceAll(uri, s, "<redacted>")
			}
		}

		// save into a request header for the logger.
		r.Header.Set("X-Redacted-URI", uri)
		next.ServeHTTP(w, r)
	})
}

// fixForwardedFor sets the X-Forwarded-For header to the client IP
// under specific circumstances.
func (c *Client) fixForwardedFor(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := strings.Trim(r.RemoteAddr[:strings.LastIndex(r.RemoteAddr, ":")], "[]")
		if x := r.Header.Get("X-Forwarded-For"); x == "" || !c.Config.Allow.Contains(ip) {
			r.Header.Set("X-Forwarded-For", ip)
		} else if l := strings.LastIndexAny(x, ", "); l != -1 {
			r.Header.Set("X-Forwarded-For", strings.Trim(x[l:len(x)-1], ", "))
		}

		next.ServeHTTP(w, r)
	})
}

func (c *Client) handleTrigger(r *http.Request) (int, interface{}) { //nolint:cyclop
	trigger := mux.Vars(r)["trigger"]
	c.Debugf("Incoming API Trigger: %s", trigger)

	switch trigger {
	case "cfsync":
		c.website.Trigger.SyncCF(notifiarr.EventAPI)
	case "services":
		c.Config.Services.RunChecks(notifiarr.EventAPI)
	case "sessions":
		if !c.Config.Plex.Configured() {
			return http.StatusNotImplemented, "sessions not enabled"
		}

		c.website.Trigger.SendPlexSessions(notifiarr.EventAPI)
	case "stuckitems":
		c.website.Trigger.SendStuckQueueItems(notifiarr.EventAPI)
	case "dashboard":
		c.website.Trigger.SendDashboardState(notifiarr.EventAPI)
	case "snapshot":
		c.website.Trigger.SendSnapshot(notifiarr.EventAPI)
	case "gaps":
		c.website.Trigger.SendGaps(notifiarr.EventAPI)
	case "reload":
		c.sighup <- &update.Signal{Text: "reload http triggered"}
	case "notification":
		if content := mux.Vars(r)["content"]; content != "" {
			ui.Notify("Notification: %s", content) //nolint:errcheck
			c.Printf("NOTIFICATION: %s", content)
		} else {
			return http.StatusBadRequest, "missing notification content"
		}
	default:
		return http.StatusBadRequest, "unknown trigger '" + trigger + "'"
	}

	return http.StatusOK, trigger + " initiated"
}
