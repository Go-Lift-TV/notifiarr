package notifiarr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/Notifiarr/notifiarr/pkg/plex"
	"github.com/Notifiarr/notifiarr/pkg/snapshot"
)

// httpClient is our custom http client to wrap Do and provide retries.
type httpClient struct {
	Retries int
	*log.Logger
	*http.Client
}

// sendPlexMeta is kicked off by the webserver in go routine.
// It's also called by the plex cron (with webhook set to nil).
// This runs after Plex drops off a webhook telling us someone did something.
// This gathers cpu/ram, and waits 10 seconds, then grabs plex sessions.
// It's all POSTed to notifiarr. May be used with a nil Webhook.
func (c *Config) sendPlexMeta(event EventType, hook *plexIncomingWebhook, wait bool) (*Response, error) {
	extra := time.Second
	if wait {
		extra = c.Plex.Delay.Duration
	}

	ctx, cancel := context.WithTimeout(context.Background(), extra+c.Snap.Timeout.Duration)
	defer cancel()

	var (
		payload = &Payload{Load: hook, Plex: &plex.Sessions{Name: c.Plex.Name}}
		wg      sync.WaitGroup
	)

	rep := make(chan error)
	defer close(rep)

	go func() {
		for err := range rep {
			if err != nil {
				c.Errorf("Building Metadata: %v", err)
			}
		}
	}()

	wg.Add(1)

	go func() {
		payload.Snap = c.getMetaSnap(ctx)
		wg.Done() // nolint:wsl
	}()

	if !wait || !c.Plex.NoActivity {
		var err error
		if payload.Plex, err = c.GetSessions(wait); err != nil {
			rep <- fmt.Errorf("getting sessions: %w", err)
		}
	}

	wg.Wait()

	return c.SendData(PlexRoute.Path(event), payload, true)
}

// getMetaSnap grabs some basic system info: cpu, memory, username.
func (c *Config) getMetaSnap(ctx context.Context) *snapshot.Snapshot {
	var (
		snap = &snapshot.Snapshot{}
		wg   sync.WaitGroup
	)

	rep := make(chan error)
	defer close(rep)

	go func() {
		for err := range rep {
			if err != nil { // maybe move this out of this method?
				c.Errorf("Building Metadata: %v", err)
			}
		}
	}()

	wg.Add(3) //nolint: gomnd,wsl
	go func() {
		rep <- snap.GetCPUSample(ctx)
		wg.Done() //nolint:wsl
	}()
	go func() {
		rep <- snap.GetMemoryUsage(ctx)
		wg.Done() //nolint:wsl
	}()
	go func() {
		for _, err := range snap.GetLocalData(ctx) {
			rep <- err
		}
		wg.Done() //nolint:wsl
	}()

	wg.Wait()

	return snap
}

func (c *Config) GetData(url string) (*Response, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating http request: %w", err)
	}

	req.Header.Set("X-API-Key", c.Apps.APIKey)

	start := time.Now()

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("making http request: %w", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)

	defer c.debughttplog(resp, url, start, "", string(body))

	if err != nil {
		return nil, fmt.Errorf("reading http response body: %w", err)
	}

	return unmarshalResponse(url, resp.StatusCode, body)
}

// SendData sends raw data to a notifiarr URL as JSON.
func (c *Config) SendData(uri string, payload interface{}, log bool) (*Response, error) {
	var (
		post []byte
		err  error
	)

	if data, err := json.Marshal(payload); err == nil {
		var torn map[string]interface{}
		if err := json.Unmarshal(data, &torn); err == nil {
			if torn["host"], err = c.GetHostInfoUID(); err != nil {
				c.Errorf("Host Info Unknown: %v", err)
			}

			payload = torn
		}
	}

	if log {
		post, err = json.MarshalIndent(payload, "", " ")
	} else {
		post, err = json.Marshal(payload)
	}

	if err != nil {
		return nil, fmt.Errorf("encoding data to JSON (report this bug please): %w", err)
	}

	code, body, err := c.sendJSON(c.BaseURL+uri, post, log)
	if err != nil {
		return nil, err
	}

	return unmarshalResponse(c.BaseURL+uri, code, body)
}

