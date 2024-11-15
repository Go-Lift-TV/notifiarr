package checkapp

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"

	"github.com/Notifiarr/notifiarr/pkg/snapshot"
)

func testMySQL(ctx context.Context, config *snapshot.MySQLConfig) (string, int) {
	snaptest := &snapshot.Snapshot{}

	errs := snaptest.GetMySQL(ctx, []*snapshot.MySQLConfig{config}, 1)
	if len(errs) > 0 {
		msg := fmt.Sprintf("%d errors encountered: ", len(errs))
		for _, err := range errs {
			msg += err.Error()
		}

		return msg, http.StatusBadGateway
	}

	return "Connection Successful!", http.StatusOK
}

func testNvidia(ctx context.Context, config *snapshot.NvidiaConfig) (string, int) {
	if config == nil {
		return ErrBadIndex.Error(), http.StatusBadRequest
	} else if config.SMIPath != "" {
		if _, err := os.Stat(config.SMIPath); err != nil {
			return fmt.Sprintf("nvidia-smi not found at provided path '%s': %v", config.SMIPath, err), http.StatusNotAcceptable
		}
	} else if _, err := exec.LookPath("nvidia-smi"); err != nil {
		return fmt.Sprintf("unable to locate nvidia-smi in PATH '%s'", os.Getenv("PATH")), http.StatusNotAcceptable
	}

	snaptest := &snapshot.Snapshot{}
	config.Disabled = false

	if err := snaptest.GetNvidia(ctx, config); err != nil {
		return err.Error(), http.StatusFailedDependency
	}

	msg := fmt.Sprintf("SMI found %d Graphics Adapter", len(snaptest.Nvidia))

	switch len(snaptest.Nvidia) {
	case 0:
		msg += "s."
	case 1:
		msg += ":"
	default:
		msg += "s:"
	}

	for _, adapter := range snaptest.Nvidia {
		msg += "<br>" + adapter.BusID
	}

	return msg, http.StatusOK
}
