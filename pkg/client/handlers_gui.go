package client

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/Notifiarr/notifiarr/pkg/bindata"
	"github.com/Notifiarr/notifiarr/pkg/bindata/docs"
	"github.com/Notifiarr/notifiarr/pkg/checkapp"
	"github.com/Notifiarr/notifiarr/pkg/configfile"
	"github.com/Notifiarr/notifiarr/pkg/logs"
	"github.com/Notifiarr/notifiarr/pkg/mnd"
	"github.com/Notifiarr/notifiarr/pkg/services"
	"github.com/Notifiarr/notifiarr/pkg/snapshot"
	"github.com/Notifiarr/notifiarr/pkg/triggers/common"
	"github.com/Notifiarr/notifiarr/pkg/update"
	"github.com/Notifiarr/notifiarr/pkg/website"
	"github.com/gorilla/mux"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/swaggo/swag"
	"github.com/vearutop/statigz"
	"golift.io/cnfgfile"
	"golift.io/version"
)

// @title Notifiarr Client GUI API Documentation
// @description Monitors local services and sends notifications.
// @termsOfService https://notifiarr.com
// @contact.name Notifiarr Discord
// @contact.url https://notifiarr.com/discord
// @license.name MIT
// @license.url https://github.com/Notifiarr/notifiarr/blob/main/LICENSE
// @BasePath /

const (
	minPasswordLen = 9
	fileSourceLogs = "logs"
)

// userNameValue is used a context value key.
type userNameValue int

//nolint:gochecknoglobals // used as context value key.
var userNameStr = userNameValue(1)

func (c *Client) checkAuthorized(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		userName, dynamic := c.getUserName(request)
		if userName != "" {
			ctx := context.WithValue(request.Context(), userNameStr, []any{userName, dynamic})
			next.ServeHTTP(response, request.WithContext(ctx))
		} else {
			http.Redirect(response, request, c.Config.URLBase, http.StatusFound)
		}
	})
}

// getUserName returns the username and a bool if it's dynamic (not the one from the config file).
func (c *Client) getUserName(request *http.Request) (string, bool) {
	if userName := request.Context().Value(userNameStr); userName != nil {
		u, _ := userName.([]any)
		username, _ := u[0].(string)
		found, _ := u[1].(bool)

		return username, found
	}

	if c.Config.Allow.Contains(request.RemoteAddr) && c.webauth {
		// If the upstream is allowed and gave us a username header, use it.
		if userName := request.Header.Get(c.authHeader); userName != "" {
			return userName, true
		}

		// If the upstream IP is allowed and no auth is enabled, set a username.
		if c.noauth { // c.webauth is always true if c.noauth is true.
			return configfile.DefaultUsername, true
		}
	}

	cookie, err := request.Cookie("session")
	if err != nil {
		return "", false
	}

	cookieValue := make(map[string]string)
	if err = c.cookies.Decode("session", cookie.Value, &cookieValue); err != nil {
		return "", false
	}

	return cookieValue["username"], false
}

func (c *Client) setSession(userName string, response http.ResponseWriter) {
	value := map[string]string{
		"username": userName,
	}

	encoded, err := c.cookies.Encode("session", value)
	if err != nil {
		return
	}

	http.SetCookie(response, &http.Cookie{
		Name:  "session",
		Value: encoded,
		Path:  "/",
	})
}

func (c *Client) loginHandler(response http.ResponseWriter, request *http.Request) {
	loggedinUsername, _ := c.getUserName(request)
	providedUsername := request.FormValue("name")

	switch {
	case loggedinUsername != "": // already logged in.
		http.Redirect(response, request, c.Config.URLBase, http.StatusFound)
	case request.Method == http.MethodGet: // dont handle login without POST
		c.indexPage(request.Context(), response, request, "")
	case c.webauth:
		c.indexPage(request.Context(), response, request, "Logins Disabled")
	case len(request.FormValue("password")) < minPasswordLen:
		c.indexPage(request.Context(), response, request, "Invalid Password Length")
	case c.checkUserPass(providedUsername, request.FormValue("password")):
		c.setSession(providedUsername, response)
		mnd.HTTPRequests.Add("GUI Logins", 1)
		http.Redirect(response, request, c.Config.URLBase, http.StatusFound)
	default: // Start over.
		c.indexPage(request.Context(), response, request, "Invalid Password")
	}
}

