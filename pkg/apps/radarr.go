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
	"golift.io/starr/radarr"
)

// radarrHandlers is called once on startup to register the web API paths.
func (a *Apps) radarrHandlers() {
	a.HandleAPIpath(starr.Radarr, "/add", radarrAddMovie, "POST")
	a.HandleAPIpath(starr.Radarr, "/check/{tmdbid:[0-9]+}", radarrCheckMovie, "GET")
	a.HandleAPIpath(starr.Radarr, "/get/{movieid:[0-9]+}", radarrGetMovie, "GET")
	a.HandleAPIpath(starr.Radarr, "/get", radarrGetAllMovies, "GET")
	a.HandleAPIpath(starr.Radarr, "/qualityProfiles", radarrQualityProfiles, "GET")
	a.HandleAPIpath(starr.Radarr, "/qualityProfile", radarrQualityProfile, "GET")
	a.HandleAPIpath(starr.Radarr, "/qualityProfile", radarrAddQualityProfile, "POST")
	a.HandleAPIpath(starr.Radarr, "/qualityProfile/{profileID:[0-9]+}", radarrUpdateQualityProfile, "PUT")
	a.HandleAPIpath(starr.Radarr, "/rootFolder", radarrRootFolders, "GET")
	a.HandleAPIpath(starr.Radarr, "/search/{query}", radarrSearchMovie, "GET")
	a.HandleAPIpath(starr.Radarr, "/tag", radarrGetTags, "GET")
	a.HandleAPIpath(starr.Radarr, "/tag/{tid:[0-9]+}/{label}", radarrUpdateTag, "PUT")
	a.HandleAPIpath(starr.Radarr, "/tag/{label}", radarrSetTag, "PUT")
	a.HandleAPIpath(starr.Radarr, "/update", radarrUpdateMovie, "PUT")
	a.HandleAPIpath(starr.Radarr, "/exclusions", radarrGetExclusions, "GET")
	a.HandleAPIpath(starr.Radarr, "/exclusions", radarrAddExclusions, "POST")
	a.HandleAPIpath(starr.Radarr, "/exclusions/{eid:(?:[0-9],?)+}", radarrDelExclusions, "DELETE")
	a.HandleAPIpath(starr.Radarr, "/customformats", radarrGetCustomFormats, "GET")
	a.HandleAPIpath(starr.Radarr, "/customformats", radarrAddCustomFormat, "POST")
	a.HandleAPIpath(starr.Radarr, "/customformats/{cfid:[0-9]+}", radarrUpdateCustomFormat, "PUT")
	a.HandleAPIpath(starr.Radarr, "/command/search/{movieid:[0-9]+}", radarrTriggerSearchMovie, "GET")
}

// RadarrConfig represents the input data for a Radarr server.
type RadarrConfig struct {
	Name      string        `toml:"name" xml:"name"`
	Interval  cnfg.Duration `toml:"interval" xml:"interval"`
	StuckItem bool          `toml:"stuck_items" xml:"stuck_items"`
	CheckQ    *uint         `toml:"check_q" xml:"check_q"`
	*starr.Config
	*radarr.Radarr
}

func (r *RadarrConfig) setup(timeout time.Duration) {
	r.Radarr = radarr.New(r.Config)
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

func radarrAddMovie(r *http.Request) (int, interface{}) {
	var payload radarr.AddMovieInput
	// Extract payload and check for TMDB ID.
	err := json.NewDecoder(r.Body).Decode(&payload)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("decoding payload: %w", err)
	} else if payload.TmdbID == 0 {
		return http.StatusUnprocessableEntity, fmt.Errorf("0: %w", ErrNoTMDB)
	}

	app := getRadarr(r)
	// Check for existing movie.
	m, err := app.GetMovie(payload.TmdbID)
	if err != nil {
		return http.StatusServiceUnavailable, fmt.Errorf("checking movie: %w", err)
	} else if len(m) > 0 {
		return http.StatusConflict, radarrData(m[0])
	}

	if payload.Title == "" {
		// Title must exist, even if it's wrong.
		payload.Title = strconv.FormatInt(payload.TmdbID, mnd.Base10)
	}

	if payload.MinimumAvailability == "" {
		payload.MinimumAvailability = "released"
	}

	// Add movie using fixed payload.
	movie, err := app.AddMovie(&payload)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("adding movie: %w", err)
	}

	return http.StatusCreated, movie
}

