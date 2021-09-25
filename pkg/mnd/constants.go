// Package mnd provides re-usable constants for the Notifiarr application packages.
package mnd

import "time"

// Application Constants.
const (
	Mode0755 = 0o755
	Mode0750 = 0o750
	Mode0600 = 0o600
	Megabyte = 1024 * 1024
	KB100    = 1024 * 100
	OneDay   = 24 * time.Hour
	HalfHour = 30 * time.Minute
	Base10   = 10
	Base8    = 8
	Bits64   = 64
	Bits32   = 32
	Windows  = "windows"
	HelpLink = "GoLift Discord: https://golift.io/discord"
	UserRepo = "Notifiarr/notifiarr"
	BugIssue = "This is a bug please report it on github: https://github.com/" + UserRepo + "/issues/new"
)

// Application Defaults.
const (
	Title            = "Notifiarr"
	DefaultName      = "notifiarr"
	DefaultLogFileMb = 100
	DefaultLogFiles  = 0 // delete none
	DefaultEnvPrefix = "DN"
	DefaultTimeout   = time.Minute
	DefaultBindAddr  = "0.0.0.0:5454"
)
