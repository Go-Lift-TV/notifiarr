package ui

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"

	"github.com/Notifiarr/notifiarr/pkg/mnd"
	"github.com/gen2brain/beeep"
)

// SystrayIcon is the icon in the menu bar.
const SystrayIcon = "files/macos.png"

var hasGUI = os.Getenv("USEGUI") == "true"

// HasGUI returns false on Linux, true on Windows and optional on macOS.
func HasGUI() bool {
	return hasGUI
}

// HideConsoleWindow doesn't work on maacOS.
func HideConsoleWindow() {}

// ShowConsoleWindow does nothing on OSes besides Windows.
func ShowConsoleWindow() {}

func Notify(msg string) error {
	if !hasGUI {
		return nil
	}

	err := beeep.Notify(mnd.Title, msg, "")
	if err != nil {
		return fmt.Errorf("ui element failed: %w", err)
	}

	return nil
}

// StartCmd starts a command.
func StartCmd(c string, v ...string) error {
	cmd := exec.Command(c, v...)
	cmd.Stdout = ioutil.Discard
	cmd.Stderr = ioutil.Discard

	return cmd.Run()
}

// OpenCmd opens anything.
func OpenCmd(cmd ...string) error {
	return StartCmd("open", cmd...)
}

// OpenURL opens URL Links.
func OpenURL(url string) error {
	return OpenCmd(url)
}

// OpenLog opens Log Files.
func OpenLog(logFile string) error {
	return OpenCmd("-b", "com.apple.Console", logFile)
}

// OpenFile open Config Files.
func OpenFile(filePath string) error {
	return OpenCmd("-t", filePath)
}
