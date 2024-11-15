// Package plex provides the methods the Notifiarr client uses to interface with Plex.
// This package also provides a web handler for incoming plex webhooks, and another
// two handlers for requests from Notifiarr.com to list sessions and kill a session.
// The purpose is to keep track of Plex viewers and send meaningful alerts to their
// respective Discord server about user behavior.
// ie. user started watching something, paused it, resumed it, and finished something.
// This package can be disabled by not providing a Plex Media Server URL or Token.
package plex

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Server is the Plex configuration from a config file.
// Without a URL or Token, nothing works and this package is unused.
type Server struct {
	config Config
	name   string
}

type Config struct {
	URL    string       `json:"url"   toml:"url"   xml:"url"`
	Token  string       `json:"token" toml:"token" xml:"token"`
	Client *http.Client `json:"-"     toml:"-"     xml:"-"`
}

// New turns a config into a server.
func New(config *Config) *Server {
	if config.Client == nil {
		config.Client = &http.Client{
			Timeout: time.Minute,
		}
	}

	return &Server{
		config: *config,
	}
}

// Name returns the server name.
func (s *Server) Name() string {
	return s.name
}

// ErrNoURLToken is returned when there is no token or URL.
var ErrNoURLToken = errors.New("token or URL for Plex missing")

func (s *Server) getPlexURL(ctx context.Context, url string, params url.Values) ([]byte, error) {
	return s.reqPlexURL(ctx, url, http.MethodGet, params, nil)
}

func (s *Server) putPlexURL(ctx context.Context, url string, params url.Values, putData io.Reader) ([]byte, error) {
	return s.reqPlexURL(ctx, url, http.MethodPut, params, putData)
}

/*
func (s *Server) postPlexURL(ctx context.Context, url string, params url.Values, postData io.Reader) ([]byte, error) {
	return s.reqPlexURL(ctx, url, http.MethodPost, params, postData)
}
*/

func (s *Server) reqPlexURL(
	ctx context.Context,
	url, method string,
	params url.Values,
	sendData io.Reader,
) ([]byte, error) {
	if s.config.URL == "" || s.config.Token == "" {
		return nil, ErrNoURLToken
	}

	req, err := http.NewRequestWithContext(ctx, method, url, sendData)
	if err != nil {
		return nil, fmt.Errorf("creating http request: %w", err)
	}

	req.URL.RawQuery = params.Encode()
	req.Header.Set("X-Plex-Token", s.config.Token)
	req.Header.Set("Accept", "application/json")

	resp, err := s.config.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("making http request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading http response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return body, fmt.Errorf("%w: %s", ErrBadStatus, resp.Status)
	}

	return body, nil
}