func (c *Client) checkUserPass(username, password string) bool {
	c.Lock()
	defer c.Unlock()

	return c.Config.UIPassword.Valid(username + ":" + password)
}

func (c *Client) logoutHandler(response http.ResponseWriter, request *http.Request) {
	http.SetCookie(response, &http.Cookie{
		Name:   "session",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
	http.Redirect(response, request, c.Config.URLBase, http.StatusFound)
}

// getFileDeleteHandler deletes log and config files.
func (c *Client) getFileDeleteHandler(response http.ResponseWriter, req *http.Request) {
	if mux.Vars(req)["source"] != fileSourceLogs {
		http.Error(response, "invalid source", http.StatusBadRequest)
		return
	}

	fileInfos := c.Logger.GetAllLogFilePaths()
	id := mux.Vars(req)["id"]

	for _, fileInfo := range fileInfos.List {
		if fileInfo.ID != id {
			continue
		}

		user, _ := c.getUserName(req)

		if err := os.Remove(fileInfo.Path); err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
			c.Errorf("[gui '%s' requested] Deleting file: %v", user, err)
		}

		c.Printf("[gui '%s' requested] Deleted file: %s", user, fileInfo.Path)

		if _, err := response.Write([]byte("ok")); err != nil {
			c.Errorf("Writing HTTP Response: %v", err)
		}

		return
	}
}

// uploadFileHandler uploads a log file to notifiarr.com.
func (c *Client) uploadFileHandler(response http.ResponseWriter, req *http.Request) {
	if mux.Vars(req)["source"] != fileSourceLogs {
		http.Error(response, "invalid source", http.StatusBadRequest)
		return
	}

	id := mux.Vars(req)["id"]
	for _, fileInfo := range c.Logger.GetAllLogFilePaths().List {
		if fileInfo.ID != id {
			continue
		}

		err := c.triggers.FileUpload.Upload(website.EventGUI, fileInfo.Path)
		if err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
		}

		user, _ := c.getUserName(req)
		c.Printf("[gui '%s' requested] Uploaded file: %s", user, fileInfo.Path)

		return
	}
}

// getFileDownloadHandler downloads log files to the browser.
func (c *Client) getFileDownloadHandler(response http.ResponseWriter, req *http.Request) {
	id := mux.Vars(req)["id"]
	for _, fileInfo := range c.Logger.GetAllLogFilePaths().List {
		if fileInfo.ID != id {
			continue
		}

		zipWriter := zip.NewWriter(response)
		defer zipWriter.Close()

		fileOpen, err := os.Open(fileInfo.Path)
		if err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
		}
		defer fileOpen.Close()

		newZippedFile, err := zipWriter.Create(fileInfo.Name)
		if err != nil {
			http.Error(response, err.Error(), http.StatusInternalServerError)
		}

		response.Header().Set("Content-Disposition", "attachment; filename="+fileInfo.Name+".zip")
		response.Header().Set("Content-Type", "application/zip")

		if _, err := io.Copy(newZippedFile, fileOpen); err != nil {
			c.Errorf("Sending Zipped File %s: %v", fileInfo.Path, err)
			http.Error(response, err.Error(), http.StatusInternalServerError)
		}

		user, _ := c.getUserName(req)
		c.Printf("[gui '%s' requested] Downloaded file: %s", user, fileInfo.Path)

		return
	}
}

func (c *Client) handleShutdown(response http.ResponseWriter, _ *http.Request) {
	defer func() {
		c.sigkil <- &update.Signal{Text: "shutdown gui triggered"}
	}()

	http.Error(response, "OK", http.StatusOK)
}

func (c *Client) handleReload(response http.ResponseWriter, _ *http.Request) {
	c.reloadAppNow()
	http.Error(response, "OK", http.StatusOK)
}

