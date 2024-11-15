package snapshot

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"strconv"
	"strings"

	"github.com/Notifiarr/notifiarr/pkg/mnd"
)

// NvidiaConfig is our input data.
type NvidiaConfig struct {
	SMIPath  string   `json:"smiPath"  toml:"smi_path" xml:"smi_path"`
	BusIDs   []string `json:"busIDs"   toml:"bus_ids"  xml:"bus_id"`
	Disabled bool     `json:"disabled" toml:"disabled" xml:"disabled"`
}

// NvidiaOutput is what we send to the website.
type NvidiaOutput struct {
	Name        string `json:"name"`
	Driver      string `json:"driverVersion"`
	Pstate      string `json:"pState"`
	Vbios       string `json:"vBios"`
	BusID       string `json:"busId"`
	Temperature int    `json:"temperature"`
	Utilization int    `json:"utiliization"`
	MemTotal    int    `json:"memTotal"`
	MemFree     int    `json:"memFree"`
}

// HasID returns true if the ID is requested, or no IDs are filtered.
func (n *NvidiaConfig) HasID(busID string) bool {
	for _, id := range n.BusIDs {
		if id == busID {
			return true
		}
	}

	return len(n.BusIDs) == 0
}

// GetNvidia requires nvidia-smi executable and Nvidia drivers.
func (s *Snapshot) GetNvidia(ctx context.Context, config *NvidiaConfig) error {
	if config == nil || config.Disabled {
		return nil
	}

	var err error

	cmdPath := config.SMIPath
	if cmdPath != "" {
		if _, err = os.Stat(cmdPath); err != nil {
			return fmt.Errorf("unable to locate nvidia-smi at provided path '%s': %w", cmdPath, err)
		}
	} else if cmdPath, err = exec.LookPath(nvidiaSMIname()); err != nil {
		return fmt.Errorf("nvidia-smi missing from PATH! %w", err)
	}

	cmd := exec.CommandContext(ctx, cmdPath, "--format=csv,noheader", "--query-gpu="+
		"name,"+ // 0
		"driver_version,"+ // 1
		"pstate,"+ // 2
		"vbios_version,"+ // 3
		"pci.bus_id,"+ // 4
		"temperature.gpu,"+ // 5
		"utilization.gpu,"+ // 6
		"memory.total,"+ // 7
		"memory.free", // 8
	)
	SysCallSettings(cmd)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	scanner := bufio.NewScanner(&stdout)
	scanner.Split(bufio.ScanLines)

	defer s.scanNvidiaSMIOutput(scanner, config)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%v: %w: %s", cmd.Args, err, stderr.String())
	}

	return nil
}

func (s *Snapshot) scanNvidiaSMIOutput(scanner *bufio.Scanner, config *NvidiaConfig) {
	// Output (1 card per line):
	// NVIDIA GeForce GTX 1060 3GB, 465.19.01, P0, 86.06.3C.40.17, 00000000:04:00.0, 63, 2 %, 3017 MiB, 3017 MiB
	// GeForce GTX 1660 Ti, 456.71, P2, 90.16.20.00.89, 00000000:01:00.0, 50, 0 %, 6144 MiB, 4292 MiB
	for scanner.Scan() {
		item := strings.Split(scanner.Text(), ", ")
		if len(item) != reflect.TypeOf(NvidiaOutput{}).NumField() || !config.HasID(item[4]) {
			continue // line has wrong item count, or ID not in list of allowed Bus IDs.
		}

		output := NvidiaOutput{
			Name:   item[0],
			Driver: item[1],
			Pstate: item[2],
			Vbios:  item[3],
			BusID:  item[4],
		}
		output.Temperature, _ = strconv.Atoi(item[5])
		output.Utilization, _ = strconv.Atoi(strings.Fields(item[6])[0])
		output.MemTotal, _ = strconv.Atoi(strings.Fields(item[7])[0])
		output.MemFree, _ = strconv.Atoi(strings.Fields(item[8])[0])

		s.Nvidia = append(s.Nvidia, &output)
	}
}

func nvidiaSMIname() string {
	if runtime.GOOS == mnd.Windows {
		return "nvidia-smi.exe"
	}

	return "nvidia-smi"
}
