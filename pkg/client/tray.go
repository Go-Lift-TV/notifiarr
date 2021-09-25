//go:build darwin || windows
// +build darwin windows

package client

import (
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/Notifiarr/notifiarr/pkg/bindata"
	"github.com/Notifiarr/notifiarr/pkg/mnd"
	"github.com/Notifiarr/notifiarr/pkg/notifiarr"
	"github.com/Notifiarr/notifiarr/pkg/ui"
	"github.com/getlantern/systray"
	"golift.io/version"
)

/* This file handles the OS GUI elements. */

// startTray Run()s readyTray to bring up the web server and the GUI app.
func (c *Client) startTray() {
	systray.Run(func() {
		defer os.Exit(0)
		defer c.CapturePanic()

		b, _ := bindata.Asset(ui.SystrayIcon)
		systray.SetTemplateIcon(b, b)
		systray.SetTooltip(c.Flags.Name() + " v" + version.Version)
		c.makeChannels() // make these before starting the web server.
		c.makeMoreChannels()
		c.setupChannels(c.watchKillerChannels, c.watchNotifiarrMenu, c.watchLogsChannels,
			c.watchConfigChannels, c.watchGuiChannels, c.watchTimerChannels, c.watchTopChannels)

		c.setupMenus()

		// This starts the web server, and waits for reload/exit signals.
		if err := c.Exit(); err != nil {
			c.Errorf("Server: %v", err)
			os.Exit(1) // web server problem
		}
	}, func() {
		// This code only fires from menu->quit.
		if err := c.exit(); err != nil {
			c.Errorf("Server: %v", err)
			os.Exit(1) // web server problem
		}
		// because systray wants to control the exit code? no..
		os.Exit(0)
	})
}

func (c *Client) setupMenus() {
	if c.Config.LogConfig.DebugLog == "" {
		c.menu["debug_logs"].Hide()
		c.menu["debug_logs2"].Hide()
	} else {
		c.menu["debug_logs"].Show()
		c.menu["debug_logs2"].Show()
	}

	if !c.Config.Debug {
		c.menu["debug"].Hide()
		c.menu["debug_logs"].Hide()
		c.menu["debug_logs2"].Hide()
	} else {
		c.menu["debug"].Show()
		c.menu["debug_logs"].Show()
		c.menu["debug_logs2"].Show()
	}

	if c.Config.Services.LogFile == "" {
		c.menu["logs_svcs"].Hide()
	} else {
		c.menu["logs_svcs"].Show()
	}

	if !c.Config.Services.Disabled {
		c.menu["svcs"].Check()
	} else {
		c.menu["svcs"].Uncheck()
	}

	if ci, err := c.website.GetClientInfo(notifiarr.EventStart); err == nil {
		if ci.IsSub() {
			c.menu["sub"].SetTitle("Subscriber \u2764\ufe0f")
			c.menu["sub"].Check()
			c.menu["sub"].Disable()
			c.menu["sub"].SetTooltip("THANK YOU for supporting the project!")
		} else if ci.IsPatron() {
			c.menu["sub"].SetTitle("Patron \U0001f9e1")
			c.menu["sub"].SetTooltip("THANK YOU for supporting the project!")
			c.menu["sub"].Check()
		}
	}
}

// setupChannels runs the channel watcher loops in go routines with a panic catcher.
func (c *Client) setupChannels(funcs ...func()) {
	for _, f := range funcs {
		go func(f func()) {
			defer c.CapturePanic()
			f()
		}(f)
	}
}

