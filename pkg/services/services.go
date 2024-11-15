// Package services provides service-checks to the notifiarr client application.
// This package spins up go routines to check http endpoints, running processes,
// tcp ports, etc. The configuration comes directly from the config file.
package services

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/Notifiarr/notifiarr/pkg/logs"
	"github.com/Notifiarr/notifiarr/pkg/mnd"
	"github.com/Notifiarr/notifiarr/pkg/website"
	"github.com/gorilla/mux"
	"golift.io/version"
)

func (c *Config) Setup(services []*Service) error {
	if c.Parallel > MaximumParallel {
		c.Parallel = MaximumParallel
	} else if c.Parallel == 0 {
		c.Parallel = 1
	}

	if c.Interval.Duration == 0 {
		c.Interval.Duration = DefaultSendInterval
	} else if c.Interval.Duration < MinimumSendInterval {
		c.Interval.Duration = MinimumSendInterval
	}

	services = append(services, c.collectApps()...)

	return c.setup(services)
}

func (c *Config) setup(services []*Service) error {
	c.services = make(map[string]*Service)

	for idx, check := range services {
		if err := services[idx].Validate(); err != nil {
			return err
		}

		mnd.ServiceChecks.Add(check.Name+"&&Total", 0)
		mnd.ServiceChecks.Add(check.Name+"&&"+StateUnknown.String(), 0)
		mnd.ServiceChecks.Add(check.Name+"&&"+StateOK.String(), 0)
		mnd.ServiceChecks.Add(check.Name+"&&"+StateWarning.String(), 0)
		mnd.ServiceChecks.Add(check.Name+"&&"+StateCritical.String(), 0)

		// Add this validated service to our service map.
		c.services[services[idx].Name] = services[idx]
	}

	return nil
}

func (c *Config) SetWebsite(website *website.Server) {
	c.website = website
}

// Start begins the service check routines.
// Runs Parallel checkers and the check reporter.
func (c *Config) Start(ctx context.Context) {
	if len(c.services) == 0 {
		c.Printf("==> Service Checker Disabled! No services to check.")
		return
	}

	c.stopLock.Lock()
	defer c.stopLock.Unlock()

	if c.LogFile != "" {
		c.Logger = logs.CustomLog(c.LogFile, "Services")
	}

	for name := range c.services {
		c.services[name].svc.log = c.Logger
	}

	c.applyLocalOverrides()
	c.loadServiceStates(ctx)
	c.checks = make(chan *Service, DefaultBuffer)
	c.done = make(chan bool)
	c.stopChan = make(chan struct{})
	c.triggerChan = make(chan website.EventType)
	c.checkChan = make(chan triggerCheck)

	for range c.Parallel {
		go func() {
			defer c.CapturePanic()

			for check := range c.checks {
				if c.done == nil {
					return
				} else if check == nil {
					c.done <- false
					return
				}

				c.done <- check.check(ctx)
			}
		}()
	}

	go c.runServiceChecker()

	word := "Started"
	if c.Disabled {
		word = "Disabled"
	}

	c.Printf("==> Service Checker %s! %d services, interval: %s, parallel: %d",
		word, len(c.services), c.Interval, c.Parallel)
}

func (c *Config) applyLocalOverrides() {
	if !c.Apps.Plex.Enabled() {
		return
	}

	name := c.Apps.Plex.Server.Name()
	if name == "" {
		return
	}

	// This is how we shoehorn the plex servr name into the service check.
	// We do this because we don't have the name when the config file is parsed.
	for _, svc := range c.services {
		if svc.Name == PlexServerName {
			svc.Tags = map[string]any{"name": name}
			return
		}
	}
}

// loadServiceStates brings service states from the website into the fold.
// In other words, states are stored in the website's database.
func (c *Config) loadServiceStates(ctx context.Context) {
	names := []string{}
	for name := range c.services {
		names = append(names, valuePrefix+name)
	}

	if len(names) == 0 {
		return
	}

	values, err := c.website.GetState(ctx, names...)
	if err != nil {
		c.ErrorfNoShare("Getting initial service states from website: %v", err)
		return
	}

	for name := range c.services {
		for siteDataName := range values {
			if name == strings.TrimPrefix(siteDataName, valuePrefix) {
				var svc service
				if err := json.Unmarshal(values[siteDataName], &svc); err != nil {
					c.ErrorfNoShare("Service check data for '%s' returned from site is invalid: %v", name, err)
					break
				}

				if time.Since(svc.LastCheck) < 2*time.Hour {
					c.Printf("==> Set service state with website-saved data: %s, %s for %s",
						name, svc.State, time.Since(svc.Since).Round(time.Second))

					c.services[name].svc.Output = svc.Output
					c.services[name].svc.State = svc.State
					c.services[name].svc.Since = svc.Since
					c.services[name].svc.LastCheck = svc.LastCheck
				}

				break
			}
		}
	}
}