func (c *Client) reloadAppNow() {
	c.Lock()
	c.reloading = true
	c.Unlock()

	defer c.triggerConfigReload(website.EventGUI, "GUI Requested")
}

func (c *Client) handlePing(response http.ResponseWriter, _ *http.Request) {
	c.RLock()
	defer c.RUnlock()

	if c.reloading {
		http.Error(response, "Reloading", http.StatusLocked)
	} else {
		http.Error(response, "OK", http.StatusOK)
	}
}

func (c *Client) handleServicesStopStart(response http.ResponseWriter, req *http.Request) {
	user, _ := c.getUserName(req)

	switch action := mux.Vars(req)["action"]; action {
	case "stop":
		c.Config.Services.Stop()
		c.Printf("[gui '%s' requested] Service Checks Stopped", user)
		http.Error(response, "Service Checks Stopped", http.StatusOK)
	case "start":
		c.Config.Services.Start(req.Context())
		c.Printf("[gui '%s' requested] Service Checks Started", user)
		http.Error(response, "Service Checks Started", http.StatusOK)
	default:
		http.Error(response, "invalid action: "+action, http.StatusBadRequest)
	}
}

func (c *Client) handleServicesCheck(response http.ResponseWriter, req *http.Request) {
	svc := mux.Vars(req)["service"]
	if err := c.Config.Services.RunCheck(website.EventAPI, svc); err != nil {
		http.Error(response, err.Error(), http.StatusBadRequest)
		return
	}

	user, _ := c.getUserName(req)
	c.Printf("[gui '%s' requested] Check Service: %s", user, svc)
	http.Error(response, "Service Check Initiated", http.StatusOK)
}

// getFileHandler returns portions of a config or log file based on request parameters.
func (c *Client) getFileHandler(response http.ResponseWriter, req *http.Request) {
	var fileInfos *logs.LogFileInfos

	switch mux.Vars(req)["source"] {
	case fileSourceLogs:
		fileInfos = c.Logger.GetAllLogFilePaths()
	default:
		http.Error(response, "invalid source", http.StatusBadRequest)
		return
	}

	skip, _ := strconv.Atoi(mux.Vars(req)["skip"])

	count, _ := strconv.Atoi(mux.Vars(req)["lines"])
	if count == 0 {
		count = 500
		skip = 0
	}

	for _, fileInfo := range fileInfos.List {
		if fileInfo.ID != mux.Vars(req)["id"] {
			continue
		}

		lines, err := getLinesFromFile(fileInfo.Path, mux.Vars(req)["sort"], count, skip)
		if err != nil {
			c.Errorf("Handling Log File Request: %v", err)
			http.Error(response, err.Error(), http.StatusInternalServerError)
		} else if fileInfo.Size == 0 {
			http.Error(response, "the file is empty", http.StatusInternalServerError)
		} else if _, err = response.Write(lines); err != nil {
			c.Errorf("Writing HTTP Response: %v", err)
		}

		return
	}
}

func (c *Client) handleProfilePost(response http.ResponseWriter, request *http.Request) {
	var (
		currPass          = request.PostFormValue("Password")
		authType          = request.PostFormValue("AuthType")
		authHeader        = request.PostFormValue("AuthHeader")
		currUser, dynamic = c.getUserName(request)
	)

	if !dynamic {
		// If the auth is currently using a password, check the password.
		if !c.checkUserPass(currUser, currPass) {
			http.Error(response, "Invalid existing (current) password provided.", http.StatusBadRequest)
			return
		}
	}

	// Upstreams is only read on reload, but this is still not thread safe
	// because two people could click save at the same time.
	c.Lock()
	c.Config.Upstreams = strings.Fields(request.PostFormValue("Upstreams"))
	c.Unlock()

	if authType == "password" {
		c.handleProfilePostPassword(response, request)
		return
	}

	switch err := c.setUserPass(request.Context(), authType, authHeader, ""); {
	case err != nil:
		c.Errorf("[gui '%s' requested] Saving Config: %v", currUser, err)
		http.Error(response, "Saving Config: "+err.Error(), http.StatusInternalServerError)
	case authType == "nopass":
		c.Printf("[gui '%s' requested] Disabled WebUI authentication.", currUser)
		http.Error(response, "Disabled WebUI authentication.", http.StatusOK)
		c.reloadAppNow()
	default:
		c.Printf("[gui '%s' requested] Enabled WebUI proxy authentication, header: %s", currUser, authHeader)
		c.setSession(request.Header.Get(authHeader), response)
		http.Error(response, "Enabled WebUI proxy authentication. Header: "+authHeader, http.StatusOK)
		c.reloadAppNow()
	}
}

