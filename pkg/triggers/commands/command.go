// package commands provides the interfaces and structures to trigger and run shell commands.
package commands

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/Notifiarr/notifiarr/pkg/mnd"
	"github.com/Notifiarr/notifiarr/pkg/triggers/common"
	"github.com/Notifiarr/notifiarr/pkg/website"
	"github.com/hugelgupf/go-shlex"
)

var ErrDisabled = errors.New("the command is disabled due to an error")

const hashLen = 64

const (
	argPfx = "({"
	argSfx = "})"
)

// Setup must run in the creation routine.
func (c *Command) Setup(logger mnd.Logger, website *website.Server) {
	if c.Name == "" {
		if args := shlex.Split(c.Command); len(args) > 0 {
			c.Name = args[0]
		}
	}

	if len(c.Hash) != hashLen {
		hash := sha256.New()
		hash.Write([]byte(fmt.Sprint(time.Now(), c.Name, c.Command, c.Timeout)))
		c.Hash = hex.EncodeToString(hash.Sum(nil))
	}

	c.log = logger
	c.website = website
	c.Args = len(c.args)

	if c.Timeout.Duration == 0 {
		c.Timeout.Duration = defaultTimeout
	}
}

func (c *Command) SetupRegexpArgs() error {
	c.cmd = c.Command

	pfxs := strings.Count(c.Command, argPfx)
	if sfxs := strings.Count(c.Command, argSfx); pfxs != sfxs {
		return fmt.Errorf("%w: regexp pfx/sfx mismatch, pfx %s count %d, sfx %s count: %d",
			ErrArgValue, argPfx, pfxs, argSfx, sfxs)
	}

	matches := regexp.MustCompile(`\({([^}]*)}\)`).FindAllStringIndex(c.cmd, -1)
	if matches == nil {
		return nil
	}

	c.args = make([]*regexp.Regexp, len(matches))

	for idx := len(matches) - 1; idx >= 0; idx-- {
		arg := matches[idx]
		instance := idx + 1

		regex, err := regexp.Compile(c.cmd[arg[0]+2 : arg[1]-2])
		if err != nil {
			return fmt.Errorf("parsing command '%s' arg %d regexp: %w", c.Name, instance, err)
		}

		c.cmd = c.cmd[:arg[0]] + fmt.Sprintf("%s%d%s", argPfx, instance, argSfx) + c.cmd[arg[1]:]
		c.args[idx] = regex
	}

	return nil
}

// run executes this command and logs the output. This is executed from the trigger channel.
func (c *Command) run(ctx context.Context, input *common.ActionInput) {
	_, _ = c.RunNow(ctx, input)
}

// RunNow runs the command immediately, waits for and returns the output.
func (c *Command) RunNow(ctx context.Context, input *common.ActionInput) (string, error) {
	if c.disable {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.output = ErrDisabled.Error()

		return "<command disabled>", ErrDisabled
	}

	output, elapsed, err := c.exec(ctx, input)
	oStr := output.String()
	eStr := ""

	if err != nil {
		eStr = err.Error()
	}

	// Send the notification before the lock.
	if c.Notify {
		c.website.SendData(&website.Request{
			Route: website.CommandRoute,
			Event: input.Type,
			Payload: map[string]string{
				"name":   c.Name,
				"hash":   c.Hash,
				"output": oStr,
				"error":  eStr,
			},
			LogMsg:     fmt.Sprintf("Custom Command '%s' Output (elapsed: %s)", c.Name, elapsed.Round(time.Millisecond)),
			LogPayload: c.Log,
		})
	}

	c.logOutput(input, oStr, eStr, elapsed, err)

	return oStr, err
}

func (c *Command) logOutput(input *common.ActionInput, oStr, eStr string, elapsed time.Duration, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.runs++
	c.lastRun = time.Now().Round(time.Second)
	c.output = oStr
	c.lastArg = input.Args

	extra := ""
	if len(c.lastArg) > 0 {
		extra = ", args:"
		for idx, arg := range c.lastArg {
			extra += fmt.Sprintf(" %d: %q", idx+1, arg)
		}
	}

	if err != nil {
		c.fails++
		c.output = "error: " + eStr + ", output: " + oStr

		if c.Log && oStr != "" {
			c.log.Errorf("[%s requested] Custom Command '%s%s' Failed (elapsed: %s): %v, Output:\n%s",
				input.Type, c.Name, extra, elapsed.Round(time.Millisecond), err, oStr)
		} else {
			c.log.Errorf("[%s requested] Custom Command '%s%s' Failed (elapsed: %s): %v",
				input.Type, c.Name, extra, elapsed.Round(time.Millisecond), err)
		}
	} else if c.Log && oStr != "" {
		c.log.Printf("[%s requested] Custom Command '%s%s' Output (elapsed: %s):\n%s",
			input.Type, c.Name, extra, elapsed.Round(time.Millisecond), oStr)
	}
}

// run read locks and runs the command then returns the output.
func (c *Command) exec(ctx context.Context, input *common.ActionInput) (*bytes.Buffer, time.Duration, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	ctx, cancel := context.WithTimeout(ctx, c.Timeout.Duration)
	defer cancel()

	builder := &cmdBuilder{
		cmd:          c.cmd,
		expectedArgs: c.args,
		providedArgs: input.Args,
		shell:        c.Shell,
	}

	cmd, err := builder.getCmd(ctx)
	if err != nil {
		return nil, 0, err
	}

	var out bytes.Buffer

	if cmd.Stdout != nil {
		cmd.Stdout = io.MultiWriter(&out, cmd.Stdout)
	}

	if cmd.Stderr != nil {
		cmd.Stderr = io.MultiWriter(&out, cmd.Stderr)
	}

	start := time.Now()
	if err := cmd.Run(); err != nil {
		return &out, time.Since(start), fmt.Errorf(`running cmd %s: %w`, cmd.Args, err)
	}

	return &out, time.Since(start), nil
}
