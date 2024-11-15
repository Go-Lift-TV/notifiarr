package checkapp

import (
	"context"
	"fmt"
	"net/http"

	"github.com/Notifiarr/notifiarr/pkg/apps"
)

func testPlex(ctx context.Context, app *apps.PlexConfig) (string, int) {
	app.Setup(0, nil)

	info, err := app.GetInfo(ctx)
	if err != nil {
		return "Getting Info: " + err.Error(), http.StatusFailedDependency
	}

	return "Plex OK! Version: " + info.Version, http.StatusOK
}

func testTautulli(ctx context.Context, app *apps.TautulliConfig) (string, int) {
	app.Setup(0, nil)

	users, err := app.GetUsers(ctx)
	if err != nil {
		return "Getting Users: " + err.Error(), http.StatusFailedDependency
	}

	return fmt.Sprintf("Tautulli OK! Users: %d", len(users.Response.Data)), http.StatusOK
}