func (c *Client) handleProfilePostPassword(response http.ResponseWriter, request *http.Request) {
	currPass := request.PostFormValue("Password")
	currUser, _ := c.getUserName(request)

	username := request.PostFormValue("NewUsername")
	if username == "" {
		username = currUser
	}

	newPassw := request.PostFormValue("NewPassword")
	if newPassw == "" {
		newPassw = currPass
	}

	if len(newPassw) < minPasswordLen {
		http.Error(response, fmt.Sprintf("New password must be at least %d characters.",
			minPasswordLen), http.StatusBadRequest)
		return
	}

	if err := c.setUserPass(request.Context(), "password", username, newPassw); err != nil {
		c.Errorf("[gui '%s' requested] Saving Trust Profile: %v", currUser, err)
		http.Error(response, "Saving Trust Profile: "+err.Error(), http.StatusInternalServerError)

		return
	}

	c.Printf("[gui '%s' requested] Updated Trust Profile settings, username: %s", currUser, username)
	c.setSession(username, response)
	http.Error(response, "Trust Profile saved.", http.StatusOK)
	c.reloadAppNow()
}

func (c *Client) handleInstanceCheck(response http.ResponseWriter, request *http.Request) {
	mnd.ConfigPostDecoder.RegisterConverter([]string{}, func(input string) reflect.Value {
		return reflect.ValueOf(strings.Fields(input))
	})

	if err := request.ParseForm(); err != nil {
		http.Error(response, "Parsing form data failed: "+err.Error(), http.StatusBadRequest)
		return
	}

	checkapp.Test(c.Config, response, request)
}

// handleFileBrowser returns a list of files and folders in a path.
// part of the file browser javascript code.
func (c *Client) handleFileBrowser(response http.ResponseWriter, request *http.Request) {
	type dir struct {
		Dirs  []string `json:"dirs"`
		Files []string `json:"files"`
	}

	output := dir{Dirs: []string{}, Files: []string{}}

	switch dirPath := mux.Vars(request)["dir"]; {
	case dirPath != "":
		dir, err := os.ReadDir(filepath.Join(dirPath, "/"))
		if err != nil {
			http.Error(response, err.Error(), http.StatusNotAcceptable)
			return
		}

		for _, file := range dir {
			if file.IsDir() {
				output.Dirs = append(output.Dirs, file.Name())
			} else {
				output.Files = append(output.Files, file.Name())
			}
		}
	case runtime.GOOS == mnd.Windows:
		partitions, err := disk.PartitionsWithContext(request.Context(), false)
		if err != nil {
			c.Errorf("Getting disk partitions: %v", err)
		}
		// this runs anyway.
		for _, partition := range partitions {
			output.Dirs = append(output.Dirs, partition.Mountpoint)
		}
	default:
		output.Dirs = []string{"/"}
	}

	if err := json.NewEncoder(response).Encode(&output); err != nil {
		c.Errorf("Encoding file browser directory: %v", err)
	}
}

// handleCommandStats is for js getCmdStats.
func (c *Client) handleCommandStats(response http.ResponseWriter, request *http.Request) {
	cmd := c.triggers.Commands.GetByHash(mux.Vars(request)["hash"])
	if cmd == nil {
		http.Error(response, "Invalid command Hash provided", http.StatusBadRequest)
		return
	}

	uri := "ajax/" + mux.Vars(request)["path"] + ".html"

	if err := c.template.ExecuteTemplate(response, uri, cmd); err != nil {
		http.Error(response, "template error: "+err.Error(), http.StatusOK)
	}
}

