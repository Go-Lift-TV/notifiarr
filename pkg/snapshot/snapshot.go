// Package snapshot generates system reports and sends them to notifiarr.com.
// The reports include zfs data, cpu, memory, mdadm info, megaraid arrays,
// smart status, mounted volume (disk) usage, cpu temp, other temps, uptime,
// drive age/health, logged on user count, etc. Works across most platforms.
// These snapshots are posted to a user's Chatroom on request.
package snapshot

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"github.com/Notifiarr/notifiarr/pkg/mnd"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"golift.io/cnfg"
	"golift.io/version"
)

// DefaultTimeout is used when one is not provided.
const DefaultTimeout = 45 * time.Second

const (
	minimumTimeout  = 20 * time.Second
	maximumTimeout  = 3 * time.Minute
	minimumInterval = time.Minute
	defaultMyLimit  = 10
)

// Config determines which checks to run, etc.
//
//nolint:lll
type Config struct {
	Timeout   cnfg.Duration `json:"timeout"       toml:"timeout"        xml:"timeout"`        // total run time allowed.
	Interval  cnfg.Duration `json:"interval"      toml:"interval"       xml:"interval"`       // how often to send snaps (cron).
	ZFSPools  []string      `json:"zfsPools"      toml:"zfs_pools"      xml:"zfs_pool"`       // zfs pools to monitor.
	UseSudo   bool          `json:"useSudo"       toml:"use_sudo"       xml:"use_sudo"`       // use sudo for smartctl commands.
	Raid      bool          `json:"monitorRaid"   toml:"monitor_raid"   xml:"monitor_raid"`   // include mdstat and/or megaraid.
	DriveData bool          `json:"monitorDrives" toml:"monitor_drives" xml:"monitor_drives"` // smartctl commands.
	DiskUsage bool          `json:"monitorSpace"  toml:"monitor_space"  xml:"monitor_space"`  // get disk usage.
	AllDrives bool          `json:"allDrives"     toml:"all_drives"     xml:"all_drives"`     // usage for all drives?
	Quotas    bool          `json:"quotas"        toml:"quotas"         xml:"quotas"`         // usage for user quotas?
	IOTop     int           `json:"ioTop"         toml:"iotop"          xml:"iotop"`          // number of processes to include from ioTop
	PSTop     int           `json:"psTop"         toml:"pstop"          xml:"pstop"`          // number of processes to include from top (cpu usage)
	MyTop     int           `json:"myTop"         toml:"mytop"          xml:"mytop"`          // number of processes to include from mysql servers.
	IPMI      bool          `json:"ipmi"          toml:"ipmi"           xml:"ipmi"`           // get ipmi sensor info.
	IPMISudo  bool          `json:"ipmiSudo"      toml:"ipmiSudo"       xml:"ipmiSudo"`       // use sudo to get ipmi sensor info.
	Plugins
}

// Plugins is optional configuration for "plugins".
type Plugins struct {
	Nvidia *NvidiaConfig  `json:"nvidia" toml:"nvidia" xml:"nvidia"`
	MySQL  []*MySQLConfig `json:"mysql"  toml:"mysql"  xml:"mysql"`
}

// Errors this package generates.
var (
	ErrPlatformUnsup = errors.New("the requested metric is not available on this platform, " +
		"if you know how to collect it, please open an issue on the github repo")
	ErrNonZeroExit = errors.New("cmd exited non-zero")
)

// Snapshot is the output data sent to Notifiarr.
type Snapshot struct {
	Debug   func(string, ...any) `json:"-"`
	Version string               `json:"version"`
	System  struct {
		*host.InfoStat
		Username string             `json:"username"`
		CPU      float64            `json:"cpuPerc"`
		MemFree  uint64             `json:"memFree"`
		MemUsed  uint64             `json:"memUsed"`
		MemTotal uint64             `json:"memTotal"`
		Temps    map[string]float64 `json:"temperatures,omitempty"`
		Users    int                `json:"users"`
		*load.AvgStat
		CPUTime cpu.TimesStat `json:"cpuTime"`
	} `json:"system"`
	Raid       *RaidData                      `json:"raid,omitempty"`
	DriveAges  map[string]int                 `json:"driveAges,omitempty"`
	DriveTemps map[string]int                 `json:"driveTemps,omitempty"`
	DiskHealth map[string]string              `json:"driveHealth,omitempty"`
	DiskUsage  map[string]*Partition          `json:"diskUsage,omitempty"`
	Quotas     map[string]*Partition          `json:"quotas,omitempty"`
	ZFSPool    map[string]*Partition          `json:"zfsPools,omitempty"`
	IOTop      *IOTopData                     `json:"ioTop,omitempty"`
	IOStat     *IoStatDisks                   `json:"ioStat,omitempty"`
	IOStat2    map[string]disk.IOCountersStat `json:"ioStat2,omitempty"`
	Processes  Processes                      `json:"processes,omitempty"`
	MySQL      map[string]*MySQLServerData    `json:"mysql,omitempty"`
	Nvidia     []*NvidiaOutput                `json:"nvidia,omitempty"`
	Sensors    []*IPMISensor                  `json:"ipmiSensors"`
	Synology   *Synology                      `json:"synology,omitempty"`
}

