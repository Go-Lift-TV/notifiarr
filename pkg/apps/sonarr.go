package apps

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Notifiarr/notifiarr/pkg/mnd"
	"github.com/gorilla/mux"
	"golift.io/cnfg"
	"golift.io/starr"
	"golift.io/starr/sonarr"
)

// sonarrHandlers is called once on startup to register the web API paths.
func (a *Apps) sonarrHandlers() {
	a.HandleAPIpath(starr.Sonarr, "/add", sonarrAddSeries, "POST")
	a.HandleAPIpath(starr.Sonarr, "/check/{tvdbid:[0-9]+}", sonarrCheckSeries, "GET")
	a.HandleAPIpath(starr.Sonarr, "/get/{seriesid:[0-9]+}", sonarrGetSeries, "GET")
	a.HandleAPIpath(starr.Sonarr, "/getEpisodes/{seriesid:[0-9]+}", sonarrGetEpisodes, "GET")
	a.HandleAPIpath(starr.Sonarr, "/unmonitor/{episodeid:[0-9]+}", sonarrUnmonitorEpisode, "GET")
	a.HandleAPIpath(starr.Sonarr, "/languageProfiles", sonarrLangProfiles, "GET")
	a.HandleAPIpath(starr.Sonarr, "/qualityProfiles", sonarrGetQualityProfiles, "GET")
	a.HandleAPIpath(starr.Sonarr, "/qualityProfile", sonarrGetQualityProfile, "GET")
	a.HandleAPIpath(starr.Sonarr, "/qualityProfile", sonarrAddQualityProfile, "POST")
	a.HandleAPIpath(starr.Sonarr, "/qualityProfile/{profileID:[0-9]+}", sonarrUpdateQualityProfile, "PUT")
	a.HandleAPIpath(starr.Sonarr, "/releaseProfiles", sonarrGetReleaseProfiles, "GET")
	a.HandleAPIpath(starr.Sonarr, "/releaseProfile", sonarrAddReleaseProfile, "POST")
	a.HandleAPIpath(starr.Sonarr, "/releaseProfile/{profileID:[0-9]+}", sonarrUpdateReleaseProfile, "PUT")
	a.HandleAPIpath(starr.Sonarr, "/rootFolder", sonarrRootFolders, "GET")
	a.HandleAPIpath(starr.Sonarr, "/search/{query}", sonarrSearchSeries, "GET")
	a.HandleAPIpath(starr.Sonarr, "/tag", sonarrGetTags, "GET")
	a.HandleAPIpath(starr.Sonarr, "/tag/{tid:[0-9]+}/{label}", sonarrUpdateTag, "PUT")
	a.HandleAPIpath(starr.Sonarr, "/tag/{label}", sonarrSetTag, "PUT")
	a.HandleAPIpath(starr.Sonarr, "/update", sonarrUpdateSeries, "PUT")
	a.HandleAPIpath(starr.Sonarr, "/command/search/{seriesid:[0-9]+}", sonarrTriggerSearchSeries, "GET")
}

// SonarrConfig represents the input data for a Sonarr server.
type SonarrConfig struct {
	Name      string        `toml:"name" xml:"name"`
	Interval  cnfg.Duration `toml:"interval" xml:"interval"`
	StuckItem bool          `toml:"stuck_items" xml:"stuck_items"`
	CheckQ    *uint         `toml:"check_q" xml:"check_q"`
	*starr.Config
	*sonarr.Sonarr
}

func (r *SonarrConfig) setup(timeout time.Duration) {
	r.Sonarr = sonarr.New(r.Config)
	if r.Timeout.Duration == 0 {
		r.Timeout.Duration = timeout
	}

	// These things are not used in this package but this package configures them.
	if r.StuckItem && r.CheckQ == nil {
		i := uint(0)
		r.CheckQ = &i
	} else if r.CheckQ != nil {
		r.StuckItem = true
	}
}

func sonarrAddSeries(r *http.Request) (int, interface{}) {
	var payload sonarr.AddSeriesInput
	// Extract payload and check for TVDB ID.
	err := json.NewDecoder(r.Body).Decode(&payload)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("decoding payload: %w", err)
	} else if payload.TvdbID == 0 {
		return http.StatusUnprocessableEntity, fmt.Errorf("0: %w", ErrNoTMDB)
	}

	app := getSonarr(r)
	// Check for existing series.
	m, err := app.GetSeries(payload.TvdbID)
	if err != nil {
		return http.StatusServiceUnavailable, fmt.Errorf("checking series: %w", err)
	} else if len(m) > 0 {
		return http.StatusConflict, sonarrData(m[0])
	}

	series, err := app.AddSeries(&payload)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("adding series: %w", err)
	}

	return http.StatusCreated, series
}

func sonarrData(series *sonarr.Series) map[string]interface{} {
	hasFile := false
	if series.Statistics != nil {
		hasFile = series.Statistics.SizeOnDisk > 0
	}

	return map[string]interface{}{
		"id":        series.ID,
		"hasFile":   hasFile,
		"monitored": series.Monitored,
	}
}

