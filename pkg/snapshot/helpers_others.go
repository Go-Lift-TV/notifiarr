//go:build !windows

package snapshot

import (
	"os/exec"
)

func SysCallSettings(_ *exec.Cmd) {}
