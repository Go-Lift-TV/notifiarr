// Package services provides service-checks to the notifiarr client application.
// This package spins up go routines to check http endpoints, running processes,
// tcp ports, etc. The configuration comes directly from the config file.
package services

import (
	"encoding/json"
	"time"

	"github.com/Notifiarr/notifiarr/pkg/logs"
	"github.com/Notifiarr/notifiarr/pkg/notifiarr"
)

func (c *Config) Setup(services []*Service) (*notifiarr.ServiceConfig, error) {
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
	if len(services) == 0 {
		c.Disabled = true
	}

	return c.setup(services)
}

func (c *Config) setup(services []*Service) (*notifiarr.ServiceConfig, error) {
	c.services = make(map[string]*Service)
	scnfg := &notifiarr.ServiceConfig{
		Interval: c.Interval,
		Parallel: c.Parallel,
		Disabled: c.Disabled,
		Checks:   make([]*notifiarr.ServiceCheck, len(services)),
	}

	for i, check := range services {
		if err := services[i].validate(); err != nil {
			return nil, err
		}

		// Add this validated service to our service map.
		c.services[services[i].Name] = services[i]
		scnfg.Checks[i] = &notifiarr.ServiceCheck{
			Name:     check.Name,
			Type:     string(check.Type),
			Expect:   check.Expect,
			Timeout:  check.Timeout,
			Interval: check.Interval,
		}
	}

	return scnfg, nil
}

// Start begins the service check routines.
// Runs Parallel checkers and the check reporter.
func (c *Config) Start() {
	if c.LogFile != "" {
		c.Logger = logs.CustomLog(c.LogFile, "Services")
	}

	for i := range c.services {
		c.services[i].log = c.Logger
	}

	c.checks = make(chan *Service, DefaultBuffer)
	c.done = make(chan bool)
	c.stopChan = make(chan struct{})
	c.triggerChan = make(chan notifiarr.EventType)

	for i := uint(0); i < c.Parallel; i++ {
		go func() {
			defer c.CapturePanic()

			for check := range c.checks {
				if c.done == nil {
					return
				} else if check == nil {
					c.done <- false
					return
				}

				c.done <- check.check()
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

func (c *Config) runServiceChecker() {
	defer c.CapturePanic()

	ticker := &time.Ticker{C: make(<-chan time.Time)}
	second := &time.Ticker{C: make(<-chan time.Time)}

	if !c.Disabled {
		ticker = time.NewTicker(c.Interval.Duration)
		defer ticker.Stop()

		second = time.NewTicker(10 * time.Second) //nolint:gomnd
		defer second.Stop()

		c.runChecks(true)
		c.SendResults(&Results{What: notifiarr.EventStart, Svcs: c.getResults()})
	}

	for {
		select {
		case <-c.stopChan:
			for i := uint(0); i < c.Parallel; i++ {
				c.checks <- nil
				<-c.done
			}

			c.stopChan <- struct{}{}
			c.Printf("==> Service Checker Stopped!")

			return
		case <-ticker.C:
			c.SendResults(&Results{What: notifiarr.EventCron, Svcs: c.getResults()})
		case event := <-c.triggerChan:
			c.Debugf("Running all service checks via event: %s, buffer: %d/%d", event, len(c.checks), cap(c.checks))
			c.runChecks(true)

			if event != "log" {
				c.SendResults(&Results{What: event, Svcs: c.getResults()})
			} else {
				data, _ := json.MarshalIndent(&Results{Svcs: c.getResults(), Interval: c.Interval.Seconds()}, "", " ")
				c.Debug("Service Checks Payload (log only):", string(data))
			}
		case <-second.C:
			c.runChecks(false)
		}
	}
}

// Stop ends all service checker routines.
func (c *Config) Stop() {
	if c.stopChan == nil {
		return
	}

	defer close(c.triggerChan)
	defer close(c.stopChan)
	defer close(c.checks)
	defer close(c.done)

	c.triggerChan = nil
	c.stopChan <- struct{}{}
	<-c.stopChan
	c.checks = nil
	c.done = nil
	c.stopChan = nil
}
