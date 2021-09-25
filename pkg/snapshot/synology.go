package snapshot

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/shirou/gopsutil/v3/host"
)

// SynologyConf is the path to the syno config file.
const SynologyConf = "/etc/synoinfo.conf"

// Synology is the data we care about from the config file.
type Synology struct {
	Build   string `json:"last_admin_login_build"` // 254263
	Manager string `json:"manager"`                // Synology DiskStation
	Vendor  string `json:"vender"`                 // Synology Inc.
	Model   string `json:"upnpmodelname"`          // DS1517+
	Version string `json:"udc_check_state"`        // 6.2.3
}

/*
 "platform": "Synology Inc.",
 "platformFamily": "Synology DiskStation DS1517+",
 "platformVersion": "6.2.3-254263",
*/

// GetSynology checks if the app is running on a Synology, and gets system info.
func GetSynology(run bool) (*Synology, error) { //nolint:cyclop
	if !run {
		return nil, nil
	}

	file, err := os.Open(SynologyConf)
	if err != nil {
		return nil, fmt.Errorf("opening synology conf: %w", err)
	}
	defer file.Close()

	// Start reading from the file with a reader.
	var (
		reader = bufio.NewReader(file)
		syn    = &Synology{}
	)

	for {
		line, err := reader.ReadString('\n')
		if errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, fmt.Errorf("reading synology conf: %w", err)
		}

		lsplit := strings.Split(line, "=")
		if len(lsplit) < 2 { //nolint:gomnd
			continue
		}

		switch lsplit[0] {
		case "last_admin_login_build":
			syn.Build = strings.Trim(lsplit[1], "\n\"")
		case "manager":
			syn.Manager = strings.Trim(lsplit[1], "\n\"")
		case "vender":
			syn.Vendor = strings.Trim(lsplit[1], "\n\"")
		case "upnpmodelname":
			syn.Model = strings.Trim(lsplit[1], "\n\"")
		case "udc_check_state":
			syn.Version = strings.Trim(lsplit[1], "\n\"")
		}
	}

	return syn, nil
}

func (s *Synology) SetInfo(hi *host.InfoStat) {
	if hi.Platform == "" && s.Vendor != "" {
		hi.Platform = s.Vendor
	}

	if hi.PlatformFamily == "" && s.Manager != "" {
		hi.PlatformFamily = s.Manager + " " + s.Model
	}

	if hi.PlatformVersion == "" && s.Version != "" {
		hi.PlatformVersion = s.Version + "-" + s.Build
	}
}
