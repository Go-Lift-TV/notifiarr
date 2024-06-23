package update

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/Notifiarr/notifiarr/pkg/mnd"
)

type UnstableFile struct {
	Time time.Time `json:"time"`
	File string    `json:"file"`
	Ver  string    `json:"version"`
	Rev  int       `json:"revision"`
	Size int64     `json:"size"`
}

// LatestUS is where we find the latest unstable.
const unstableURL = "https://unstable.golift.io"

// CheckUnstable checks if the provided app has an updated version on GitHub.
// Pass in revision only, no version.
func CheckUnstable(ctx context.Context, app string, revision string) (*Update, error) {
	app = strings.ToLower(app)
	uri := fmt.Sprintf("%s/%s/%s.%s.exe.zip", unstableURL, app, app, runtime.GOARCH)

	if mnd.IsDarwin {
		uri = fmt.Sprintf("%s/%s/%s.dmg", unstableURL, app, app)
	} else if !mnd.IsWindows {
		uri = fmt.Sprintf("%s/%s/%s.%s.%s.gz", unstableURL, app, app, runtime.GOARCH, runtime.GOOS)
	}

	release, err := GetUnstable(ctx, uri)
	if err != nil {
		return nil, err
	}

	oldRev, _ := strconv.Atoi(revision)

	return &Update{
		RelDate: release.Time,
		CurrURL: release.File,
		Current: fmt.Sprint(release.Ver, "-", release.Rev),
		Version: revision, // on well.
		RelSize: release.Size,
		Outdate: release.Rev > oldRev,
	}, nil
}

// GetUnstable returns an unstable release. See CheckUnstable for an example on how to use it.
func GetUnstable(ctx context.Context, uri string) (*UnstableFile, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	release := UnstableFile{File: uri}
	uri += ".txt"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return nil, fmt.Errorf("requesting %s: %w", uri, err)
	}

	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("querying %s: %w", uri, err)
	}
	defer resp.Body.Close()

	if err = json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("decoding %s response: %w", uri, err)
	}

	release.Time, _ = time.Parse(time.RFC1123, resp.Header.Get("last-modified"))

	return &release, nil
}