// RaidData contains raid information from mdstat and/or megacli.
type RaidData struct {
	MDstat  string     `json:"mdstat,omitempty"`
	MegaCLI []*MegaCLI `json:"megacli,omitempty"`
}

// Partition is used for ZFS pools as well as normal Disk arrays.
type Partition struct {
	Device   string   `json:"name"`
	Total    uint64   `json:"total"`
	Free     uint64   `json:"free"`
	Used     uint64   `json:"used"`
	FSType   string   `json:"fsType,omitempty"`
	ReadOnly bool     `json:"readOnly,omitempty"`
	Opts     []string `json:"opts,omitempty"`
}

// Validate makes sure the snapshot configuration is valid.
func (c *Config) Validate() {
	switch {
	case c.Timeout.Duration == 0:
		c.Timeout.Duration = DefaultTimeout
	case c.Timeout.Duration < minimumTimeout:
		c.Timeout.Duration = minimumTimeout
	case c.Timeout.Duration > maximumTimeout:
		c.Timeout.Duration = maximumTimeout
	}

	if c.Interval.Duration != 0 && c.Interval.Duration < minimumInterval {
		c.Interval.Duration = minimumInterval
	}

	if mnd.IsDocker || mnd.IsWindows {
		c.UseSudo = false
		c.IPMISudo = false
	}

	if mnd.IsDocker || !mnd.IsLinux {
		c.IOTop = 0
	}
}

// GetSnapshot returns a system snapshot based on requested data in the config.
func (c *Config) GetSnapshot(ctx context.Context, debugf func(string, ...any)) (*Snapshot, []error, []error) {
	ctx, cancel := context.WithTimeout(ctx, c.Timeout.Duration)
	defer cancel()

	snap := &Snapshot{Version: version.Version + "-" + version.Revision, Debug: debugf}
	errs, debug := c.getSnapshot(ctx, snap)

	return snap, errs, debug
}

func (c *Config) getSnapshot(ctx context.Context, snap *Snapshot) ([]error, []error) {
	errs := []error{snap.GetProcesses(ctx, c.PSTop), snap.GetCPUSample(ctx)}

	if err := snap.GetLocalData(ctx); len(err) != 0 {
		errs = append(errs, err...)
	}

	var err error
	if snap.Synology, err = GetSynology(true); err != nil && !errors.Is(err, ErrNotSynology) {
		errs = append(errs, err)
	} else if snap.Synology != nil {
		snap.Synology.SetInfo(snap.System.InfoStat)
	}

	if err := snap.getDisksUsage(ctx, c.DiskUsage, c.AllDrives); len(err) != 0 {
		errs = append(errs, err...)
	}

	if err := snap.getQuota(ctx, c.Quotas); err != nil {
		errs = append(errs, err)
	}

	var debug []error

	if err := snap.getDriveData(ctx, c.DriveData, c.UseSudo); len(err) != 0 {
		debug = append(debug, err...) // these can be noisy, so debug/hide them.
	}

	if err := snap.GetMySQL(ctx, c.Plugins.MySQL, c.MyTop); len(err) != 0 {
		errs = append(errs, err...)
	}

	errs = append(errs, snap.GetMemoryUsage(ctx))
	errs = append(errs, snap.getZFSPoolData(ctx, c.ZFSPools))
	errs = append(errs, snap.getRaidData(ctx, c.UseSudo, c.Raid))
	errs = append(errs, snap.getSystemTemps(ctx))
	errs = append(errs, snap.getIOTop(ctx, c.UseSudo, c.IOTop))
	errs = append(errs, snap.getIoStat(ctx, c.DiskUsage && mnd.IsLinux))
	errs = append(errs, snap.getIoStat2(ctx, c.DiskUsage))
	errs = append(errs, snap.GetNvidia(ctx, c.Nvidia))
	errs = append(errs, snap.GetIPMI(ctx, c.IPMI, c.IPMISudo))

	return errs, debug
}

/*******************************************************/
/*********************** HELPERS ***********************/
/*******************************************************/

// readyCommand gets a command ready for output capture.
func readyCommand(
	ctx context.Context,
	useSudo bool,
	run string,
	args ...string,
) (*exec.Cmd, *bufio.Scanner, *sync.WaitGroup, error) {
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
	SysCallSettings(cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("%s stdout error: %w", cmdPath, err)
	}

	return cmd, bufio.NewScanner(stdout), &sync.WaitGroup{}, nil
}

// runCommand executes the readied command and waits for the output loop to finish.
func runCommand(cmd *exec.Cmd, waitg *sync.WaitGroup) error {
	waitg.Add(1)

	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr
	err := cmd.Run() //nolint:ifshort

	waitg.Wait()

	if err != nil {
		return fmt.Errorf("%v %w: %s", cmd.Args, err, stderr)
	}

	return nil
}
