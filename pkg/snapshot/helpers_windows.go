package snapshot

import (
	"os/exec"
	"syscall"
)

func SysCallSettings(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} //nolint:mnd
}
