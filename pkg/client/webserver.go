package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	apachelog "github.com/lestrrat-go/apache-logformat"
)

// ErrNoServer returns when the server is already stopped and a stop req occurs.
var ErrNoServer = fmt.Errorf("the web server is not running, cannot stop it")

// StartWebServer starts the web server.
func (c *Client) StartWebServer() {
	// Create an apache-style logger.
	l, _ := apachelog.New(`%{X-Forwarded-For}i %l %u %t "%m %{X-Redacted-URI}i %H" %>s %b "%{Referer}i" ` +
		`"%{User-agent}i" %{X-Request-Time}i %{ms}Tms`)
	// Create a request router.
	c.Config.Apps.Router = mux.NewRouter()
	// Create a server.
	c.server = &http.Server{ // nolint: exhaustivestruct
		Handler:           c.stripSecrets(l.Wrap(c.fixForwardedFor(c.Config.Apps.Router), c.Logger.HTTPLog.Writer())),
		Addr:              c.Config.BindAddr,
		IdleTimeout:       time.Minute,
		WriteTimeout:      c.Config.Timeout.Duration,
		ReadTimeout:       c.Config.Timeout.Duration,
		ReadHeaderTimeout: c.Config.Timeout.Duration,
		ErrorLog:          c.Logger.ErrorLog,
	}

	// Initialize all the application API paths.
	c.Config.Apps.InitHandlers()
	c.internalHandlers()
	// Run the server.
	go c.runWebServer()
}

// runWebServer starts the http or https listener.
func (c *Client) runWebServer() {
	defer c.CapturePanic()

	var err error

	if c.menu["stat"] != nil {
		c.menu["stat"].Check()
		c.menu["stat"].SetTooltip("web server running, uncheck to pause")
	}

	if c.Config.SSLCrtFile != "" && c.Config.SSLKeyFile != "" {
		err = c.server.ListenAndServeTLS(c.Config.SSLCrtFile, c.Config.SSLKeyFile)
	} else {
		err = c.server.ListenAndServe()
	}

	c.server = nil

	if err != nil && !errors.Is(http.ErrServerClosed, err) {
		c.Errorf("Web Server Failed: %v (shutting down)", err)
		c.sigkil <- os.Kill // stop the app.
	}
}

// StopWebServer stops the web servers. Panics if that causes an error or timeout.
func (c *Client) StopWebServer() error {
	ctx, cancel := context.WithTimeout(context.Background(), c.Config.Timeout.Duration)
	defer cancel()

	if c.server == nil {
		return ErrNoServer
	}

	if c.menu["stat"] != nil {
		c.menu["stat"].Uncheck()
		c.menu["stat"].SetTooltip("web server paused, click to start")
	}

	if err := c.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutting down web server: %w", err)
	}

	return nil
}