func (c *Config) runServiceChecker() { //nolint:cyclop
	defer func() {
		defer c.CapturePanic()
		c.Printf("==> Service Checker Stopped!")
		c.stopChan <- struct{}{} // signal we're finished.
	}()

	checker := &time.Ticker{C: make(<-chan time.Time)}

	if !c.Disabled {
		checker = time.NewTicker(time.Second)
		defer checker.Stop()

		c.runChecks(true, version.Started)
		c.SendResults(&Results{What: website.EventStart, Svcs: c.GetResults()})
	}

	for {
		select {
		case <-c.stopChan:
			for range c.Parallel {
				c.checks <- nil
				<-c.done
			}

			return
		case event := <-c.checkChan:
			c.Printf("Running service check '%s' via event: %s, buffer: %d/%d",
				event.Service.Name, event.Source, len(c.checks), cap(c.checks))

			if c.runCheck(event.Service, true, time.Now()) {
				c.SendResults(&Results{What: event.Source, Svcs: c.GetResults()})
			}
		case event := <-c.triggerChan:
			c.Debugf("Running all service checks via event: %s, buffer: %d/%d", event, len(c.checks), cap(c.checks))
			c.runChecks(true, time.Now())

			if event != "log" {
				c.SendResults(&Results{What: event, Svcs: c.GetResults()})
				continue
			}

			data, err := json.MarshalIndent(&Results{Svcs: c.GetResults(), Interval: c.Interval.Seconds()}, "", " ")
			if err != nil {
				c.Errorf("Marshalling Service Checks: %v; payload: %s", err, string(data))
				continue
			}

			c.Debug("Service Checks Payload (log only):", string(data))
		case now := <-checker.C:
			if c.runChecks(false, now) {
				c.SendResults(&Results{What: website.EventCron, Svcs: c.GetResults()})
			}
		}
	}
}

func (c *Config) Running() bool {
	c.stopLock.Lock()
	defer c.stopLock.Unlock()

	return c.stopChan != nil
}

// Stop sends current states to the website and ends all service checker routines.
func (c *Config) Stop() {
	c.stopLock.Lock()
	defer c.stopLock.Unlock()

	if c.stopChan == nil {
		return
	}

	defer close(c.stopChan)
	c.stopChan <- struct{}{}
	<-c.stopChan // wait for all go routines to die off.

	close(c.triggerChan)
	close(c.checkChan)
	close(c.checks)
	close(c.done)

	c.triggerChan = nil
	c.checkChan = nil
	c.checks = nil
	c.done = nil
	c.stopChan = nil
}

// SvcCount returns the count of services being monitored.
func (c *Config) SvcCount() int {
	return len(c.services)
}

// APIHandler is passed into the webserver so services can be accessed by the API.
func (c *Config) APIHandler(req *http.Request) (int, any) {
	return c.handleTrigger(req, website.EventAPI)
}

func (c *Config) handleTrigger(req *http.Request, event website.EventType) (int, any) {
	action := mux.Vars(req)["action"]
	c.Debugf("[%s requested] Incoming Service Action: %s (%s)", event, action)

	switch action {
	case "list":
		return c.returnServiceList()
	default:
		return http.StatusBadRequest, "unknown service action: " + action
	}
}

// @Description  Returns a list of service check results.
// @Summary      Get service check results
// @Tags         Triggers
// @Produce      json
// @Success      200  {object} apps.Respond.apiResponse{message=[]CheckResult} "list check results"
// @Failure      404  {object} string "bad token or api key"
// @Router       /api/services/list [get]
// @Security     ApiKeyAuth
func (c *Config) returnServiceList() (int, any) {
	return http.StatusOK, c.GetResults()
}