// handleRunCommand only handles commands with arguments.
// Commands without arguments are handled as an instance test.
func (c *Client) handleRunCommand(response http.ResponseWriter, request *http.Request) {
	cmd := c.triggers.Commands.GetByHash(mux.Vars(request)["hash"])
	if cmd == nil {
		http.Error(response, "Invalid command Hash provided", http.StatusBadRequest)
		return
	}

	_ = request.ParseForm()

	cmd.Run(&common.ActionInput{
		Type: website.EventGUI,
		Args: request.PostForm["args"],
	})
	http.Error(response, "Check command output after a few seconds.", http.StatusOK)
}

// handleProcessList just returns the running process list for a human to view.
func (c *Client) handleProcessList(response http.ResponseWriter, request *http.Request) {
	if ps, err := getProcessList(request.Context()); err != nil {
		http.Error(response, err.Error(), http.StatusInternalServerError)
	} else if _, err = ps.WriteTo(response); err != nil {
		user, _ := c.getUserName(request)
		c.Errorf("[gui '%s' requested] Writing HTTP Response: %v", user, err)
	}
}

func (c *Client) handleStartFileWatcher(response http.ResponseWriter, request *http.Request) {
	idx, err := strconv.Atoi(mux.Vars(request)["index"])
	if err != nil {
		http.Error(response, "invalid index provided:"+mux.Vars(request)["index"], http.StatusBadRequest)
		return
	}

	if idx < 0 || idx >= len(c.triggers.FileWatch.Files()) {
		http.Error(response, "unknown index provided:"+mux.Vars(request)["index"], http.StatusBadRequest)
		return
	}

	watch := c.triggers.FileWatch.Files()[idx]
	if watch.Active() {
		http.Error(response, "Watcher already running! "+watch.Path, http.StatusNotAcceptable)
		return
	}

	if err := c.triggers.FileWatch.AddFileWatcher(watch); err != nil {
		http.Error(response, "Start Failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Error(response, "Started: "+watch.Path, http.StatusOK)
}

func (c *Client) handleStopFileWatcher(response http.ResponseWriter, request *http.Request) {
	idx, err := strconv.Atoi(mux.Vars(request)["index"])
	if err != nil {
		http.Error(response, "invalid index provided:"+mux.Vars(request)["index"], http.StatusBadRequest)
		return
	}

	if idx < 0 || idx >= len(c.triggers.FileWatch.Files()) {
		http.Error(response, "unknown index provided:"+mux.Vars(request)["index"], http.StatusBadRequest)
		return
	}

	watch := c.triggers.FileWatch.Files()[idx]
	if !watch.Active() {
		http.Error(response, "Watcher already stopped! "+watch.Path, http.StatusNotAcceptable)
		return
	}

	if err := watch.Stop(); err != nil {
		http.Error(response, "Stop Failed: "+err.Error(), http.StatusInternalServerError)

		user, _ := c.getUserName(request)
		c.Errorf("[gui '%s' requested] Stopping File Watcher: %v", user, err)

		return
	}

	http.Error(response, "Stopped: "+watch.Path, http.StatusOK)
}

// handleRegexTest tests a regular expression.
func (c *Client) handleRegexTest(response http.ResponseWriter, request *http.Request) {
	regex := request.PostFormValue("regexTestRegex")
	line := request.PostFormValue("regexTestLine")

	switch reg, err := regexp.Compile(regex); {
	case err != nil:
		http.Error(response, "Regex Parse Failed: "+err.Error(), http.StatusNotAcceptable)
	case regex == "":
		http.Error(response, "Regular Expression is blank!", http.StatusBadRequest)
	case reg.MatchString(line):
		http.Error(response, "Regular Expression matches! Found: "+reg.FindString(line), http.StatusOK)
	default:
		http.Error(response, "Regular Expression does not match!", http.StatusBadRequest)
	}
}

// handleConfigPost handles the reconfig endpoint.
func (c *Client) handleConfigPost(response http.ResponseWriter, request *http.Request) {
	user, _ := c.getUserName(request)
	// copy running config,
	config, err := c.Config.CopyConfig()
	if err != nil {
		c.Errorf("[gui '%s' requested] Copying Config (GUI request): %v", user, err)
		http.Error(response, "Error copying internal configuration (this is a bug): "+
			err.Error(), http.StatusInternalServerError)

		return
	}

	// update config.
	if err = c.mergeAndValidateNewConfig(config, request); err != nil {
		c.Errorf("[gui '%s' requested] Validating POSTed Config: %v", user, err)
		http.Error(response, err.Error(), http.StatusBadRequest)

		return
	}

	// Check app integration configs before saving.
	config.Apps.Logger = c.Logger
	if err := config.Apps.Setup(); err != nil {
		http.Error(response, err.Error(), http.StatusNotAcceptable)
		return
	}

	if err := c.saveNewConfig(request.Context(), config); err != nil {
		c.Errorf("[gui '%s' requested] Saving Config: %v", user, err)
		http.Error(response, "Saving Config: "+err.Error(), http.StatusInternalServerError)

		return
	}

	// reload.
	defer c.triggerConfigReload(website.EventGUI, "GUI Requested")

	c.Lock()
	c.reloading = true
	c.Unlock()

	// respond.
	c.Printf("[gui '%s' requested] Updated Configuration.", user)
	http.Error(response, "Config Saved. Reloading in 5 seconds...", http.StatusOK)
}

// saveNewConfig takes a fully built (copy) of config data, and saves it as the config file.
func (c *Client) saveNewConfig(ctx context.Context, config *configfile.Config) error {
	date := time.Now().Format("20060102T150405") // for file names.

	// make config file backup.
	if err := configfile.BackupFile(c.Flags.ConfigFile); err != nil {
		return fmt.Errorf("backing up config file: %w", err)
	}

	// write new config file to temporary path.
	destFile := filepath.Join(filepath.Dir(c.Flags.ConfigFile), "_tmpConfig."+date)
	if _, err := config.Write(ctx, destFile, true); err != nil { // write our config file template.
		return fmt.Errorf("writing new config file: %w", err)
	}

	// move new config file to existing config file.
	if err := os.Rename(destFile, c.Flags.ConfigFile); err != nil {
		return fmt.Errorf("renaming temporary file: %w", err)
	}

	return nil
}

func (c *Client) mergeAndValidateNewConfig(config *configfile.Config, request *http.Request) error {
	// This turns text fields into a []string (extra keys and upstreams use this).
	mnd.ConfigPostDecoder.RegisterConverter([]string{}, func(input string) reflect.Value {
		return reflect.ValueOf(strings.Fields(input))
	})

	if err := request.ParseForm(); err != nil {
		return fmt.Errorf("parsing form data failed: %w", err)
	}

	if config.Snapshot == nil {
		config.Snapshot = &snapshot.Config{}
	}

	if config.Apps != nil {
		config.Apps.Lidarr = nil
		config.Apps.Prowlarr = nil
		config.Apps.Radarr = nil
		config.Apps.Readarr = nil
		config.Apps.Sonarr = nil
		config.Apps.Qbit = nil
		config.Apps.Rtorrent = nil
		config.Apps.Deluge = nil
		config.Apps.SabNZB = nil
		config.Apps.NZBGet = nil
		config.Apps.Tautulli = nil
		config.Apps.Transmission = nil
		config.Apps.Qbit = nil
		config.Apps.Plex = nil
	}

	config.SSLCrtFile = ""
	config.SSLKeyFile = ""
	config.Plex = nil
	config.WatchFiles = nil
	config.Commands = nil
	config.Service = nil
	config.Snapshot.Plugins.MySQL = nil
	config.Snapshot.Plugins.Nvidia = nil

	// for k, v := range request.PostForm {
	// 	c.Errorf("Config Post: %s = %+v", k, v)
	// }

	// Decode the POST'd data directly into the mostly-empty config struct.
	if err := mnd.ConfigPostDecoder.Decode(config, request.PostForm); err != nil {
		return fmt.Errorf("decoding POST data into Go data structure failed: %w", err)
	}

	return c.validateNewConfig(config)
}

func (c *Client) validateNewConfig(config *configfile.Config) error {
	for idx, cmd := range config.Commands {
		if err := cmd.SetupRegexpArgs(); err != nil {
			return fmt.Errorf("command %d '%s' failed setup: %w", idx+1, cmd.Name, err)
		}
	}

	if err := c.validateNewServiceConfig(config); err != nil {
		return err
	}

	copied, err := config.CopyConfig()
	if err != nil {
		return fmt.Errorf("copying config: %w", err)
	}

	_, err = cnfgfile.Parse(copied, &cnfgfile.Opts{
		Name:          mnd.Title,
		TransformPath: configfile.ExpandHomedir,
		Prefix:        "filepath:",
	})
	if err != nil {
		return fmt.Errorf("filepath: %w", err)
	}

	return nil
}

func (c *Client) validateNewServiceConfig(config *configfile.Config) error {
	// Check service checks for non-unique names.
	serviceNames := make(map[string]struct{})
	index := 0

	for _, svc := range config.Service {
		if svc == nil {
			continue
		}

		config.Service[index] = svc
		index++

		if _, ok := serviceNames[svc.Name]; ok {
			return fmt.Errorf("%w (%d): %s", services.ErrNoName, index+1, svc.Name)
		}

		if err := svc.Validate(); err != nil {
			return fmt.Errorf("validating service check %d: %w", index+1, err)
		}

		serviceNames[svc.Name] = struct{}{}
	}

	// Clean up to avoid leaking memory.
	for j := index; j < len(config.Service); j++ {
		config.Service[j] = nil
	}

	config.Service = config.Service[:index]

	return nil
}

func (c *Client) indexPage(ctx context.Context, response http.ResponseWriter, request *http.Request, msg string) {
	response.Header().Add("Content-Type", "text/html")

	user, _ := c.getUserName(request)
	if request.Method != http.MethodGet || (user == "" && c.webauth) {
		response.WriteHeader(http.StatusUnauthorized)
	}

	c.renderTemplate(ctx, response, request, "index.html", msg)
}

func (c *Client) getTemplatePageHandler(response http.ResponseWriter, req *http.Request) {
	page := mux.Vars(req)["template"] + ".html"
	if c.template.Lookup(page) == nil {
		page = filepath.Join(mux.Vars(req)["template"], "index.html")
	}

	c.renderTemplate(req.Context(), response, req, page, "")
}

func (c *Client) handlerSwaggerDoc(response http.ResponseWriter, request *http.Request) {
	instance := strings.TrimSuffix(mux.Vars(request)["instance"], ".json")
	if instance == "" {
		instance = "api"
	}

	if version.Version == "" {
		docs.SwaggerInfoapi.Version = "v.dev"
	} else {
		docs.SwaggerInfoapi.Version = "v" + version.Version + "-" + version.Revision
	}

	docs.SwaggerInfoapi.BasePath = c.Config.URLBase
	docs.SwaggerInfoapi.Host = request.Host

	doc, err := swag.ReadDoc(instance)
	if err != nil {
		http.Error(response, err.Error(), http.StatusInternalServerError)
		return
	}

	_, _ = response.Write([]byte(doc))
}

func (c *Client) handleSwaggerIndex(response http.ResponseWriter, request *http.Request) {
	c.renderTemplate(request.Context(), response, request, "swagger/index.html", "")
}

func (c *Client) handleStaticAssets(resp http.ResponseWriter, req *http.Request) {
	if req.URL.Path == "/files/css/custom.css" {
		if cssFile := c.haveCustomFile("custom.css"); cssFile != "" {
			// custom css file exists on disk, use http.FileServer to serve the dir it's in.
			http.StripPrefix("/files/css", http.FileServer(http.Dir(filepath.Dir(cssFile)))).ServeHTTP(resp, req)
			return
		}
	}

	internalServer := statigz.FileServer(bindata.Files)
	if c.Flags.Assets == "" {
		internalServer.ServeHTTP(resp, req)
	} else {
		statigz.FileServer(os.DirFS(c.Flags.Assets).(fs.ReadDirFS), //nolint:forcetypeassert // This is OK!
			statigz.OnNotFound(internalServer.ServeHTTP)).ServeHTTP(resp, req)
	}
}