func radarrData(movie *radarr.Movie) map[string]interface{} {
	return map[string]interface{}{
		"id":        movie.ID,
		"hasFile":   movie.HasFile,
		"monitored": movie.Monitored,
	}
}

func radarrCheckMovie(r *http.Request) (int, interface{}) {
	tmdbID, _ := strconv.ParseInt(mux.Vars(r)["tmdbid"], mnd.Base10, mnd.Bits64)
	// Check for existing movie.
	m, err := getRadarr(r).GetMovie(tmdbID)
	if err != nil {
		return http.StatusServiceUnavailable, fmt.Errorf("checking movie: %w", err)
	} else if len(m) > 0 {
		return http.StatusConflict, radarrData(m[0])
	}

	return http.StatusOK, http.StatusText(http.StatusNotFound)
}

func radarrGetMovie(r *http.Request) (int, interface{}) {
	movieID, _ := strconv.ParseInt(mux.Vars(r)["movieid"], mnd.Base10, mnd.Bits64)

	movie, err := getRadarr(r).GetMovieByID(movieID)
	if err != nil {
		return http.StatusServiceUnavailable, fmt.Errorf("checking movie: %w", err)
	}

	return http.StatusOK, movie
}

func radarrTriggerSearchMovie(r *http.Request) (int, interface{}) {
	movieID, _ := strconv.ParseInt(mux.Vars(r)["movieid"], mnd.Base10, mnd.Bits64)

	output, err := getRadarr(r).SendCommand(&radarr.CommandRequest{
		Name:     "MoviesSearch",
		MovieIDs: []int64{movieID},
	})
	if err != nil {
		return http.StatusServiceUnavailable, fmt.Errorf("triggering movie search: %w", err)
	}

	return http.StatusOK, output.Status
}

func radarrGetAllMovies(r *http.Request) (int, interface{}) {
	movies, err := getRadarr(r).GetMovie(0)
	if err != nil {
		return http.StatusServiceUnavailable, fmt.Errorf("checking movie: %w", err)
	}

	return http.StatusOK, movies
}

func radarrQualityProfile(r *http.Request) (int, interface{}) {
	// Get the profiles from radarr.
	profiles, err := getRadarr(r).GetQualityProfiles()
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("getting profiles: %w", err)
	}

	return http.StatusOK, profiles
}

func radarrQualityProfiles(r *http.Request) (int, interface{}) {
	// Get the profiles from radarr.
	profiles, err := getRadarr(r).GetQualityProfiles()
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

func radarrAddQualityProfile(r *http.Request) (int, interface{}) {
	var profile radarr.QualityProfile

	// Extract payload and check for TMDB ID.
	err := json.NewDecoder(r.Body).Decode(&profile)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("decoding payload: %w", err)
	}

	// Get the profiles from radarr.
	id, err := getRadarr(r).AddQualityProfile(&profile)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("adding profile: %w", err)
	}

	return http.StatusOK, id
}

func radarrUpdateQualityProfile(r *http.Request) (int, interface{}) {
	var profile radarr.QualityProfile

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
	err = getRadarr(r).UpdateQualityProfile(&profile)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("updating profile: %w", err)
	}

	return http.StatusOK, "OK"
}