// unmarshalResponse attempts to turn the reply from notifiarr.com into structured data.
func unmarshalResponse(url string, code int, body []byte) (*Response, error) {
	var r Response
	err := json.Unmarshal(body, &r)

	if code < http.StatusOK || code > http.StatusIMUsed {
		if err != nil {
			return nil, fmt.Errorf("%w: %s: %d %s (unmarshal error: %v)",
				ErrNon200, url, code, http.StatusText(code), err)
		}

		return nil, fmt.Errorf("%w: %s: %d %s, %s: %s",
			ErrNon200, url, code, http.StatusText(code), r.Result, r.Details.Response)
	}

	if err != nil {
		return nil, fmt.Errorf("converting json response: %w", err)
	}

	return &r, nil
}

// sendJSON posts a JSON payload to a URL. Returns the response body or an error.
func (c *Config) sendJSON(url string, data []byte, log bool) (int, []byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(data))
	if err != nil {
		return 0, nil, fmt.Errorf("creating http request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.Apps.APIKey)

	start := time.Now()

	resp, err := c.client.Do(req)
	if err != nil {
		c.debughttplog(nil, url, start, string(data), "")
		return 0, nil, fmt.Errorf("making http request: %w", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)

	if log {
		defer c.debughttplog(resp, url, start, string(data), string(body))
	} else {
		defer c.debughttplog(resp, url, start, "<data not logged>", string(body))
	}

	if err != nil {
		return resp.StatusCode, body, fmt.Errorf("reading http response body: %w", err)
	}

	return resp.StatusCode, body, nil
}

// Do performs an http Request with retries and logging!
func (h *httpClient) Do(req *http.Request) (*http.Response, error) {
	deadline, ok := req.Context().Deadline()
	if !ok {
		deadline = time.Now().Add(h.Timeout)
	}

	timeout := time.Until(deadline).Round(time.Millisecond)

	for i := 0; ; i++ {
		resp, err := h.Client.Do(req)
		if err == nil {
			for i, c := range resp.Cookies() {
				h.Printf("Unexpected cookie [%v/%v] returned from notifiarr.com: %s", i+1, len(resp.Cookies()), c.String())
			}

			if resp.StatusCode < http.StatusInternalServerError {
				return resp, nil
			}

			// resp.StatusCode is 500 or higher, make that en error.
			size, _ := io.Copy(io.Discard, resp.Body) // must read the entire body when err == nil
			resp.Body.Close()                         // do not defer, because we're in a loop.
			// shoehorn a non-200 error into the empty http error.
			err = fmt.Errorf("%w: %s: %d bytes, %s", ErrNon200, req.URL, size, resp.Status)
		}

		switch {
		case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
			if i == 0 {
				return resp, fmt.Errorf("notifiarr req timed out after %s: %s: %w", timeout, req.URL, err)
			}

			return resp, fmt.Errorf("[%d/%d] Notifiarr req timed out after %s, giving up: %w",
				i+1, h.Retries+1, timeout, err)
		case i == h.Retries:
			return resp, fmt.Errorf("[%d/%d] Notifiarr req failed: %w", i+1, h.Retries+1, err)
		default:
			h.Printf("[%d/%d] Notifiarr req failed, retrying in %s, error: %v", i+1, h.Retries+1, RetryDelay, err)
			time.Sleep(RetryDelay)
		}
	}
}

func (c *Config) debughttplog(resp *http.Response, url string, start time.Time, data, body string) {
	headers := ""
	status := "0"

	if resp != nil {
		status = resp.Status

		for k, vs := range resp.Header {
			for _, v := range vs {
				headers += k + ": " + v + "\n"
			}
		}
	}

	if c.MaxBody > 0 && len(body) > c.MaxBody {
		body = fmt.Sprintf("%s <body truncated, max: %d>", body[:c.MaxBody], c.MaxBody)
	}

	if c.MaxBody > 0 && len(data) > c.MaxBody {
		data = fmt.Sprintf("%s <data truncated, max: %d>", data[:c.MaxBody], c.MaxBody)
	}

	if data == "" {
		c.Debugf("Sent GET Request to %s in %s, Response (%s):\n%s\n%s",
			url, time.Since(start).Round(time.Microsecond), status, headers, body)
	} else {
		c.Debugf("Sent JSON Payload to %s in %s:\n%s\nResponse (%s):\n%s\n%s",
			url, time.Since(start).Round(time.Microsecond), data, status, headers, body)
	}
}