func sonarrCheckSeries(r *http.Request) (int, interface{}) {
	tvdbid, _ := strconv.ParseInt(mux.Vars(r)["tvdbid"], mnd.Base10, mnd.Bits64)
	// Check for existing series.
	m, err := getSonarr(r).GetSeries(tvdbid)
	if err != nil {
		return http.StatusServiceUnavailable, fmt.Errorf("checking series: %w", err)
	} else if len(m) > 0 {
		return http.StatusConflict, sonarrData(m[0])
	}

	return http.StatusOK, http.StatusText(http.StatusNotFound)
}

func sonarrGetSeries(r *http.Request) (int, interface{}) {
	seriesID, _ := strconv.ParseInt(mux.Vars(r)["seriesid"], mnd.Base10, mnd.Bits64)

	series, err := getSonarr(r).GetSeriesByID(seriesID)
	if err != nil {
		return http.StatusServiceUnavailable, fmt.Errorf("checking series: %w", err)
	}

	return http.StatusOK, series
}

func sonarrGetEpisodes(r *http.Request) (int, interface{}) {
	seriesID, _ := strconv.ParseInt(mux.Vars(r)["seriesid"], mnd.Base10, mnd.Bits64)

	episodes, err := getSonarr(r).GetSeriesEpisodes(seriesID)
	if err != nil {
		return http.StatusServiceUnavailable, fmt.Errorf("checking series: %w", err)
	}

	return http.StatusOK, episodes
}

func sonarrUnmonitorEpisode(r *http.Request) (int, interface{}) {
	episodeID, _ := strconv.ParseInt(mux.Vars(r)["episodeid"], mnd.Base10, mnd.Bits64)

	episodes, err := getSonarr(r).MonitorEpisode([]int64{episodeID}, false)
	if err != nil {
		return http.StatusServiceUnavailable, fmt.Errorf("checking series: %w", err)
	} else if len(episodes) != 1 {
		return http.StatusServiceUnavailable, fmt.Errorf("%w (%d): %v", ErrWrongCount, len(episodes), episodes)
	}

	return http.StatusOK, episodes[0]
}

func sonarrTriggerSearchSeries(r *http.Request) (int, interface{}) {
	seriesID, _ := strconv.ParseInt(mux.Vars(r)["seriesid"], mnd.Base10, mnd.Bits64)

	output, err := getSonarr(r).SendCommand(&sonarr.CommandRequest{
		Name:     "SeriesSearch",
		SeriesID: seriesID,
	})
	if err != nil {
		return http.StatusServiceUnavailable, fmt.Errorf("triggering series search: %w", err)
	}

	return http.StatusOK, output.Status
}

func sonarrLangProfiles(r *http.Request) (int, interface{}) {
	// Get the profiles from sonarr.
	profiles, err := getSonarr(r).GetLanguageProfiles()
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("getting language profiles: %w", err)
	}

	// Format profile ID=>Name into a nice map.
	p := make(map[int64]string)
	for i := range profiles {
		p[profiles[i].ID] = profiles[i].Name
	}

	return http.StatusOK, p
}

func sonarrGetQualityProfile(r *http.Request) (int, interface{}) {
	// Get the profiles from sonarr.
	profiles, err := getSonarr(r).GetQualityProfiles()
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("getting profiles: %w", err)
	}

	return http.StatusOK, profiles
}

func sonarrGetQualityProfiles(r *http.Request) (int, interface{}) {
	// Get the profiles from sonarr.
	profiles, err := getSonarr(r).GetQualityProfiles()
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("getting profiles: %w", err)
	}

	// Format profile ID=>Name into a nice map.
	p := make(map[int64]string)
	for i := range profiles {
		p[profiles[i].ID] = profiles[i].Name
	}

	return http.StatusOK, p
}

func sonarrAddQualityProfile(r *http.Request) (int, interface{}) {
	var profile sonarr.QualityProfile

	// Extract payload and check for TMDB ID.
	err := json.NewDecoder(r.Body).Decode(&profile)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("decoding payload: %w", err)
	}

	// Get the profiles from radarr.
	id, err := getSonarr(r).AddQualityProfile(&profile)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("adding profile: %w", err)
	}

	return http.StatusOK, id
}

func sonarrUpdateQualityProfile(r *http.Request) (int, interface{}) {
	var profile sonarr.QualityProfile

	// Extract payload and check for TMDB ID.
	err := json.NewDecoder(r.Body).Decode(&profile)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("decoding payload: %w", err)
	}

	profile.ID, _ = strconv.ParseInt(mux.Vars(r)["profileID"], mnd.Base10, mnd.Bits64)
	if profile.ID == 0 {
		return http.StatusBadRequest, ErrNonZeroID
	}

	// Get the profiles from radarr.
	err = getSonarr(r).UpdateQualityProfile(&profile)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("updating profile: %w", err)
	}

	return http.StatusOK, "OK"
}