func (c *Client) makeChannels() {
	c.menu["stat"] = ui.WrapMenu(systray.AddMenuItem("Running", "web server state unknown"))

	conf := systray.AddMenuItem("Config", "show configuration")
	c.menu["conf"] = ui.WrapMenu(conf)
	c.menu["view"] = ui.WrapMenu(conf.AddSubMenuItem("View", "show configuration"))
	c.menu["edit"] = ui.WrapMenu(conf.AddSubMenuItem("Edit", "edit configuration"))
	c.menu["write"] = ui.WrapMenu(conf.AddSubMenuItem("Write", "write config file"))
	c.menu["svcs"] = ui.WrapMenu(conf.AddSubMenuItem("Services", "toggle service checks routine"))
	c.menu["load"] = ui.WrapMenu(conf.AddSubMenuItem("Reload", "reload configuration"))

	link := systray.AddMenuItem("Links", "external resources")
	c.menu["link"] = ui.WrapMenu(link)
	c.menu["info"] = ui.WrapMenu(link.AddSubMenuItem(c.Flags.Name(), version.Print(c.Flags.Name())))
	c.menu["info"].Disable()
	c.menu["hp"] = ui.WrapMenu(link.AddSubMenuItem("Notifiarr.com", "open Notifiarr.com"))
	c.menu["wiki"] = ui.WrapMenu(link.AddSubMenuItem("Notifiarr.Wiki", "open Notifiarr wiki"))
	c.menu["trash"] = ui.WrapMenu(link.AddSubMenuItem("TRaSH Guide", "open TRaSH wiki for Notifiarr"))
	c.menu["disc1"] = ui.WrapMenu(link.AddSubMenuItem("Notifiarr Discord", "open Notifiarr discord server"))
	c.menu["disc2"] = ui.WrapMenu(link.AddSubMenuItem("Go Lift Discord", "open Go Lift discord server"))
	c.menu["gh"] = ui.WrapMenu(link.AddSubMenuItem("GitHub Project", c.Flags.Name()+" on GitHub"))

	logs := systray.AddMenuItem("Logs", "log file info")
	c.menu["logs"] = ui.WrapMenu(logs)
	c.menu["logs_view"] = ui.WrapMenu(logs.AddSubMenuItem("View", "view the application log"))
	c.menu["logs_http"] = ui.WrapMenu(logs.AddSubMenuItem("HTTP", "view the HTTP log"))
	c.menu["debug_logs2"] = ui.WrapMenu(logs.AddSubMenuItem("Debug", "view the Debug log"))
	c.menu["logs_svcs"] = ui.WrapMenu(logs.AddSubMenuItem("Services", "view the Services log"))
	c.menu["logs_rotate"] = ui.WrapMenu(logs.AddSubMenuItem("Rotate", "rotate both log files"))
}

// makeMoreChannels makes the Notifiarr menu and Debug menu items.
//nolint:lll
func (c *Client) makeMoreChannels() {
	data := systray.AddMenuItem("Notifiarr", "plex sessions, system snapshots, service checks")
	c.menu["data"] = ui.WrapMenu(data)
	c.menu["gaps"] = ui.WrapMenu(data.AddSubMenuItem("Send Radarr Gaps", "[premium feature] trigger radarr collections gaps"))
	c.menu["sync_cf"] = ui.WrapMenu(data.AddSubMenuItem("Sync Custom Formats", "[premium feature] trigger custom format sync"))
	c.menu["svcs_prod"] = ui.WrapMenu(data.AddSubMenuItem("Check and Send Services", "check all services and send results to notifiarr"))
	c.menu["plex_prod"] = ui.WrapMenu(data.AddSubMenuItem("Send Plex Sessions", "send plex sessions to notifiarr"))
	c.menu["snap_prod"] = ui.WrapMenu(data.AddSubMenuItem("Send System Snapshot", "send system snapshot to notifiarr"))
	c.menu["app_ques"] = ui.WrapMenu(data.AddSubMenuItem("Stuck Queue Items Check", "check app queues for stuck items and send to notifiarr"))
	c.menu["send_dash"] = ui.WrapMenu(data.AddSubMenuItem("Send Dashboard States", "collect and send all application states for a dashboard update"))

	if ci, err := c.website.GetClientInfo(notifiarr.EventStart); err == nil {
		ui.WrapMenu(data.AddSubMenuItem("- Custom Timers -", "")).Disable()

		for _, t := range ci.Actions.Custom {
			desc := "this is a dynamic custom timer"
			if t.Desc != "" {
				desc = t.Desc
			}

			c.menu["timer"+t.Name] = ui.WrapMenu(data.AddSubMenuItem(t.Name,
				fmt.Sprintf("%s; config: interval: %s, path: %s", desc, t.Interval, t.URI)))
		}
	}

	debug := systray.AddMenuItem("Debug", "Debug Menu")
	c.menu["debug"] = ui.WrapMenu(debug)
	c.menu["mode"] = ui.WrapMenu(debug.AddSubMenuItem("Mode: "+strings.Title(c.Config.Mode), "toggle application mode"))
	c.menu["debug_logs"] = ui.WrapMenu(debug.AddSubMenuItem("View Debug Log", "view the Debug log"))
	c.menu["svcs_log"] = ui.WrapMenu(debug.AddSubMenuItem("Log Service Checks", "check all services and log results"))

	ui.WrapMenu(debug.AddSubMenuItem("- Danger Zone -", "")).Disable()
	c.menu["debug_panic"] = ui.WrapMenu(debug.AddSubMenuItem("Application Panic", "cause an application panic (crash)"))
	c.menu["update"] = ui.WrapMenu(systray.AddMenuItem("Update", "check GitHub for updated version"))
	c.menu["sub"] = ui.WrapMenu(systray.AddMenuItem("Subscribe", "subscribe for premium features"))
	c.menu["exit"] = ui.WrapMenu(systray.AddMenuItem("Quit", "exit "+c.Flags.Name()))
}

