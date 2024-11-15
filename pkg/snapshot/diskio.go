//nolint:tagliatelle
package snapshot

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/Notifiarr/notifiarr/pkg/mnd"
	"github.com/shirou/gopsutil/v4/disk"
)

/* This files has procedures to structure data from iotop and iostat. */

// IOTopData is the data structure for iotop output.
type IOTopData struct {
	TotalRead  float64    `json:"totalRead"`
	TotalWrite float64    `json:"totalWrite"`
	CurrRead   float64    `json:"currentRead"`
	CurrWrite  float64    `json:"currentWrite"`
	Processes  IOTopProcs `json:"procs"`
}

// IOTopProcs is part of IOTopData.
type IOTopProcs []*IOTopProc

// IOTopProc is part of IOTopData.
type IOTopProc struct {
	Pid       int     `json:"pid"`
	Priority  string  `json:"prio"`
	User      string  `json:"user"`
	DiskRead  float64 `json:"diskRead"`
	DiskWrite float64 `json:"diskWrite"`
	SwapIn    float64 `json:"swapIn"`
	IO        float64 `json:"io"`
	Command   string  `json:"command"`
}

// IoStatData is the data structure for iostat output.
type IoStatData struct {
	Sysstat struct {
		Hosts []*IoStatHost `json:"hosts"`
	} `json:"sysstat"`
}

// IoStatHost is part of IoStatData.
type IoStatHost struct {
	Nodename     string `json:"nodename"`
	Sysname      string `json:"sysname"`
	Release      string `json:"release"`
	Machine      string `json:"machine"`
	NumberOfCpus int    `json:"number-of-cpus"`
	Date         string `json:"date"`
	Statistics   []struct {
		Disk IoStatDisks `json:"disk"`
	} `json:"statistics"`
}

// IoStatDisks is part of IoStatData.
type IoStatDisks []*IoStatDisk

// IoStatDisk is part of IoStatData.
type IoStatDisk struct {
	DiskDevice string  `json:"disk_device"`
	RS         float64 `json:"r/s"`
	WS         float64 `json:"w/s"`
	DS         float64 `json:"d/s"`
	RkBS       float64 `json:"rkB/s"`
	WkBS       float64 `json:"wkB/s"`
	DkBS       float64 `json:"dkB/s"`
	RrqmS      float64 `json:"rrqm/s"`
	WrqmS      float64 `json:"wrqm/s"`
	DrqmS      float64 `json:"drqm/s"`
	Rrqm       float64 `json:"rrqm"`
	Wrqm       float64 `json:"wrqm"`
	Drqm       float64 `json:"drqm"`
	RAwait     float64 `json:"r_await"`
	WAwait     float64 `json:"w_await"`
	DAwait     float64 `json:"d_await"`
	RareqSz    float64 `json:"rareq-sz"`
	WareqSz    float64 `json:"wareq-sz"`
	DareqSz    float64 `json:"dareq-sz"`
	AquSz      float64 `json:"aqu-sz"`
	Util       float64 `json:"util"`
}

func (s *Snapshot) getIOTop(ctx context.Context, useSudo bool, procs int) error {
	if procs < 1 {
		return nil
	}

	args := []string{"--batch", "--only", "--iter=2", "--kilobytes"}

	cmd, stdout, waitg, err := readyCommand(ctx, useSudo, "iotop", args...)
	if err != nil {
		return err
	}

	s.IOTop = &IOTopData{}

	go s.scanIOTop(stdout, waitg)

	defer func() {
		sort.Sort(s.IOTop.Processes)
		s.IOTop.Processes.Shrink(procs)
	}()

	return runCommand(cmd, waitg)
}