func sonarrGetReleaseProfiles(r *http.Request) (int, interface{}) {
	// Get the profiles from sonarr.
	profiles, err := getSonarr(r).GetReleaseProfiles()
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("getting profiles: %w", err)
	}

	return http.StatusOK, profiles
}

func sonarrAddReleaseProfile(r *http.Request) (int, interface{}) {
	var profile sonarr.ReleaseProfile

	// Extract payload and check for TMDB ID.
	err := json.NewDecoder(r.Body).Decode(&profile)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("decoding payload: %w", err)
	}

	// Get the profiles from radarr.
	id, err := getSonarr(r).AddReleaseProfile(&profile)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("adding profile: %w", err)
	}

	return http.StatusOK, id
}

func sonarrUpdateReleaseProfile(r *http.Request) (int, interface{}) {
	var profile sonarr.ReleaseProfile

	// Extract payload and check for TMDB ID.
	err := json.NewDecoder(r.Body).Decode(&profile)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("decoding payload: %w", err)
	}

	profile.ID, _ = strconv.ParseInt(mux.Vars(r)["profileID"], mnd.Base10, mnd.Bits64)
	if profile.ID == 0 {
		return http.StatusBadRequest, ErrNonZeroID
	}

	// Get the profiles from radarr.
	err = getSonarr(r).UpdateReleaseProfile(&profile)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("updating profile: %w", err)
	}

	return http.StatusOK, "OK"
}

func sonarrRootFolders(r *http.Request) (int, interface{}) {
	// Get folder list from Sonarr.
	folders, err := getSonarr(r).GetRootFolders()
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("getting folders: %w", err)
	}

	// Format folder list into a nice path=>freesSpace map.
	p := make(map[string]int64)
	for i := range folders {
		p[folders[i].Path] = folders[i].FreeSpace
	}

	return http.StatusOK, p
}

func sonarrSearchSeries(r *http.Request) (int, interface{}) {
	// Get all movies
	series, err := getSonarr(r).GetAllSeries()
	if err != nil {
		return http.StatusServiceUnavailable, fmt.Errorf("getting series: %w", err)
	}

	query := strings.TrimSpace(strings.ToLower(mux.Vars(r)["query"])) // in
	returnSeries := make([]map[string]interface{}, 0)                 // out

	for _, s := range series {
		if seriesSearch(query, s.Title, s.AlternateTitles) {
			b := map[string]interface{}{
				"id":                s.ID,
				"title":             s.Title,
				"first":             s.FirstAired,
				"next":              s.NextAiring,
				"prev":              s.PreviousAiring,
				"added":             s.Added,
				"status":            s.Status,
				"path":              s.Path,
				"tvdbId":            s.TvdbID,
				"monitored":         s.Monitored,
				"qualityProfileId":  s.QualityProfileID,
				"seasonFolder":      s.SeasonFolder,
				"seriesType":        s.SeriesType,
				"languageProfileId": s.LanguageProfileID,
				"seasons":           s.Seasons,
				"exists":            false,
			}

			if s.Statistics != nil {
				b["exists"] = s.Statistics.SizeOnDisk > 0
			}

			returnSeries = append(returnSeries, b)
		}
	}

	return http.StatusOK, returnSeries
}

func seriesSearch(query, title string, alts []*sonarr.AlternateTitle) bool {
	if strings.Contains(strings.ToLower(title), query) {
		return true
	}

	for _, t := range alts {
		if strings.Contains(strings.ToLower(t.Title), query) {
			return true
		}
	}

	return false
}

func sonarrGetTags(r *http.Request) (int, interface{}) {
	tags, err := getSonarr(r).GetTags()
	if err != nil {
		return http.StatusServiceUnavailable, fmt.Errorf("getting tags: %w", err)
	}

	return http.StatusOK, tags
}

func sonarrUpdateTag(r *http.Request) (int, interface{}) {
	id, _ := strconv.Atoi(mux.Vars(r)["tid"])

	tagID, err := getSonarr(r).UpdateTag(id, mux.Vars(r)["label"])
	if err != nil {
		return http.StatusServiceUnavailable, fmt.Errorf("updating tag: %w", err)
	}

	return http.StatusOK, tagID
}

func sonarrSetTag(r *http.Request) (int, interface{}) {
	tagID, err := getSonarr(r).AddTag(mux.Vars(r)["label"])
	if err != nil {
		return http.StatusServiceUnavailable, fmt.Errorf("setting tag: %w", err)
	}

	return http.StatusOK, tagID
}

func sonarrUpdateSeries(r *http.Request) (int, interface{}) {
	var series sonarr.Series

	err := json.NewDecoder(r.Body).Decode(&series)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("decoding payload: %w", err)
	}

	err = getSonarr(r).UpdateSeries(series.ID, &series)
	if err != nil {
		return http.StatusServiceUnavailable, fmt.Errorf("updating series: %w", err)
	}

	return http.StatusOK, "sonarr seems to have worked"
}