// Listen to the top-menu-item channels so they don't back up with junk.
func (c *Client) watchTopChannels() {
	for {
		select {
		case <-c.menu["conf"].Clicked(): // unused, top menu.
		case <-c.menu["link"].Clicked(): // unused, top menu.
		case <-c.menu["logs"].Clicked(): // unused, top menu.
		case <-c.menu["data"].Clicked(): // unused, top menu.
		case <-c.menu["debug"].Clicked(): // unused, top menu.
		}
	}
}

// dynamic menu items with reflection, anyone?
func (c *Client) watchTimerChannels() {
	ci, err := c.website.GetClientInfo(notifiarr.EventStart)
	if err != nil || len(ci.Actions.Custom) == 0 {
		return
	}

	cases := make([]reflect.SelectCase, len(ci.Actions.Custom))
	for i, t := range ci.Actions.Custom {
		cases[i] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(c.menu["timer"+t.Name].Clicked())}
	}

	for {
		index, _, _ := reflect.Select(cases)
		ci.Actions.Custom[index].Run(notifiarr.EventUser)
	}
}

func (c *Client) watchKillerChannels() {
	defer systray.Quit() // this kills the app

	for {
		select {
		case <-c.menu["exit"].Clicked():
			c.Errorf("Need help? %s\n=====> Exiting! User Requested", mnd.HelpLink)
			return
		case <-c.menu["debug_panic"].Clicked():
			c.menuPanic()
		case <-c.menu["load"].Clicked():
			if err := c.reloadConfiguration("User Requested"); err != nil {
				c.Errorf("Need help? %s\n=====> Exiting! Reloading Configuration: %v", mnd.HelpLink, err)
				os.Exit(1) //nolint:gocritic // exit now since config is bad and everything is disabled.
			}
		}
	}
}

// nolint:errcheck
func (c *Client) watchGuiChannels() {
	for {
		select {
		case <-c.menu["stat"].Clicked():
			c.toggleServer()
		case <-c.menu["gh"].Clicked():
			go ui.OpenURL("https://github.com/Notifiarr/notifiarr/")
		case <-c.menu["hp"].Clicked():
			go ui.OpenURL("https://notifiarr.com/")
		case <-c.menu["wiki"].Clicked():
			go ui.OpenURL("https://notifiarr.wiki/")
		case <-c.menu["trash"].Clicked():
			go ui.OpenURL("https://trash-guides.info/Notifiarr/Quick-Start/")
		case <-c.menu["disc1"].Clicked():
			go ui.OpenURL("https://notifiarr.com/discord")
		case <-c.menu["disc2"].Clicked():
			go ui.OpenURL("https://golift.io/discord")
		case <-c.menu["sub"].Clicked():
			go ui.OpenURL("https://www.buymeacoffee.com/nitsua")
		}
	}
}

