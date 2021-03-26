package snapshot

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"golift.io/cnfg"
)

const (
	defaultTimeout  = 10 * time.Second
	minimumTimeout  = 5 * time.Second
	minimumInterval = 10 * time.Minute
)

// Config determines which checks to run, etc.
type Config struct {
	Timeout   cnfg.Duration `toml:"timeout"`                  // total run time allowed.
	Interval  cnfg.Duration `toml:"interval"`                 // how often to send snaps (cron).
	ZFSPools  []string      `toml:"zfs_pools" xml:"zfs_pool"` // zfs pools to monitor.
	UseSudo   bool          `toml:"use_sudo"`                 // use sudo for smartctl commands.
	Raid      bool          `toml:"monitor_raid"`             // include mdstat and/or megaraid.
	DriveData bool          `toml:"monitor_drives"`           // smartctl commands.
	DiskUsage bool          `toml:"monitor_space"`            // get disk usage.
	Uptime    bool          `toml:"monitor_uptime"`           // all system stats.
	CPUMem    bool          `toml:"monitor_cpuMemory"`        // cpu perct and memory used/free.
	CPUTemp   bool          `toml:"monitor_cpuTemp"`          // not everything supports temps.
	synology  bool
}

// Errors this package generates.
var (
	ErrPlatformUnsup = fmt.Errorf("the requested metric is not available on this platform, " +
		"if you know how to collect it, please open an issue on the github repo")
	ErrNonZeroExit = fmt.Errorf("cmd exited non-zero")
)

// Snapshot is the output data sent to Notifiarr.
type Snapshot struct {
	System struct {
		*host.InfoStat
		Username string             `json:"username"`
		CPU      float64            `json:"cpuPerc"`
		MemFree  uint64             `json:"memFree"`
		MemUsed  uint64             `json:"memUsed"`
		MemTotal uint64             `json:"memTotal"`
		Temps    map[string]float64 `json:"temperatures,omitempty"`
		Users    int                `json:"users"`
		*load.AvgStat
	} `json:"system"`
	Raid       *RaidData             `json:"raid,omitempty"`
	DriveAges  map[string]int        `json:"driveAges,omitempty"`
	DriveTemps map[string]int        `json:"driveTemps,omitempty"`
	DiskUsage  map[string]*Partition `json:"diskUsage,omitempty"`
	DiskHealth map[string]string     `json:"driveHealth,omitempty"`
	ZFSPool    map[string]*Partition `json:"zfsPools,omitempty"`
	synology   bool
}

// RaidData contains raid information from mdstat and/or megacli.
type RaidData struct {
	MDstat  string `json:"mdstat,omitempty"`
	MegaCLI string `json:"megacli,omitempty"`
}

// Partition is used for ZFS pools as well as normal Disk arrays.
type Partition struct {
	Device string `json:"name"`
	Total  uint64 `json:"total"`
	Free   uint64 `json:"free"`
}

func (c *Config) Validate() {
	if c.Timeout.Duration < minimumTimeout {
		c.Timeout.Duration = minimumTimeout
	}

	if c.Interval.Duration == 0 {
		return
	} else if c.Interval.Duration < minimumInterval {
		c.Interval.Duration = minimumInterval
	}

	if _, err := os.Stat(synologyConf); err == nil {
		c.synology = true
	}
}

// GetSnapshot returns a system snapshot based on requested data in the config.
func (c *Config) GetSnapshot() (*Snapshot, []error, []error) {
	if c.Timeout.Duration == 0 {
		c.Timeout.Duration = defaultTimeout
	} else if c.Timeout.Duration < minimumTimeout {
		c.Timeout.Duration = minimumTimeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout.Duration)
	defer cancel()

	s := &Snapshot{synology: c.synology}
	errs, debug := c.getSnapshot(ctx, s)

	return s, errs, debug
}

func (c *Config) getSnapshot(ctx context.Context, s *Snapshot) ([]error, []error) {
	var errs, debug []error

	if err := s.GetLocalData(ctx, c.Uptime); len(err) != 0 {
		errs = append(errs, err...)
	}

	if err := s.GetSynology(c.Uptime); err != nil {
		errs = append(errs, err)
	}

	if err := s.getDisksUsage(ctx, c.DiskUsage); len(err) != 0 {
		errs = append(errs, err...)
	}

	if err := s.getDriveData(ctx, c.DriveData, c.UseSudo); len(err) != 0 {
		debug = append(debug, err...) // these can be noisy, so debug/hide them.
	}

	errs = append(errs, s.GetCPUSample(ctx, c.CPUMem))
	errs = append(errs, s.GetMemoryUsage(ctx, c.CPUMem))
	errs = append(errs, s.getZFSPoolData(ctx, c.ZFSPools))
	errs = append(errs, s.getRaidData(ctx, c.Raid))
	errs = append(errs, s.getSystemTemps(ctx, c.CPUTemp))

	return errs, debug
}

/*******************************************************/
/*********************** HELPERS ***********************/
/*******************************************************/

// readyCommand gets a command ready for output capture.
func readyCommand(ctx context.Context, useSudo bool, run string, args ...string) (
	*exec.Cmd, *bufio.Scanner, *sync.WaitGroup, error) {
	cmdPath, err := exec.LookPath(run)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("%s missing! %w", run, err)
	}

	if args == nil { // avoid nil pointer deref.
		args = []string{}
	}

	if useSudo {
		args = append([]string{"-n", cmdPath}, args...)

		if cmdPath, err = exec.LookPath("sudo"); err != nil {
			return nil, nil, nil, fmt.Errorf("sudo missing! %w", err)
		}
	}

	cmd := exec.CommandContext(ctx, cmdPath, args...)
	sysCallSettings(cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("%s stdout error: %w", cmdPath, err)
	}

	return cmd, bufio.NewScanner(stdout), &sync.WaitGroup{}, nil
}

// runCommand executes the readied command and waits for the output loop to finish.
func runCommand(cmd *exec.Cmd, wg *sync.WaitGroup) error {
	wg.Add(1)

	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr
	err := cmd.Run()

	wg.Wait()

	if err != nil {
		return fmt.Errorf("%v %w: %s", cmd.Args, err, stderr)
	}

	if exitCode := cmd.ProcessState.ExitCode(); exitCode != 0 {
		return fmt.Errorf("%v %w (%d): %s", cmd.Args, ErrNonZeroExit, exitCode, stderr)
	}

	return nil
}
