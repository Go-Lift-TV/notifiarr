module github.com/Notifiarr/notifiarr

go 1.22.0

toolchain go1.23.5

// pflag and tail are pinned to master. 12/31/2022

require (
	github.com/BurntSushi/toml v1.4.0
	github.com/CAFxX/httpcompression v0.0.9
	github.com/akavel/rsrc v0.10.2
	github.com/dsnet/compress v0.0.1
	github.com/energye/systray v1.0.2
	github.com/fsnotify/fsnotify v1.7.0
	github.com/gen2brain/beeep v0.0.0-20240516210008-9c006672e7f4
	github.com/gen2brain/dlgs v0.0.0-20220603100644-40c77870fa8d
	github.com/go-ping/ping v1.1.0
	github.com/go-sql-driver/mysql v1.8.1
	github.com/gorilla/mux v1.8.1
	github.com/gorilla/schema v1.4.1
	github.com/gorilla/securecookie v1.1.2
	github.com/gorilla/websocket v1.5.3
	github.com/hako/durafmt v0.0.0-20210608085754-5c1018a4e16b
	github.com/hekmon/transmissionrpc/v3 v3.0.0
	github.com/hugelgupf/go-shlex v0.0.0-20200702092117-c80c9d0918fa
	github.com/jackpal/gateway v1.0.15
	github.com/jaypipes/ghw v0.13.0
	github.com/jxeng/shortcut v1.0.2
	github.com/lestrrat-go/apache-logformat/v2 v2.0.6
	github.com/mitchellh/go-homedir v1.1.0
	github.com/mrobinsn/go-rtorrent v1.8.0
	github.com/nxadm/tail v1.4.11
	github.com/shirou/gopsutil/v4 v4.24.9
	github.com/spf13/pflag v1.0.6-0.20210604193023-d5e0c0615ace
	github.com/stretchr/testify v1.9.0
	github.com/swaggo/swag v1.16.3
	github.com/vearutop/statigz v1.4.3
	golang.org/x/crypto v0.27.0
	golang.org/x/mod v0.21.0
	golang.org/x/sys v0.25.0
	golang.org/x/text v0.18.0
	golang.org/x/time v0.6.0
	golift.io/cache v0.0.2
	golift.io/cnfg v0.2.3
	golift.io/cnfgfile v0.0.0-20240713024420-a5436d84eb48
	golift.io/datacounter v1.0.4
	golift.io/deluge v0.10.1
	golift.io/mulery v0.0.8
	golift.io/nzbget v0.1.5
	golift.io/qbit v0.0.0-20240715191156-11930ac2546e
	golift.io/rotatorr v0.0.0-20240723172740-cb73b9c4894c
	golift.io/starr v1.0.1-0.20240918221538-33c5229c6ddb
	golift.io/version v0.0.2
	golift.io/xtractr v0.2.2
	modernc.org/sqlite v1.33.1
)

require (
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/KyleBanks/depth v1.2.1 // indirect
	github.com/StackExchange/wmi v1.2.1 // indirect
	github.com/andybalholm/brotli v1.1.0 // indirect
	github.com/bodgit/plumbing v1.3.0 // indirect
	github.com/bodgit/sevenzip v1.5.2 // indirect
	github.com/bodgit/windows v1.0.1 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/ebitengine/purego v0.8.0 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/go-openapi/jsonpointer v0.21.0 // indirect
	github.com/go-openapi/jsonreference v0.21.0 // indirect
	github.com/go-openapi/spec v0.21.0 // indirect
	github.com/go-openapi/swag v0.23.0 // indirect
	github.com/go-toast/toast v0.0.0-20190211030409-01e6764cf0a4 // indirect
	github.com/godbus/dbus/v5 v5.1.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gopherjs/gopherjs v1.17.2 // indirect
	github.com/gorilla/rpc v1.2.1 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/hekmon/cunits/v2 v2.1.0 // indirect
	github.com/jaypipes/pcidb v1.0.1 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/kdomanski/iso9660 v0.4.0 // indirect
	github.com/klauspost/compress v1.17.10 // indirect
	github.com/lestrrat-go/strftime v1.1.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20240909124753-873cd0166683 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/ncruces/go-strftime v0.1.9 // indirect
	github.com/nu7hatch/gouuid v0.0.0-20131221200532-179d4d0c4d8d // indirect
	github.com/nwaples/rardecode v1.1.3 // indirect
	github.com/pierrec/lz4/v4 v4.1.21 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/power-devops/perfstat v0.0.0-20240221224432-82ca36839d55 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/tadvi/systray v0.0.0-20190226123456-11a2b8fa57af // indirect
	github.com/tevino/abool v1.2.0 // indirect
	github.com/tklauser/go-sysconf v0.3.14 // indirect
	github.com/tklauser/numcpus v0.8.0 // indirect
	github.com/ulikunitz/xz v0.5.12 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go4.org v0.0.0-20230225012048-214862532bf5 // indirect
	golang.org/x/exp v0.0.0-20240909161429-701f63a606c0 // indirect
	golang.org/x/net v0.29.0 // indirect
	golang.org/x/sync v0.8.0 // indirect
	golang.org/x/tools v0.25.0 // indirect
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	howett.net/plist v1.0.1 // indirect
	modernc.org/gc/v3 v3.0.0-20240801135723-a856999a2e4a // indirect
	modernc.org/libc v1.61.0 // indirect
	modernc.org/mathutil v1.6.0 // indirect
	modernc.org/memory v1.8.0 // indirect
	modernc.org/strutil v1.2.0 // indirect
	modernc.org/token v1.1.0 // indirect
)