// scanIOTop turns the iotop output into structured data using a Scanner.
func (s *Snapshot) scanIOTop(stdout *bufio.Scanner, wg *sync.WaitGroup) {
	defer wg.Done()

	regex := regexp.MustCompile(`[0-9]+\.[0-9]+`)
	captured := map[string]*IOTopProc{} // used to de-dup by pid.

	for stdout.Scan() {
		text := stdout.Text()

		switch fields := strings.Fields(text); {
		case strings.Contains(text, "illegal option"):
			return
			// it's a bad command wrong OS.
		case len(fields) < 10, fields[0] == "PID": //nolint:mnd
			// PID  PRIO  USER     DISK READ  DISK WRITE  SWAPIN      IO    COMMAND
			// not enough fields, or header row.
			continue
		case fields[0] == "Total":
			//	Total DISK READ:         0.00 K/s | Total DISK WRITE:         0.00 K/s
			if nums := regex.FindAllString(text, 2); len(nums) == 2 { //nolint:mnd
				s.IOTop.TotalRead, _ = strconv.ParseFloat(nums[0], mnd.Bits64)
				s.IOTop.TotalWrite, _ = strconv.ParseFloat(nums[1], mnd.Bits64)
				s.IOTop.TotalRead *= mnd.Kilobyte  // convert to bytes.
				s.IOTop.TotalWrite *= mnd.Kilobyte // convert to bytes.
			}
		case fields[0] == "Current", fields[0] == "Actual":
			//	Current DISK READ:       0.00 K/s | Current DISK WRITE:       0.00 K/s
			if nums := regex.FindAllString(text, 2); len(nums) == 2 { //nolint:mnd
				s.IOTop.CurrRead, _ = strconv.ParseFloat(nums[0], mnd.Bits64)
				s.IOTop.CurrWrite, _ = strconv.ParseFloat(nums[1], mnd.Bits64)
				s.IOTop.CurrRead *= mnd.Kilobyte  // convert to bytes.
				s.IOTop.CurrWrite *= mnd.Kilobyte // convert to bytes.
			}
		case len(fields) >= 12: //nolint:mnd
			// 780711 be/4 david       0.00 K/s    0.00 K/s  0.00 %  0.00 % pulseaudio --daemonize=no --log-target=journal
			proc := &IOTopProc{
				// Pid:   fields[0]
				Priority: fields[1],
				User:     fields[2],
				// DiskRead:  fields[3],
				// DiskWrite: fields[5],
				// SwapIn:    fields[7],
				// IO:        fields[9],
				Command: strings.Join(fields[11:], " "),
			}
			proc.Pid, _ = strconv.Atoi(fields[0])
			proc.DiskRead, _ = strconv.ParseFloat(fields[3], mnd.Bits64)
			proc.DiskWrite, _ = strconv.ParseFloat(fields[5], mnd.Bits64)
			proc.SwapIn, _ = strconv.ParseFloat(fields[7], mnd.Bits64)
			proc.IO, _ = strconv.ParseFloat(fields[9], mnd.Bits64)
			proc.DiskRead *= mnd.Kilobyte
			proc.DiskWrite *= mnd.Kilobyte
			captured[fields[0]] = proc
		}
	}

	for _, proc := range captured {
		s.IOTop.Processes = append(s.IOTop.Processes, proc)
	}
}

// getIoStat only works with newer versions of sysstat on linux.
func (s *Snapshot) getIoStat(ctx context.Context, run bool) error {
	if !run {
		return nil
	}

	cmdPath, err := exec.LookPath("iostat")
	if err != nil {
		// do not throw an error if iostat is missing.
		// return fmt.Errorf("iostat missing! %w", err)
		return nil //nolint:nilerr
	}

	cmd := exec.CommandContext(ctx, cmdPath, "-x", "-d", "-o", "JSON")
	SysCallSettings(cmd)

	stderr := &bytes.Buffer{}
	stdout := &bytes.Buffer{}
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		e := stderr.String()
		if strings.Contains(e, "illegal option") {
			return nil
		}

		return fmt.Errorf("%v: %w: %s", cmd.Args, err, e)
	}

	var output IoStatData
	if err = json.NewDecoder(stdout).Decode(&output); err != nil {
		return fmt.Errorf("%v: output error: %w, %s", cmd.Args, err, stderr)
	}

	if len(output.Sysstat.Hosts) > 0 && len(output.Sysstat.Hosts[0].Statistics) > 0 {
		s.IOStat = &output.Sysstat.Hosts[0].Statistics[0].Disk
	}

	return nil
}

// getIoStat2 works on most platforms, but returns unusual data.
func (s *Snapshot) getIoStat2(ctx context.Context, run bool) error {
	if !run {
		return nil
	}

	var err error
	if s.IOStat2, err = disk.IOCountersWithContext(ctx); err != nil {
		return fmt.Errorf("disk IO counters: %w", err)
	}

	return nil
}

// Len allows us to sort IOTopProcs.
func (s IOTopProcs) Len() int {
	return len(s)
}

// Swap allows us to sort IOTopProcs.
func (s IOTopProcs) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// Less allows us to sort IOTopProcs.
func (s IOTopProcs) Less(i, j int) bool {
	return s[i].DiskRead+s[i].DiskWrite < s[j].DiskRead+s[j].DiskWrite
}

// Shrink a process list.
func (s *IOTopProcs) Shrink(size int) {
	if s == nil {
		return
	}

	if len(*s) > size {
		*s = (*s)[:size]
	}
}
