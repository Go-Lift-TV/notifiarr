package services

import (
	"errors"
	"fmt"
	"time"

	"github.com/Notifiarr/notifiarr/pkg/website"
)

const valuePrefix = "serviceCheck-"

var ErrSvcsStopped = errors.New("service check routine stopped")

// RunChecks runs checks from an external package.
func (c *Config) RunChecks(source website.EventType) {
	c.stopLock.Lock()
	defer c.stopLock.Unlock()

	if c.triggerChan == nil || c.stopChan == nil {
		c.Errorf("Cannot run service checks. Go routine is not running.")
		return
	}

	c.triggerChan <- source
}

// RunCheck runs a single check from an external package.
func (c *Config) RunCheck(source website.EventType, name string) error {
	c.stopLock.Lock()
	defer c.stopLock.Unlock()

	if c.triggerChan == nil || c.stopChan == nil {
		return fmt.Errorf("cannot check service, %w", ErrSvcsStopped)
	}

	svc, ok := c.services[name]
	if !ok {
		return fmt.Errorf("%w: service '%s' not found", ErrNoName, name)
	}

	c.checkChan <- triggerCheck{Source: source, Service: svc}

	return nil
}

// runCheck runs a service check if it is due. Passing force runs it regardless.
func (c *Config) runCheck(svc *Service, force bool, now time.Time) bool {
	if force || svc.Due(now) {
		c.checks <- svc
		return <-c.done
	}

	return false
}

// runChecks runs checks that are due. Passing true, runs them even if they're not due.
// Returns true if any service state changed.
func (c *Config) runChecks(forceAll bool, now time.Time) bool {
	if c.checks == nil || c.done == nil {
		return false
	}

	count := 0
	changes := false

	for s := range c.services {
		if forceAll || c.services[s].Due(now) {
			count++
			c.checks <- c.services[s]
		}
	}

	for ; count > 0; count-- {
		changes = <-c.done || changes
	}

	return changes
}

// GetResults creates a copy of all the results and returns them.
func (c *Config) GetResults() []*CheckResult {
	svcs := make([]*CheckResult, len(c.services))
	count := 0

	for _, svc := range c.services {
		svcs[count] = svc.copyResults()
		count++
	}

	return svcs
}

func (s *Service) copyResults() *CheckResult {
	s.svc.RLock()
	defer s.svc.RUnlock()

	return &CheckResult{
		Interval:    s.Interval.Duration.Seconds(),
		Name:        s.Name,
		State:       s.svc.State,
		Output:      s.svc.Output,
		Type:        s.Type,
		Time:        s.svc.LastCheck,
		Since:       s.svc.Since,
		Check:       s.Value,
		Expect:      s.Expect,
		IntervalDur: s.Interval.Duration,
		Metadata:    s.Tags,
	}
}

// SendResults sends a set of Results to Notifiarr.
func (c *Config) SendResults(results *Results) {
	results.Interval = c.Interval.Seconds()

	c.website.SendData(&website.Request{
		Route:      website.SvcRoute,
		Event:      results.What,
		LogPayload: true,
		LogMsg: fmt.Sprintf("%d service updates to Notifiarr, event: %s, buffer: %d/%d",
			len(results.Svcs), results.What, len(c.checks), cap(c.checks)),
		Payload: results,
	})
}

// String turns a check status into a human string.
func (c CheckState) String() string {
	switch c {
	default:
		fallthrough
	case StateUnknown:
		return "Unknown"
	case StateCritical:
		return "Critical"
	case StateWarning:
		return "Warning"
	case StateOK:
		return "OK"
	}
}

func (c CheckState) Value() uint {
	return uint(c)
}