// nolint:errcheck
func (c *Client) watchConfigChannels() {
	for {
		select {
		case <-c.menu["view"].Clicked():
			go ui.Info(mnd.Title+": Configuration", c.displayConfig())
		case <-c.menu["edit"].Clicked():
			go ui.OpenFile(c.Flags.ConfigFile)
			c.Print("user requested] Editing Config File:", c.Flags.ConfigFile)
		case <-c.menu["write"].Clicked():
			go c.writeConfigFile()
		case <-c.menu["mode"].Clicked():
			if c.Config.Mode == notifiarr.ModeDev {
				c.Config.Mode = c.website.Setup(notifiarr.ModeProd)
			} else {
				c.Config.Mode = c.website.Setup(notifiarr.ModeDev)
			}

			c.menu["mode"].SetTitle("Mode: " + strings.Title(c.Config.Mode))
			c.Printf("[user requested] Application mode changed to %s!", strings.Title(c.Config.Mode))
			ui.Notify("Application mode changed to %s!", strings.Title(c.Config.Mode))
		case <-c.menu["svcs"].Clicked():
			if c.menu["svcs"].Checked() {
				c.menu["svcs"].Uncheck()
				c.Config.Services.Stop()
				ui.Notify("Stopped checking services!")
			} else {
				c.menu["svcs"].Check()
				c.Config.Services.Start()
				ui.Notify("Service checks started!")
			}
		}
	}
}

// nolint:errcheck
func (c *Client) watchLogsChannels() {
	for {
		select {
		case <-c.menu["logs_view"].Clicked():
			go ui.OpenLog(c.Config.LogFile)
			c.Print("[user requested] Viewing App Log File:", c.Config.LogFile)
		case <-c.menu["logs_http"].Clicked():
			go ui.OpenLog(c.Config.HTTPLog)
			c.Print("[user requested] Viewing HTTP Log File:", c.Config.HTTPLog)
		case <-c.menu["logs_svcs"].Clicked():
			go ui.OpenLog(c.Config.Services.LogFile)
			c.Print("[user requested] Viewing Services Log File:", c.Config.Services.LogFile)
		case <-c.menu["debug_logs"].Clicked():
			go ui.OpenLog(c.Config.LogConfig.DebugLog)
			c.Print("[user requested] Viewing Debug File:", c.Config.LogConfig.DebugLog)
		case <-c.menu["debug_logs2"].Clicked():
			go ui.OpenLog(c.Config.LogConfig.DebugLog)
			c.Print("[user requested] Viewing Debug File:", c.Config.LogConfig.DebugLog)
		case <-c.menu["logs_rotate"].Clicked():
			c.rotateLogs()
		case <-c.menu["update"].Clicked():
			go c.checkForUpdate()
		}
	}
}

//nolint:errcheck
func (c *Client) watchNotifiarrMenu() {
	for {
		select {
		case <-c.menu["gaps"].Clicked():
			c.website.Trigger.SendGaps(notifiarr.EventUser)
		case <-c.menu["sync_cf"].Clicked():
			c.website.Trigger.SyncCF(notifiarr.EventUser)
		case <-c.menu["svcs_log"].Clicked():
			c.Print("[user requested] Checking services and logging results.")
			ui.Notify("Running and logging %d Service Checks.", len(c.Config.Service))
			c.Config.Services.RunChecks("log")
		case <-c.menu["svcs_prod"].Clicked():
			c.Print("[user requested] Checking services and sending results to Notifiarr.")
			ui.Notify("Running and sending %d Service Checks.", len(c.Config.Service))
			c.Config.Services.RunChecks(notifiarr.EventUser)
		case <-c.menu["app_ques"].Clicked():
			c.website.Trigger.SendStuckQueueItems(notifiarr.EventUser)
		case <-c.menu["plex_prod"].Clicked():
			c.website.Trigger.SendPlexSessions(notifiarr.EventUser)
		case <-c.menu["snap_prod"].Clicked():
			c.website.Trigger.SendSnapshot(notifiarr.EventUser)
		case <-c.menu["send_dash"].Clicked():
			c.website.Trigger.SendDashboardState(notifiarr.EventUser)
		}
	}
}
