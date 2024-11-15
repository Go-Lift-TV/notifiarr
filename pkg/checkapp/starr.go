package checkapp

import (
	"context"
	"net/http"

	"github.com/Notifiarr/notifiarr/pkg/apps"
	"golift.io/starr/lidarr"
	"golift.io/starr/prowlarr"
	"golift.io/starr/radarr"
	"golift.io/starr/readarr"
	"golift.io/starr/sonarr"
)

func testLidarr(ctx context.Context, config *apps.LidarrConfig) (string, int) {
	status, err := lidarr.New(config.Config).GetSystemStatusContext(ctx)
	if err != nil {
		return connecting + err.Error(), http.StatusFailedDependency
	}

	return success + status.Version, http.StatusOK
}

func testProwlarr(ctx context.Context, config *apps.ProwlarrConfig) (string, int) {
	status, err := prowlarr.New(config.Config).GetSystemStatusContext(ctx)
	if err != nil {
		return connecting + err.Error(), http.StatusFailedDependency
	}

	return success + status.Version, http.StatusOK
}

func testRadarr(ctx context.Context, config *apps.RadarrConfig) (string, int) {
	status, err := radarr.New(config.Config).GetSystemStatusContext(ctx)
	if err != nil {
		return connecting + err.Error(), http.StatusFailedDependency
	}

	return success + status.Version, http.StatusOK
}

func testReadarr(ctx context.Context, config *apps.ReadarrConfig) (string, int) {
	status, err := readarr.New(config.Config).GetSystemStatusContext(ctx)
	if err != nil {
		return connecting + err.Error(), http.StatusFailedDependency
	}

	return success + status.Version, http.StatusOK
}

func testSonarr(ctx context.Context, config *apps.SonarrConfig) (string, int) {
	status, err := sonarr.New(config.Config).GetSystemStatusContext(ctx)
	if err != nil {
		return connecting + err.Error(), http.StatusFailedDependency
	}

	return success + status.Version, http.StatusOK
}
