//nolint:mnd
package snapshot

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/Notifiarr/notifiarr/pkg/mnd"
)

// IPMISensor contains the data for one sensor.
type IPMISensor struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
	State string  `json:"state"`
}

// GetIPMI grans basic sensor info.
func (s *Snapshot) GetIPMI(ctx context.Context, run, useSudo bool) error {
	if !run {
		return nil
	}

	tool := "/ipmitool"
	args := []string{"sdr", "elist", "full"}
	scanFn := s.scanIPMIToolOutput

	_, err := os.Stat(tool)
	if err != nil {
		tool, err = exec.LookPath("ipmitool")
	}

	if err != nil {
		args = []string{
			"--sensor-types",
			"temperature,voltage,fan,current,Other_Units_Based_Sensor",
			"--sdr-cache-recreate",
			"--quiet-cache",
		}
		scanFn = s.scanIPMISensorsOutput

		if tool, err = exec.LookPath("ipmi-sensors"); err != nil {
			return fmt.Errorf("unable to find 'ipmitool' or 'ipmi-sensors': %w", os.ErrNotExist)
		}
	}

	if useSudo {
		args = append([]string{"-n", tool}, args...)

		if tool, err = exec.LookPath("sudo"); err != nil {
			return fmt.Errorf("sudo missing! %w", err)
		}
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, tool, args...)
	SysCallSettings(cmd)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	scanner := bufio.NewScanner(&stdout)
	scanner.Split(bufio.ScanLines)

	defer scanFn(scanner)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%v: %w: %s", cmd.Args, err, stderr.String())
	}

	return nil
}

// 14  | Fan1 RPM         | Fan                      | 3720.00    | RPM   | 'OK'
// 15  | Fan2 RPM         | Fan                      | 3600.00    | RPM   | 'OK'
// 16  | Fan3 RPM         | Fan                      | 3600.00    | RPM   | 'OK'
// 17  | Fan4 RPM         | Fan                      | 3480.00    | RPM   | 'OK'
// 18  | Fan5 RPM         | Fan                      | 3600.00    | RPM   | 'OK'
// 19  | Fan6 RPM         | Fan                      | 3600.00    | RPM   | 'OK'
// 20  | Inlet Temp       | Temperature              | 27.00      | C     | 'OK'
// 21  | CPU Usage        | Other Units Based Sensor | 1.00       | %     | 'OK'
// 22  | IO Usage         | Other Units Based Sensor | 0.00       | %     | 'OK'
// 23  | MEM Usage        | Other Units Based Sensor | 0.00       | %     | 'OK'
// 24  | SYS Usage        | Other Units Based Sensor | 2.00       | %     | 'OK'
// 25  | Exhaust Temp     | Temperature              | 41.00      | C     | 'OK'
// 26  | Temp             | Temperature              | 51.00      | C     | 'OK'
// 27  | Temp             | Temperature              | 52.00      | C     | 'OK'
// 91  | Current 1        | Current                  | 1.00       | A     | 'OK'
// 92  | Current 2        | Current                  | 1.00       | A     | 'OK'
// 93  | Voltage 1        | Voltage                  | 116.00     | V     | 'OK'
// 94  | Voltage 2        | Voltage                  | 116.00     | V     | 'OK'
// 98  | Pwr Consumption  | Current                  | 238.00     | W     | 'OK'
func (s *Snapshot) scanIPMISensorsOutput(scanner *bufio.Scanner) {
	for scanner.Scan() {
		item := strings.Split(scanner.Text(), "|")
		if len(item) != 6 {
			continue // line has wrong item count.
		}

		val, err := strconv.ParseFloat(strings.TrimSpace(item[3]), mnd.Bits64)
		if err != nil {
			continue
		}

		s.Sensors = append(s.Sensors, &IPMISensor{
			Name:  strings.TrimSpace(item[1]),
			Value: val,
			Unit:  strings.TrimSpace(item[4]),
			State: strings.ToLower(strings.Trim(strings.TrimSpace(item[5]), "'")),
		})
	}
}

// Fan1 RPM         | 30h | ok  |  7.1 | 3720 RPM
// Fan2 RPM         | 31h | ok  |  7.1 | 3600 RPM
// Fan3 RPM         | 32h | ok  |  7.1 | 3600 RPM
// Fan4 RPM         | 33h | ok  |  7.1 | 3480 RPM
// Fan5 RPM         | 34h | ok  |  7.1 | 3600 RPM
// Fan6 RPM         | 35h | ok  |  7.1 | 3600 RPM
// Inlet Temp       | 04h | ok  |  7.1 | 27 degrees C
// CPU Usage        | FDh | ok  |  7.1 | 1 percent
// IO Usage         | F1h | ok  |  7.1 | 0 percent
// MEM Usage        | F2h | ok  |  7.1 | 0 percent
// SYS Usage        | F3h | ok  |  7.1 | 2 percent
// Exhaust Temp     | 01h | ok  |  7.1 | 41 degrees C
// Temp             | 0Eh | ok  |  3.1 | 51 degrees C
// Temp             | 0Fh | ok  |  3.2 | 52 degrees C
// Current 1        | 6Ah | ok  | 10.1 | 1 Amps
// Current 2        | 6Bh | ok  | 10.2 | 1 Amps
// Voltage 1        | 6Ch | ok  | 10.1 | 118 Volts
// Voltage 2        | 6Dh | ok  | 10.2 | 118 Volts
// Pwr Consumption  | 77h | ok  |  7.1 | 238 Watts
func (s *Snapshot) scanIPMIToolOutput(scanner *bufio.Scanner) {
	for scanner.Scan() {
		item := strings.Split(scanner.Text(), "|")
		if len(item) != 5 {
			continue // line has wrong item count.
		}

		valSplit := strings.SplitN(strings.TrimSpace(item[4]), " ", 2)

		val, err := strconv.ParseFloat(valSplit[0], mnd.Bits64)
		if len(valSplit) != 2 || err != nil {
			continue
		}

		s.Sensors = append(s.Sensors, &IPMISensor{
			Name:  strings.TrimSpace(item[0]),
			Value: val,
			Unit:  valSplit[1],
			State: strings.ToLower(strings.TrimSpace(item[2])),
		})
	}
}
