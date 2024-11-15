package commands

/* This file contains the procedures that build the exec.Cmd and the (possibly) custom arguments. */

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strings"

	"github.com/Notifiarr/notifiarr/pkg/mnd"
	"github.com/hugelgupf/go-shlex"
)

// Errors produced by this file.
var (
	ErrArgCount = errors.New("wrong count of args provided")
	ErrArgValue = errors.New("provided argument value invalid")
)

type cmdBuilder struct {
	cmd          string
	expectedArgs []*regexp.Regexp
	providedArgs []string
	shell        bool
}

// getCmd returns the exec.Cmd for the provided arguments.
func (c *cmdBuilder) getCmd(ctx context.Context) (*exec.Cmd, error) {
	if len(c.expectedArgs) != len(c.providedArgs) {
		return nil, fmt.Errorf("%w: expected: %d, provided: %d", ErrArgCount, len(c.expectedArgs), len(c.providedArgs))
	}

	builtArgs, err := c.getArgs()
	if err != nil {
		return nil, err
	}

	var cmd *exec.Cmd
	//nolint:gosec
	switch len(builtArgs) {
	case 0:
		return nil, ErrNoCmd
	case 1:
		cmd = exec.CommandContext(ctx, builtArgs[0])
	default:
		cmd = exec.CommandContext(ctx, builtArgs[0], builtArgs[1:]...)
	}

	// This call hides the cmd window on Windows.
	// snapshot.SysCallSettings(cmd)

	return cmd, nil
}

func (c *cmdBuilder) getArgs() ([]string, error) {
	cmd := c.cmd

	for idx, reg := range c.expectedArgs {
		instance := idx + 1
		cmd = strings.Replace(cmd, fmt.Sprintf("%s%d%s", argPfx, instance, argSfx), c.providedArgs[idx], 1)

		if !reg.MatchString(c.providedArgs[idx]) {
			return nil, fmt.Errorf("%w: arg %d '%s' failed regexp", ErrArgValue, instance, c.providedArgs[idx])
		}
	}

	if runtime.GOOS != mnd.Windows && c.shell {
		return []string{"/bin/sh", "-c", cmd}, nil
	}

	// Special shell-split command.
	builtArgs := shlex.Split(cmd)
	if len(builtArgs) == 0 {
		return nil, ErrNoCmd
	}

	var err error
	if builtArgs[0], err = exec.LookPath(builtArgs[0]); err != nil && !errors.Is(err, exec.ErrDot) {
		return nil, fmt.Errorf("finding command path: %w", err)
	}

	if c.shell { // if shell is set, we know it's windows.
		return append([]string{"cmd", "/C"}, builtArgs...), nil
	}

	return builtArgs, nil
}