func radarrRootFolders(r *http.Request) (int, interface{}) {
	// Get folder list from Radarr.
	folders, err := getRadarr(r).GetRootFolders()
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

func radarrSearchMovie(r *http.Request) (int, interface{}) {
	// Get all movies
	movies, err := getRadarr(r).GetMovie(0)
	if err != nil {
		return http.StatusServiceUnavailable, fmt.Errorf("getting movies: %w", err)
	}

	query := strings.TrimSpace(strings.ToLower(mux.Vars(r)["query"])) // in
	returnMovies := make([]map[string]interface{}, 0)                 // out

	for _, movie := range movies {
		if movieSearch(query, []string{movie.Title, movie.OriginalTitle}, movie.AlternateTitles) {
			returnMovies = append(returnMovies, map[string]interface{}{
				"id":                  movie.ID,
				"title":               movie.Title,
				"cinemas":             movie.InCinemas,
				"status":              movie.Status,
				"exists":              movie.HasFile,
				"added":               movie.Added,
				"year":                movie.Year,
				"path":                movie.Path,
				"tmdbId":              movie.TmdbID,
				"qualityProfileId":    movie.QualityProfileID,
				"monitored":           movie.Monitored,
				"minimumAvailability": movie.MinimumAvailability,
			})
		}
	}

	return http.StatusOK, returnMovies
}

func movieSearch(query string, titles []string, alts []*radarr.AlternativeTitle) bool {
	for _, t := range titles {
		if t != "" && strings.Contains(strings.ToLower(t), query) {
			return true
		}
	}

	for _, t := range alts {
		if strings.Contains(strings.ToLower(t.Title), query) {
			return true
		}
	}

	return false
}

func radarrGetTags(r *http.Request) (int, interface{}) {
	tags, err := getRadarr(r).GetTags()
	if err != nil {
		return http.StatusServiceUnavailable, fmt.Errorf("getting tags: %w", err)
	}

	return http.StatusOK, tags
}

func radarrUpdateTag(r *http.Request) (int, interface{}) {
	id, _ := strconv.Atoi(mux.Vars(r)["tid"])

	tagID, err := getRadarr(r).UpdateTag(id, mux.Vars(r)["label"])
	if err != nil {
		return http.StatusServiceUnavailable, fmt.Errorf("updating tag: %w", err)
	}

	return http.StatusOK, tagID
}

func radarrSetTag(r *http.Request) (int, interface{}) {
	tagID, err := getRadarr(r).AddTag(mux.Vars(r)["label"])
	if err != nil {
		return http.StatusServiceUnavailable, fmt.Errorf("setting tag: %w", err)
	}

	return http.StatusOK, tagID
}

func radarrUpdateMovie(r *http.Request) (int, interface{}) {
	var movie radarr.Movie
	// Extract payload and check for TMDB ID.
	err := json.NewDecoder(r.Body).Decode(&movie)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("decoding payload: %w", err)
	}

	// Check for existing movie.
	err = getRadarr(r).UpdateMovie(movie.ID, &movie)
	if err != nil {
		return http.StatusServiceUnavailable, fmt.Errorf("updating movie: %w", err)
	}

	return http.StatusOK, "radarr seems to have worked"
}

func radarrAddExclusions(r *http.Request) (int, interface{}) {
	var exclusions []*radarr.Exclusion

	err := json.NewDecoder(r.Body).Decode(&exclusions)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("decoding payload: %w", err)
	}

	// Get the profiles from radarr.
	err = getRadarr(r).AddExclusions(exclusions)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("adding exclusions: %w", err)
	}

	return http.StatusOK, "added " + strconv.Itoa(len(exclusions)) + " exclusions"
}

func radarrGetExclusions(r *http.Request) (int, interface{}) {
	exclusions, err := getRadarr(r).GetExclusions()
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("getting exclusions: %w", err)
	}

	return http.StatusOK, exclusions
}

func radarrDelExclusions(r *http.Request) (int, interface{}) {
	ids := mux.Vars(r)["eid"]
	exclusions := []int64{}

	for _, s := range strings.Split(ids, ",") {
		if i, err := strconv.ParseInt(s, mnd.Base10, mnd.Bits64); err == nil {
			exclusions = append(exclusions, i)
		}
	}

	err := getRadarr(r).DeleteExclusions(exclusions)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("deleting exclusions: %w", err)
	}

	return http.StatusOK, "deleted: " + strings.Join(strings.Split(ids, ","), ", ")
}

func radarrAddCustomFormat(r *http.Request) (int, interface{}) {
	var cf radarr.CustomFormat

	err := json.NewDecoder(r.Body).Decode(&cf)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("decoding payload: %w", err)
	}

	resp, err := getRadarr(r).AddCustomFormat(&cf)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("adding custom format: %w", err)
	}

	return http.StatusOK, resp
}

func radarrGetCustomFormats(r *http.Request) (int, interface{}) {
	cf, err := getRadarr(r).GetCustomFormats()
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("getting custom formats: %w", err)
	}

	return http.StatusOK, cf
}

func radarrUpdateCustomFormat(r *http.Request) (int, interface{}) {
	var cf radarr.CustomFormat
	if err := json.NewDecoder(r.Body).Decode(&cf); err != nil {
		return http.StatusBadRequest, fmt.Errorf("decoding payload: %w", err)
	}

	cfID, _ := strconv.Atoi(mux.Vars(r)["cfid"])

	output, err := getRadarr(r).UpdateCustomFormat(&cf, cfID)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("updating custom format: %w", err)
	}

	return http.StatusOK, output
}
