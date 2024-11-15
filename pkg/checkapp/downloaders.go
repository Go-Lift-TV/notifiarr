package checkapp

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/Notifiarr/notifiarr/pkg/apps"
	"github.com/Notifiarr/notifiarr/pkg/mnd"
	"github.com/hekmon/transmissionrpc/v3"
	"golift.io/deluge"
	"golift.io/nzbget"
	"golift.io/qbit"
	"golift.io/version"
)

func testQbit(ctx context.Context, config *apps.QbitConfig) (string, int) {
	qbit, err := qbit.New(ctx, config.Config)
	if err != nil {
		return connecting + err.Error(), http.StatusBadRequest
	}

	xfers, err := qbit.GetXfersContext(ctx)
	if err != nil {
		return "Getting Transfers: " + err.Error(), http.StatusBadRequest
	}

	return fmt.Sprintf("Connection Successful! %d Transfers", len(xfers)), http.StatusOK
}

func testRtorrent(_ context.Context, config *apps.RtorrentConfig) (string, int) {
	config.Setup(0, nil)

	result, err := config.Client.Call("system.hostname")
	if err != nil {
		return "Getting Server Name: " + err.Error(), http.StatusFailedDependency
	}

	if names, ok := result.([]any); ok {
		result = names[0]
	}

	if name, ok := result.(string); ok {
		return "Connection Successful! Server name: " + name, http.StatusOK
	}

	return "Getting Server Name: result was not a string?", http.StatusFailedDependency
}

func testSabNZB(ctx context.Context, app *apps.SabNZBConfig) (string, int) {
	app.Setup(0, nil)

	sab, err := app.GetQueue(ctx)
	if err != nil {
		return "Getting Queue: " + err.Error(), http.StatusFailedDependency
	}

	return success + sab.Version, http.StatusOK
}

func testNZBGet(ctx context.Context, config *apps.NZBGetConfig) (string, int) {
	ver, err := nzbget.New(config.Config).VersionContext(ctx)
	if err != nil {
		return "Getting Version: " + err.Error(), http.StatusFailedDependency
	}

	return fmt.Sprintf("%s%s", success, ver), http.StatusOK
}

func testDeluge(ctx context.Context, config *apps.DelugeConfig) (string, int) {
	deluge, err := deluge.New(ctx, config.Config)
	if err != nil {
		return connecting + err.Error(), http.StatusFailedDependency
	}

	return fmt.Sprintf("%s%s", success, deluge.Version), http.StatusOK
}

func testTransmission(ctx context.Context, config *apps.XmissionConfig) (string, int) {
	endpoint, err := url.Parse(config.URL)
	if err != nil {
		return "parsing url: " + err.Error(), http.StatusFailedDependency
	} else if config.User != "" {
		endpoint.User = url.UserPassword(config.User, config.Pass)
	}

	client, _ := transmissionrpc.New(endpoint, &transmissionrpc.Config{
		UserAgent: fmt.Sprintf("%s v%s-%s %s", mnd.Title, version.Version, version.Revision, version.Branch),
	})

	args, err := client.SessionArgumentsGetAll(ctx)
	if err != nil {
		return "Getting Server Version: " + err.Error(), http.StatusFailedDependency
	}

	return "Transmission Server version: " + *args.Version, http.StatusOK
}
