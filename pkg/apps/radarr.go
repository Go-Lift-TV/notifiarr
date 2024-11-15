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
	"golang.org/x/time/rate"
	"golift.io/starr"
	"golift.io/starr/debuglog"
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
	a.HandleAPIpath(starr.Radarr, "/qualityProfile/{profileID:[0-9]+}", radarrDeleteQualityProfile, "DELETE")
	a.HandleAPIpath(starr.Radarr, "/qualityProfiles/all", radarrDeleteAllQualityProfiles, "DELETE")
	a.HandleAPIpath(starr.Radarr, "/rootFolder", radarrRootFolders, "GET")
	a.HandleAPIpath(starr.Radarr, "/naming", radarrGetNaming, "GET")
	a.HandleAPIpath(starr.Radarr, "/naming", radarrUpdateNaming, "PUT")
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
	a.HandleAPIpath(starr.Radarr, "/customformats", radarrUpdateCustomFormat, "PUT")
	a.HandleAPIpath(starr.Radarr, "/customformats/{cfid:[0-9]+}", radarrUpdateCustomFormat, "PUT")
	a.HandleAPIpath(starr.Radarr, "/customformats/{cfid:[0-9]+}", radarrDeleteCustomFormat, "DELETE")
	a.HandleAPIpath(starr.Radarr, "/qualitydefinitions", radarrGetQualityDefinitions, "GET")
	a.HandleAPIpath(starr.Radarr, "/qualitydefinition", radarrUpdateQualityDefinition, "PUT")
	a.HandleAPIpath(starr.Radarr, "/customformats/all", radarrDeleteAllCustomFormats, "DELETE")
	a.HandleAPIpath(starr.Radarr, "/importlist", radarrGetImportLists, "GET")
	a.HandleAPIpath(starr.Radarr, "/importlist", radarrAddImportList, "POST")
	a.HandleAPIpath(starr.Radarr, "/importlist/{ilid:[0-9]+}", radarrUpdateImportList, "PUT")
	a.HandleAPIpath(starr.Radarr, "/command/search/{movieid:[0-9]+}", radarrTriggerSearchMovie, "GET")
	a.HandleAPIpath(starr.Radarr, "/notification", radarrGetNotifications, "GET")
	a.HandleAPIpath(starr.Radarr, "/notification", radarrUpdateNotification, "PUT")
	a.HandleAPIpath(starr.Radarr, "/notification", radarrAddNotification, "POST")
	a.HandleAPIpath(starr.Radarr, "/queue/{queueID}", radarrDeleteQueue, "DELETE")
	a.HandleAPIpath(starr.Radarr, "/delete/{movieID:[0-9]+}", radarrDeleteMovie, "POST")
	a.HandleAPIpath(starr.Radarr, "/delete/{movieFileID:[0-9]+}", radarrDeleteContent, "DELETE")
}

// RadarrConfig represents the input data for a Radarr server.
type RadarrConfig struct {
	ExtraConfig
	*starr.Config
	*radarr.Radarr `json:"-" toml:"-" xml:"-"`
	errorf         func(string, ...interface{}) `json:"-" toml:"-" xml:"-"`
}

func getRadarr(r *http.Request) *RadarrConfig {
	return r.Context().Value(starr.Radarr).(*RadarrConfig) //nolint:forcetypeassert
}

// Enabled returns true if the Radarr instance is enabled and usable.
func (r *RadarrConfig) Enabled() bool {
	return r != nil && r.Config != nil && r.URL != "" && r.APIKey != "" && r.Timeout.Duration >= 0
}

func (a *Apps) setupRadarr() error {
	for idx, app := range a.Radarr {
		if app.Config == nil || app.Config.URL == "" {
			return fmt.Errorf("%w: missing url: Radarr config %d", ErrInvalidApp, idx+1)
		} else if !strings.HasPrefix(app.Config.URL, "http://") && !strings.HasPrefix(app.Config.URL, "https://") {
			return fmt.Errorf("%w: URL must begin with http:// or https://: Radarr config %d", ErrInvalidApp, idx+1)
		}

		if a.Logger.DebugEnabled() {
			app.Config.Client = starr.ClientWithDebug(app.Timeout.Duration, app.ValidSSL, debuglog.Config{
				MaxBody: a.MaxBody,
				Debugf:  a.Debugf,
				Caller:  metricMakerCallback(string(starr.Radarr)),
				Redact:  []string{app.APIKey, app.Password, app.HTTPPass},
			})
		} else {
			app.Config.Client = starr.Client(app.Timeout.Duration, app.ValidSSL)
			app.Config.Client.Transport = NewMetricsRoundTripper(starr.Radarr.String(), app.Config.Client.Transport)
		}

		app.errorf = a.Errorf
		app.URL = strings.TrimRight(app.URL, "/")
		app.Radarr = radarr.New(app.Config)

		if app.Deletes > 0 {
			app.delLimit = rate.NewLimiter(rate.Every(1*time.Hour/time.Duration(app.Deletes)), app.Deletes)
		}
	}

	return nil
}

// @Description  Adds a new Movie to Radarr.
// @Summary      Add Radarr Movie
// @Tags         Radarr
// @Produce      json
// @Param        instance  path   int64  true  "instance ID"
// @Param        POST body radarr.AddMovieInput true "new item content"
// @Accept       json
// @Success      201  {object} apps.Respond.apiResponse{message=radarr.Movie} "created"
// @Failure      400  {object} apps.Respond.apiResponse{message=string} "bad json payload"
// @Failure      409  {object} apps.Respond.apiResponse{message=string} "item already exists"
// @Failure      422  {object} apps.Respond.apiResponse{message=string} "no item ID provided"
// @Failure      503  {object} apps.Respond.apiResponse{message=string} "instance error during check"
// @Failure      500  {object} apps.Respond.apiResponse{message=string} "instance error during add"
// @Failure      404  {object} string "bad token or api key"
// @Router       /api/radarr/{instance}/add [post]
// @Security     ApiKeyAuth
func radarrAddMovie(req *http.Request) (int, interface{}) {
	var payload radarr.AddMovieInput
	// Extract payload and check for TMDB ID.
	err := json.NewDecoder(req.Body).Decode(&payload)
	if err != nil {
		return apiError(http.StatusBadRequest, "decoding payload", err)
	} else if payload.TmdbID == 0 {
		return apiError(http.StatusUnprocessableEntity, "0", ErrNoTMDB)
	}

	// Check for existing movie.
	m, err := getRadarr(req).GetMovieContext(req.Context(), &radarr.GetMovie{TMDBID: payload.TmdbID})
	if err != nil {
		return apiError(http.StatusServiceUnavailable, "checking movie", err)
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
	movie, err := getRadarr(req).AddMovieContext(req.Context(), &payload)
	if err != nil {
		return apiError(http.StatusInternalServerError, "adding movie", err)
	}

	return http.StatusCreated, movie
}

func radarrData(movie *radarr.Movie) map[string]interface{} {
	return map[string]interface{}{
		"id":        movie.ID,
		"hasFile":   movie.HasFile,
		"monitored": movie.Monitored,
		"tags":      movie.Tags,
	}
}

// @Description  Checks if a Radarr movie already exists.
// @Summary      Check Radarr Movie Existence
// @Tags         Radarr
// @Produce      json
// @Param        instance  path   int64  true  "instance ID"
// @Param        tmdbid    path   int64  true  "TMDB ID"
// @Success      201  {object} apps.Respond.apiResponse{message=string} "movie does not exist"
// @Failure      409  {object} apps.Respond.apiResponse{message=string} "item already exists"
// @Failure      503  {object} apps.Respond.apiResponse{message=string} "instance error"
// @Failure      404  {object} string "bad token or api key"
// @Router       /api/radarr/{instance}/check/{tmdbid} [get]
// @Security     ApiKeyAuth
func radarrCheckMovie(req *http.Request) (int, interface{}) {
	tmdbID, _ := strconv.ParseInt(mux.Vars(req)["tmdbid"], mnd.Base10, mnd.Bits64)
	// Check for existing movie.
	m, err := getRadarr(req).GetMovieContext(req.Context(), &radarr.GetMovie{TMDBID: tmdbID})
	if err != nil {
		return apiError(http.StatusServiceUnavailable, "checking movie", err)
	} else if len(m) > 0 {
		return http.StatusConflict, radarrData(m[0])
	}

	return http.StatusOK, http.StatusText(http.StatusNotFound)
}

// @Description  Returns a Radarr Movie by ID.
// @Summary      Get Radarr Movie
// @Tags         Radarr
// @Produce      json
// @Param        instance  path   int64  true  "instance ID"
// @Param        movieID   path   int64  true  "Movie ID"
// @Success      201  {object} apps.Respond.apiResponse{message=radarr.Movie} "movie content"
// @Failure      503  {object} apps.Respond.apiResponse{message=string} "instance error"
// @Failure      404  {object} string "bad token or api key"
// @Router       /api/radarr/{instance}/get/{movieID} [get]
// @Security     ApiKeyAuth
func radarrGetMovie(req *http.Request) (int, interface{}) {
	movieID, _ := strconv.ParseInt(mux.Vars(req)["movieid"], mnd.Base10, mnd.Bits64)

	movie, err := getRadarr(req).GetMovieByIDContext(req.Context(), movieID)
	if err != nil {
		return apiError(http.StatusServiceUnavailable, "checking movie", err)
	}

	return http.StatusOK, movie
}

// @Description  Trigger an Internet search for a Radarr Movie.
// @Summary      Search for Radarr Movie
// @Tags         Radarr
// @Produce      json
// @Param        instance  path   int64  true  "instance ID"
// @Param        movieID   path   int64  true  "Movie ID"
// @Success      201  {object} apps.Respond.apiResponse{message=string} "search status"
// @Failure      503  {object} apps.Respond.apiResponse{message=string} "instance error"
// @Failure      404  {object} string "bad token or api key"
// @Router       /api/radarr/{instance}/command/search/{movieID} [get]
// @Security     ApiKeyAuth
func radarrTriggerSearchMovie(req *http.Request) (int, interface{}) {
	movieID, _ := strconv.ParseInt(mux.Vars(req)["movieid"], mnd.Base10, mnd.Bits64)

	output, err := getRadarr(req).SendCommandContext(req.Context(), &radarr.CommandRequest{
		Name:     "MoviesSearch",
		MovieIDs: []int64{movieID},
	})
	if err != nil {
		return apiError(http.StatusServiceUnavailable, "triggering movie search", err)
	}

	return http.StatusOK, output.Status
}

// @Description  Returns all Radarr Movies.
// @Summary      Get all Radarr Movies
// @Tags         Radarr
// @Produce      json
// @Param        instance  path   int64  true  "instance ID"
// @Success      201  {object} apps.Respond.apiResponse{message=[]radarr.Movie} "movies content"
// @Failure      503  {object} apps.Respond.apiResponse{message=string} "instance error"
// @Failure      404  {object} string "bad token or api key"
// @Router       /api/radarr/{instance}/get [get]
// @Security     ApiKeyAuth
func radarrGetAllMovies(req *http.Request) (int, interface{}) {
	movies, err := getRadarr(req).GetMovieContext(req.Context(), &radarr.GetMovie{ExcludeLocalCovers: true})
	if err != nil {
		return apiError(http.StatusServiceUnavailable, "checking movie", err)
	}

	return http.StatusOK, movies
}

// @Description  Fetches all Quality Profiles Data from Radarr.
// @Summary      Get Radarr Quality Profile Data
// @Tags         Radarr
// @Produce      json
// @Param        instance  path   int64  true  "instance ID"
// @Success      201  {object} apps.Respond.apiResponse{message=[]radarr.QualityProfile} "all profiles"
// @Failure      500  {object} apps.Respond.apiResponse{message=string} "instance error"
// @Failure      404  {object} string "bad token or api key"
// @Router       /api/radarr/{instance}/qualityProfile [get]
// @Security     ApiKeyAuth
func radarrQualityProfile(req *http.Request) (int, interface{}) {
	// Get the profiles from radarr.
	profiles, err := getRadarr(req).GetQualityProfilesContext(req.Context())
	if err != nil {
		return apiError(http.StatusInternalServerError, "getting profiles", err)
	}

	return http.StatusOK, profiles
}

// @Description  Fetches all Quality Profiles from Radarr.
// @Summary      Get Radarr Quality Profiles
// @Tags         Radarr
// @Produce      json
// @Param        instance  path   int64  true  "instance ID"
// @Success      201  {object} apps.Respond.apiResponse{message=map[int64]string} "map of ID to name"
// @Failure      500  {object} apps.Respond.apiResponse{message=string} "instance error"
// @Failure      404  {object} string "bad token or api key"
// @Router       /api/radarr/{instance}/qualityProfiles [get]
// @Security     ApiKeyAuth
func radarrQualityProfiles(req *http.Request) (int, interface{}) {
	// Get the profiles from radarr.
	profiles, err := getRadarr(req).GetQualityProfilesContext(req.Context())
	if err != nil {
		return apiError(http.StatusInternalServerError, "getting profiles", err)
	}

	// Format profile ID=>Name into a nice map.
	p := make(map[int64]string)
	for i := range profiles {
		p[profiles[i].ID] = profiles[i].Name
	}

	return http.StatusOK, p
}

// @Description  Creates a new Radarr Quality Profile.
// @Summary      Add Radarr Quality Profile
// @Tags         Radarr
// @Produce      json
// @Accept       json
// @Param        instance  path   int64  true  "instance ID"
// @Param        POST body radarr.QualityProfile true "new item content"
// @Success      200  {object} apps.Respond.apiResponse{message=int64} "new profile ID"
// @Failure      400  {object} apps.Respond.apiResponse{message=string} "json input error"
// @Failure      500  {object} apps.Respond.apiResponse{message=string} "instance error"
// @Failure      404  {object} string "bad token or api key"
// @Router       /api/radarr/{instance}/qualityProfile [post]
// @Security     ApiKeyAuth
func radarrAddQualityProfile(req *http.Request) (int, interface{}) {
	var profile radarr.QualityProfile

	// Extract payload and check for TMDB ID.
	err := json.NewDecoder(req.Body).Decode(&profile)
	if err != nil {
		return apiError(http.StatusBadRequest, "decoding payload", err)
	}

	// Get the profiles from radarr.
	id, err := getRadarr(req).AddQualityProfileContext(req.Context(), &profile)
	if err != nil {
		return apiError(http.StatusInternalServerError, "adding profile", err)
	}

	return http.StatusOK, id
}

// @Description  Updates a Radarr Quality Profile.
// @Summary      Update Radarr Quality Profile
// @Tags         Radarr
// @Produce      json
// @Accept       json
// @Param        instance  path   int64  true  "instance ID"
// @Param        profileID  path   int64  true  "profile ID to update"
// @Param        PUT body radarr.QualityProfile true "updated item content"
// @Success      200  {object} apps.Respond.apiResponse{message=string} "ok"
// @Failure      400  {object} apps.Respond.apiResponse{message=string} "json input error"
// @Failure      422  {object} apps.Respond.apiResponse{message=string} "no profile ID"
// @Failure      500  {object} apps.Respond.apiResponse{message=string} "instance error"
// @Failure      404  {object} string "bad token or api key"
// @Router       /api/radarr/{instance}/qualityProfile/{profileID} [put]
// @Security     ApiKeyAuth
func radarrUpdateQualityProfile(req *http.Request) (int, interface{}) {
	var profile radarr.QualityProfile

	// Extract payload and check for TMDB ID.
	err := json.NewDecoder(req.Body).Decode(&profile)
	if err != nil {
		return apiError(http.StatusBadRequest, "decoding payload", err)
	}

	profile.ID, _ = strconv.ParseInt(mux.Vars(req)["profileID"], mnd.Base10, mnd.Bits64)
	if profile.ID == 0 {
		return http.StatusUnprocessableEntity, ErrNonZeroID
	}

	// Get the profiles from radarr.
	_, err = getRadarr(req).UpdateQualityProfileContext(req.Context(), &profile)
	if err != nil {
		return apiError(http.StatusInternalServerError, "updating profile", err)
	}

	return http.StatusOK, "OK"
}

// @Description  Removes a Radarr Quality Profile.
// @Summary      Remove Radarr Quality Profile
// @Tags         Radarr
// @Produce      json
// @Param        instance  path   int64  true  "instance ID"
// @Param        profileID  path   int64  true  "profile ID to update"
// @Success      200  {object} apps.Respond.apiResponse{message=string} "ok"
// @Failure      400  {object} apps.Respond.apiResponse{message=string} "no profile ID"
// @Failure      500  {object} apps.Respond.apiResponse{message=string} "instance error"
// @Failure      404  {object} string "bad token or api key"
// @Router       /api/radarr/{instance}/qualityProfile/{profileID} [delete]
// @Security     ApiKeyAuth
func radarrDeleteQualityProfile(req *http.Request) (int, interface{}) {
	profileID, _ := strconv.ParseInt(mux.Vars(req)["profileID"], mnd.Base10, mnd.Bits64)
	if profileID == 0 {
		return http.StatusBadRequest, ErrNonZeroID
	}

	// Delete the profile from radarr.
	err := getRadarr(req).DeleteQualityProfileContext(req.Context(), profileID)
	if err != nil {
		return apiError(http.StatusInternalServerError, "deleting profile", err)
	}

	return http.StatusOK, "OK"
}

type deleteResponse struct {
	// How many items are found and attempted to be deleted.
	Found int `json:"found"`
	// How many items were deleted.
	Deleted int `json:"deleted"`
	// Errors returned from the delete queries.
	Errors []string `json:"errors"`
}

// @Description  Removes all Radarr Quality Profiles.
// @Summary      Remove Radarr Quality Profiles
// @Tags         Radarr
// @Produce      json
// @Param        instance  path   int64  true  "instance ID"
// @Success      200  {object} apps.Respond.apiResponse{message=apps.deleteResponse} "delete status"
// @Failure      500  {object} apps.Respond.apiResponse{message=string} "instance error getting profiles"
// @Failure      404  {object} string "bad token or api key"
// @Router       /api/radarr/{instance}/qualityProfiles/all [delete]
// @Security     ApiKeyAuth
func radarrDeleteAllQualityProfiles(req *http.Request) (int, interface{}) {
	// Get all the profiles from radarr.
	profiles, err := getRadarr(req).GetQualityProfilesContext(req.Context())
	if err != nil {
		return apiError(http.StatusInternalServerError, "getting profiles", err)
	}

	var (
		deleted int
		errs    []string
	)

	// Delete each profile from radarr.
	for _, profile := range profiles {
		if err := getRadarr(req).DeleteQualityProfileContext(req.Context(), profile.ID); err != nil {
			errs = append(errs, err.Error())
			continue
		}

		deleted++
	}

	return http.StatusOK, deleteResponse{
		Found:   len(profiles),
		Deleted: deleted,
		Errors:  errs,
	}
}

// @Description  Returns all Radarr Root Folders paths and free space.
// @Summary      Retrieve Radarr Root Folders
// @Tags         Radarr
// @Produce      json
// @Param        instance  path   int64  true  "instance ID"
// @Success      200  {object} apps.Respond.apiResponse{message=map[string]int64} "map of path->space free"
// @Failure      500  {object} apps.Respond.apiResponse{message=string} "instance error"
// @Failure      404  {object} string "bad token or api key"
// @Router       /api/radarr/{instance}/rootFolder [get]
// @Security     ApiKeyAuth
func radarrRootFolders(req *http.Request) (int, interface{}) {
	// Get folder list from Radarr.
	folders, err := getRadarr(req).GetRootFoldersContext(req.Context())
	if err != nil {
		return apiError(http.StatusInternalServerError, "getting folders", err)
	}

	// Format folder list into a nice path=>freesSpace map.
	p := make(map[string]int64)
	for i := range folders {
		p[folders[i].Path] = folders[i].FreeSpace
	}

	return http.StatusOK, p
}

// @Description  Returns Radarr movie naming conventions.
// @Summary      Retrieve Radarr Movie Naming
// @Tags         Radarr
// @Produce      json
// @Param        instance  path   int64  true  "instance ID"
// @Success      200  {object} apps.Respond.apiResponse{message=radarr.Naming} "naming conventions"
// @Failure      500  {object} apps.Respond.apiResponse{message=string} "instance error"
// @Failure      404  {object} string "bad token or api key"
// @Router       /api/radarr/{instance}/naming [get]
// @Security     ApiKeyAuth
func radarrGetNaming(req *http.Request) (int, interface{}) {
	naming, err := getRadarr(req).GetNamingContext(req.Context())
	if err != nil {
		return apiError(http.StatusInternalServerError, "getting naming", err)
	}

	return http.StatusOK, naming
}

// @Description  Updates the Radarr movie naming conventions.
// @Summary      Update Radarr Movie Naming
// @Tags         Radarr
// @Produce      json
// @Accept       json
// @Param        PUT body radarr.Naming  true  "naming conventions"
// @Success      200  {object} apps.Respond.apiResponse{message=int64} "naming ID"
// @Failure      400  {object} apps.Respond.apiResponse{message=string} "bad json input"
// @Failure      500  {object} apps.Respond.apiResponse{message=string} "instance error"
// @Failure      404  {object} string "bad token or api key"
// @Router       /api/radarr/{instance}/naming [put]
// @Security     ApiKeyAuth
func radarrUpdateNaming(req *http.Request) (int, interface{}) {
	var naming radarr.Naming

	err := json.NewDecoder(req.Body).Decode(&naming)
	if err != nil {
		return apiError(http.StatusBadRequest, "decoding payload", err)
	}

	output, err := getRadarr(req).UpdateNamingContext(req.Context(), &naming)
	if err != nil {
		return apiError(http.StatusServiceUnavailable, "updating naming", err)
	}

	return http.StatusOK, output.ID
}

// @Description  Searches all Radarr Movie Titles for the search term provided. Returns a minimal amount of data for each found item.
// @Summary      Search for Radarr Movies
// @Tags         Radarr
// @Produce      json
// @Param        query     path   string  true  "title search string"
// @Param        instance  path   int64   true  "instance ID"
// @Success      200  {object} apps.Respond.apiResponse{message=[]apps.radarrSearchMovie.movieData}  "minimal movie data"
// @Failure      503  {object} apps.Respond.apiResponse{message=string} "instance error"
// @Failure      404  {object} string "bad token or api key"
// @Router       /api/radarr/{instance}/search/{query} [get]
// @Security     ApiKeyAuth
//
//nolint:lll
func radarrSearchMovie(req *http.Request) (int, interface{}) {
	// Get all movies
	movies, err := getRadarr(req).GetMovieContext(req.Context(), &radarr.GetMovie{ExcludeLocalCovers: true})
	if err != nil {
		return apiError(http.StatusServiceUnavailable, "getting movies", err)
	}

	type movieData struct {
		ID                  int64               `json:"id"`
		Title               string              `json:"title"`
		Release             time.Time           `json:"release"`
		InCinemas           time.Time           `json:"cinemas"`
		DigitalRelease      time.Time           `json:"digital"`
		PhysicalRelease     time.Time           `json:"physical"`
		Status              string              `json:"status"`
		Exists              bool                `json:"exists"`
		Added               time.Time           `json:"added"`
		Year                int                 `json:"year"`
		Path                string              `json:"path"`
		TmdbID              int64               `json:"tmdbId"`
		QualityProfileID    int64               `json:"qualityId"`
		Monitored           bool                `json:"monitored"`
		MinimumAvailability radarr.Availability `json:"metadataId"`
	}

	query := strings.TrimSpace(strings.ToLower(mux.Vars(req)["query"])) // in
	output := make([]*movieData, 0)                                     // out

	for _, movie := range movies {
		if movieSearch(query, []string{movie.Title, movie.OriginalTitle}, movie.AlternateTitles) {
			output = append(output, &movieData{
				ID:                  movie.ID,
				Title:               movie.Title,
				InCinemas:           movie.InCinemas,
				DigitalRelease:      movie.DigitalRelease,
				PhysicalRelease:     movie.PhysicalRelease,
				Status:              movie.Status,
				Exists:              movie.HasFile,
				Added:               movie.Added,
				Year:                movie.Year,
				Path:                movie.Path,
				TmdbID:              movie.TmdbID,
				QualityProfileID:    movie.QualityProfileID,
				Monitored:           movie.Monitored,
				MinimumAvailability: movie.MinimumAvailability,
			})
		}
	}

	return http.StatusOK, output
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

// @Description  Returns all Radarr Tags.
// @Summary      Retrieve Radarr Tags
// @Tags         Radarr
// @Produce      json
// @Param        instance  path   int64  true  "instance ID"
// @Success      200  {object} apps.Respond.apiResponse{message=[]starr.Tag} "tags"
// @Failure      503  {object} apps.Respond.apiResponse{message=string} "instance error"
// @Failure      404  {object} string "bad token or api key"
// @Router       /api/radarr/{instance}/tag [get]
// @Security     ApiKeyAuth
func radarrGetTags(req *http.Request) (int, interface{}) {
	tags, err := getRadarr(req).GetTagsContext(req.Context())
	if err != nil {
		return apiError(http.StatusServiceUnavailable, "getting tags", err)
	}

	return http.StatusOK, tags
}

// @Description  Updates the label for a an existing Radarr tag.
// @Summary      Update Radarr Tag Label
// @Tags         Radarr
// @Produce      json
// @Param        instance  path   int64  true  "instance ID"
// @Param        tagID     path   int64  true  "tag ID to update"
// @Param        label     path   string  true  "new label"
// @Success      200  {object} apps.Respond.apiResponse{message=int64}  "tag ID"
// @Failure      503  {object} apps.Respond.apiResponse{message=string} "instance error"
// @Failure      404  {object} string "bad token or api key"
// @Router       /api/radarr/{instance}/tag/{tagID}/{label} [put]
// @Security     ApiKeyAuth
func radarrUpdateTag(req *http.Request) (int, interface{}) {
	id, _ := strconv.Atoi(mux.Vars(req)["tid"])

	tag, err := getRadarr(req).UpdateTagContext(req.Context(), &starr.Tag{ID: id, Label: mux.Vars(req)["label"]})
	if err != nil {
		return apiError(http.StatusServiceUnavailable, "updating tag", err)
	}

	return http.StatusOK, tag.ID
}

// @Description  Creates a new Radarr tag with the provided label.
// @Summary      Create Radarr Tag
// @Tags         Radarr
// @Produce      json
// @Param        instance  path   int64  true  "instance ID"
// @Param        label     path   string true  "new tag's label"
// @Success      200  {object} apps.Respond.apiResponse{message=int64}  "tag ID"
// @Failure      503  {object} apps.Respond.apiResponse{message=string} "instance error"
// @Failure      404  {object} string "bad token or api key"
// @Router       /api/radarr/{instance}/tag/{label} [put]
// @Security     ApiKeyAuth
func radarrSetTag(req *http.Request) (int, interface{}) {
	tag, err := getRadarr(req).AddTagContext(req.Context(), &starr.Tag{Label: mux.Vars(req)["label"]})
	if err != nil {
		return apiError(http.StatusServiceUnavailable, "setting tag", err)
	}

	return http.StatusOK, tag.ID
}

// @Description  Updates a Movie in Radarr.
// @Summary      Update Radarr Movie
// @Tags         Radarr
// @Produce      json
// @Accept       json
// @Param        instance  path  int64  true  "instance ID"
// @Param        moveFiles query int64  true  "move files? true/false"
// @Param        PUT body radarr.Movie  true  "movie content"
// @Success      200  {object} apps.Respond.apiResponse{message=string} "ok"
// @Failure      400  {object} apps.Respond.apiResponse{message=string} "bad json input"
// @Failure      503  {object} apps.Respond.apiResponse{message=string} "instance error"
// @Failure      404  {object} string "bad token or api key"
// @Router       /api/radarr/{instance}/update [put]
// @Security     ApiKeyAuth
func radarrUpdateMovie(req *http.Request) (int, interface{}) {
	var movie radarr.Movie
	// Extract payload and check for TMDB ID.
	err := json.NewDecoder(req.Body).Decode(&movie)
	if err != nil {
		return apiError(http.StatusBadRequest, "decoding payload", err)
	}

	moveFiles := mux.Vars(req)["moveFiles"] == strconv.FormatBool(true)

	// Check for existing movie.
	_, err = getRadarr(req).UpdateMovieContext(req.Context(), movie.ID, &movie, moveFiles)
	if err != nil {
		return apiError(http.StatusServiceUnavailable, "updating movie", err)
	}

	return http.StatusOK, "radarr seems to have worked"
}

// @Description  Creates a new exclusion in Radarr.
// @Summary      Create Radarr Exclusion
// @Tags         Radarr
// @Produce      json
// @Accept       json
// @Param        instance  path   int64  true  "instance ID"
// @Param        POST body []radarr.Exclusion  true  "movie content"
// @Success      200  {object} apps.Respond.apiResponse{message=string}  "ok"
// @Failure      400  {object} apps.Respond.apiResponse{message=string} "invalid json provided"
// @Failure      500  {object} apps.Respond.apiResponse{message=string} "instance error"
// @Failure      404  {object} string "bad token or api key"
// @Router       /api/radarr/{instance}/exclusions [post]
// @Security     ApiKeyAuth
func radarrAddExclusions(req *http.Request) (int, interface{}) {
	var exclusions []*radarr.Exclusion

	err := json.NewDecoder(req.Body).Decode(&exclusions)
	if err != nil {
		return apiError(http.StatusBadRequest, "decoding payload", err)
	}

	// Get the profiles from radarr.
	err = getRadarr(req).AddExclusionsContext(req.Context(), exclusions)
	if err != nil {
		return apiError(http.StatusInternalServerError, "adding exclusions", err)
	}

	return http.StatusOK, "added " + strconv.Itoa(len(exclusions)) + " exclusions"
}

// @Description  Retrieve all Radarr Exclusions.
// @Summary      Get Radarr Exclusions
// @Tags         Radarr
// @Produce      json
// @Param        instance  path   int64  true  "instance ID"
// @Success      200  {object} apps.Respond.apiResponse{message=[]radarr.Exclusion}  "exclusion list"
// @Failure      500  {object} apps.Respond.apiResponse{message=string} "instance error"
// @Failure      404  {object} string "bad token or api key"
// @Router       /api/radarr/{instance}/exclusions [get]
// @Security     ApiKeyAuth
func radarrGetExclusions(req *http.Request) (int, interface{}) {
	exclusions, err := getRadarr(req).GetExclusionsContext(req.Context())
	if err != nil {
		return apiError(http.StatusInternalServerError, "getting exclusions", err)
	}

	return http.StatusOK, exclusions
}

// @Description  Delete Exclusion(s) from Radarr.
// @Summary      Remove Radarr Exclusion(s)
// @Tags         Radarr
// @Produce      json
// @Param        instance  path   int64  true  "instance ID"
// @Param        exclusionIDs  path   []int64  true  "exclusion IDs to delete, comma separated"
// @Success      200  {object} apps.Respond.apiResponse{message=string}  "ok"
// @Failure      500  {object} apps.Respond.apiResponse{message=string} "instance error"
// @Failure      404  {object} string "bad token or api key"
// @Router       /api/radarr/{instance}/exclusions/{exclusionIDs} [delete]
// @Security     ApiKeyAuth
func radarrDelExclusions(req *http.Request) (int, interface{}) {
	ids := mux.Vars(req)["eid"]
	exclusions := []int64{}

	for _, s := range strings.Split(ids, ",") {
		if i, err := strconv.ParseInt(s, mnd.Base10, mnd.Bits64); err == nil {
			exclusions = append(exclusions, i)
		}
	}

	err := getRadarr(req).DeleteExclusionsContext(req.Context(), exclusions)
	if err != nil {
		return apiError(http.StatusInternalServerError, "deleting exclusions", err)
	}

	return http.StatusOK, mnd.Deleted + strings.Join(strings.Split(ids, ","), ", ")
}

// @Description  Creates a new Custom Format in Radarr.
// @Summary      Create Radarr Custom Format
// @Tags         Radarr
// @Produce      json
// @Accept       json
// @Param        instance  path   int64  true  "instance ID"
// @Param        POST body radarr.CustomFormatInput  true  "New Custom Format content"
// @Success      200  {object} apps.Respond.apiResponse{message=radarr.CustomFormatOutput}  "custom format"
// @Failure      400  {object} apps.Respond.apiResponse{message=string} "invalid json provided"
// @Failure      500  {object} apps.Respond.apiResponse{message=string} "instance error"
// @Failure      404  {object} string "bad token or api key"
// @Router       /api/radarr/{instance}/customformats [post]
// @Security     ApiKeyAuth
func radarrAddCustomFormat(req *http.Request) (int, interface{}) {
	var cusform radarr.CustomFormatInput

	err := json.NewDecoder(req.Body).Decode(&cusform)
	if err != nil {
		return apiError(http.StatusBadRequest, "decoding payload", err)
	}

	resp, err := getRadarr(req).AddCustomFormatContext(req.Context(), &cusform)
	if err != nil {
		return apiError(http.StatusInternalServerError, "adding custom format", err)
	}

	return http.StatusOK, resp
}

// @Description  Returns all Custom Formats Data from Radarr.
// @Summary      Get Radarr Custom Formats Data
// @Tags         Radarr
// @Produce      json
// @Param        instance  path   int64  true  "instance ID"
// @Success      200  {object} apps.Respond.apiResponse{message=[]radarr.CustomFormatOutput}  "custom formats"
// @Failure      500  {object} apps.Respond.apiResponse{message=string} "instance error"
// @Failure      404  {object} string "bad token or api key"
// @Router       /api/radarr/{instance}/customformats [get]
// @Security     ApiKeyAuth
func radarrGetCustomFormats(req *http.Request) (int, interface{}) {
	cusform, err := getRadarr(req).GetCustomFormatsContext(req.Context())
	if err != nil {
		return apiError(http.StatusInternalServerError, "getting custom formats", err)
	}

	return http.StatusOK, cusform
}

// @Description  Updates a Custom Format in Radarr.
// @Summary      Update Radarr Custom Format
// @Tags         Radarr
// @Produce      json
// @Accept       json
// @Param        instance  path   int64  true  "instance ID"
// @Param        PUT body radarr.CustomFormatInput  true  "Updated Custom Format content"
// @Success      200  {object} apps.Respond.apiResponse{message=radarr.CustomFormatOutput}  "custom format"
// @Failure      400  {object} apps.Respond.apiResponse{message=string} "invalid json provided"
// @Failure      500  {object} apps.Respond.apiResponse{message=string} "instance error"
// @Failure      404  {object} string "bad token or api key"
// @Router       /api/radarr/{instance}/customformats/{formatID} [put]
// @Security     ApiKeyAuth
func radarrUpdateCustomFormat(req *http.Request) (int, interface{}) {
	var cusform radarr.CustomFormatInput
	if err := json.NewDecoder(req.Body).Decode(&cusform); err != nil {
		return apiError(http.StatusBadRequest, "decoding payload", err)
	}

	output, err := getRadarr(req).UpdateCustomFormatContext(req.Context(), &cusform)
	if err != nil {
		return apiError(http.StatusInternalServerError, "updating custom format", err)
	}

	return http.StatusOK, output
}

// @Description  Delete a Custom Format from Radarr.
// @Summary      Delete Radarr Custom Format
// @Tags         Radarr
// @Produce      json
// @Param        instance  path   int64  true  "instance ID"
// @Param        formatID  path   int64  true  "Custom Format ID"
// @Success      200  {object} apps.Respond.apiResponse{message=string}  "ok"
// @Failure      500  {object} apps.Respond.apiResponse{message=string} "instance error"
// @Failure      404  {object} string "bad token or api key"
// @Router       /api/radarr/{instance}/customformats/{formatID} [delete]
// @Security     ApiKeyAuth
func radarrDeleteCustomFormat(req *http.Request) (int, interface{}) {
	cfID, _ := strconv.ParseInt(mux.Vars(req)["cfid"], mnd.Base10, mnd.Bits64)

	err := getRadarr(req).DeleteCustomFormatContext(req.Context(), cfID)
	if err != nil {
		return apiError(http.StatusInternalServerError, "deleting custom format", err)
	}

	return http.StatusOK, "OK"
}

// @Description  Delete all Custom Formats from Radarr.
// @Summary      Delete all Radarr Custom Formats
// @Tags         Radarr
// @Produce      json
// @Param        instance  path   int64  true  "instance ID"
// @Success      200  {object} apps.Respond.apiResponse{message=apps.deleteResponse}  "item delete counters"
// @Failure      500  {object} apps.Respond.apiResponse{message=string} "instance error"
// @Failure      404  {object} string "bad token or api key"
// @Router       /api/radarr/{instance}/customformats/all [delete]
// @Security     ApiKeyAuth
func radarrDeleteAllCustomFormats(req *http.Request) (int, interface{}) {
	formats, err := getRadarr(req).GetCustomFormatsContext(req.Context())
	if err != nil {
		return apiError(http.StatusInternalServerError, "getting custom formats", err)
	}

	var (
		deleted int
		errs    []string
	)

	for _, format := range formats {
		err := getRadarr(req).DeleteCustomFormatContext(req.Context(), format.ID)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}

		deleted++
	}

	return http.StatusOK, &deleteResponse{
		Found:   len(formats),
		Deleted: deleted,
		Errors:  errs,
	}
}

// @Description  Returns all Import Lists from Radarr.
// @Summary      Get Radarr Import Lists
// @Tags         Radarr
// @Produce      json
// @Param        instance  path   int64  true  "instance ID"
// @Success      200  {object} apps.Respond.apiResponse{message=[]radarr.ImportListOutput}  "list of import lists"
// @Failure      500  {object} apps.Respond.apiResponse{message=string} "instance error"
// @Failure      404  {object} string "bad token or api key"
// @Router       /api/radarr/{instance}/importlist [get]
// @Security     ApiKeyAuth
func radarrGetImportLists(req *http.Request) (int, interface{}) {
	ilist, err := getRadarr(req).GetImportListsContext(req.Context())
	if err != nil {
		return apiError(http.StatusInternalServerError, "getting import lists", err)
	}

	return http.StatusOK, ilist
}

// @Description  Updates an Import List in Radarr.
// @Summary      Update Radarr Import List
// @Tags         Radarr
// @Produce      json
// @Accept       json
// @Param        instance  path   int64  true  "instance ID"
// @Param        listID  path   int64  true  "Import List ID"
// @Param        PUT body radarr.ImportListInput  true  "Updated Import List Content"
// @Success      200  {object} apps.Respond.apiResponse{message=radarr.ImportListOutput}  "import list returns"
// @Failure      400  {object} apps.Respond.apiResponse{message=string} "invalid json provided"
// @Failure      500  {object} apps.Respond.apiResponse{message=string} "instance error"
// @Failure      404  {object} string "bad token or api key"
// @Router       /api/radarr/{instance}/importlist/{listID} [put]
// @Security     ApiKeyAuth
func radarrUpdateImportList(req *http.Request) (int, interface{}) {
	var ilist radarr.ImportListInput
	if err := json.NewDecoder(req.Body).Decode(&ilist); err != nil {
		return apiError(http.StatusBadRequest, "decoding payload", err)
	}

	ilist.ID, _ = strconv.ParseInt(mux.Vars(req)["ilid"], mnd.Base10, mnd.Bits64)

	output, err := getRadarr(req).UpdateImportListContext(req.Context(), &ilist, false)
	if err != nil {
		return apiError(http.StatusInternalServerError, "updating import list", err)
	}

	return http.StatusOK, output
}

// @Description  Creates a new Import List in Radarr.
// @Summary      Create Radarr Import List
// @Tags         Radarr
// @Produce      json
// @Accept       json
// @Param        instance  path   int64  true  "instance ID"
// @Param        POST body radarr.ImportListInput  true  "New Import List"
// @Success      200  {object} apps.Respond.apiResponse{message=radarr.ImportListOutput}  "import list returns"
// @Failure      400  {object} apps.Respond.apiResponse{message=string} "invalid json provided"
// @Failure      500  {object} apps.Respond.apiResponse{message=string} "instance error"
// @Failure      404  {object} string "bad token or api key"
// @Router       /api/radarr/{instance}/importlist [post]
// @Security     ApiKeyAuth
func radarrAddImportList(req *http.Request) (int, interface{}) {
	var ilist radarr.ImportListInput
	if err := json.NewDecoder(req.Body).Decode(&ilist); err != nil {
		return apiError(http.StatusBadRequest, "decoding payload", err)
	}

	output, err := getRadarr(req).AddImportListContext(req.Context(), &ilist)
	if err != nil {
		return apiError(http.StatusInternalServerError, "creating import list", err)
	}

	return http.StatusOK, output
}

// @Description  Returns all Quality Definitions from Radarr.
// @Summary      Get Radarr Quality Definitions
// @Tags         Radarr
// @Produce      json
// @Param        instance  path   int64  true  "instance ID"
// @Success      200  {object} apps.Respond.apiResponse{message=[]radarr.QualityDefinition}  "quality definitions list"
// @Failure      500  {object} apps.Respond.apiResponse{message=string} "instance error"
// @Failure      404  {object} string "bad token or api key"
// @Router       /api/radarr/{instance}/qualitydefinition [get]
// @Security     ApiKeyAuth
func radarrGetQualityDefinitions(req *http.Request) (int, interface{}) {
	output, err := getRadarr(req).GetQualityDefinitionsContext(req.Context())
	if err != nil {
		return apiError(http.StatusInternalServerError, "getting quality definitions", err)
	}

	return http.StatusOK, output
}

// @Description  Updates all Quality Definitions in Radarr.
// @Summary      Update Radarr Quality Definitions
// @Tags         Radarr
// @Produce      json
// @Accept       json
// @Param        instance  path   int64  true  "instance ID"
// @Param        PUT body []radarr.QualityDefinition  true  "Updated quality definitions"
// @Success      200  {object} apps.Respond.apiResponse{message=[]radarr.QualityDefinition}  "quality definitions return"
// @Failure      400  {object} apps.Respond.apiResponse{message=string} "invalid json provided"
// @Failure      500  {object} apps.Respond.apiResponse{message=string} "instance error"
// @Failure      404  {object} string "bad token or api key"
// @Router       /api/radarr/{instance}/qualitydefinition [put]
// @Security     ApiKeyAuth
//
//nolint:lll
func radarrUpdateQualityDefinition(req *http.Request) (int, interface{}) {
	var input []*radarr.QualityDefinition
	if err := json.NewDecoder(req.Body).Decode(&input); err != nil {
		return apiError(http.StatusBadRequest, "decoding payload", err)
	}

	output, err := getRadarr(req).UpdateQualityDefinitionsContext(req.Context(), input)
	if err != nil {
		return apiError(http.StatusInternalServerError, "updating quality definition", err)
	}

	return http.StatusOK, output
}

// @Description  Returns Radarr Notifications with a name that matches 'notifiar'.
// @Summary      Retrieve Radarr Notifications
// @Tags         Radarr
// @Produce      json
// @Param        instance  path   int64  true  "instance ID"
// @Success      200  {object} apps.Respond.apiResponse{message=[]radarr.NotificationOutput} "notifications"
// @Failure      503  {object} apps.Respond.apiResponse{message=string} "instance error"
// @Failure      404  {object} string "bad token or api key"
// @Router       /api/radarr/{instance}/notifications [get]
// @Security     ApiKeyAuth
func radarrGetNotifications(req *http.Request) (int, interface{}) {
	notifs, err := getRadarr(req).GetNotificationsContext(req.Context())
	if err != nil {
		return apiError(http.StatusServiceUnavailable, "getting notifications", err)
	}

	output := []*radarr.NotificationOutput{}

	for _, notif := range notifs {
		if strings.Contains(strings.ToLower(notif.Name), "notifiar") {
			output = append(output, notif)
		}
	}

	return http.StatusOK, output
}

// @Description  Updates a Notification in Radarr.
// @Summary      Update Radarr Notification
// @Tags         Radarr
// @Produce      json
// @Accept       json
// @Param        instance  path   int64  true  "instance ID"
// @Param        PUT body radarr.NotificationInput  true  "notification content"
// @Success      200  {object} apps.Respond.apiResponse{message=string} "ok"
// @Failure      400  {object} apps.Respond.apiResponse{message=string} "bad json input"
// @Failure      503  {object} apps.Respond.apiResponse{message=string} "instance error"
// @Failure      404  {object} string "bad token or api key"
// @Router       /api/radarr/{instance}/notification [put]
// @Security     ApiKeyAuth
func radarrUpdateNotification(req *http.Request) (int, interface{}) {
	var notif radarr.NotificationInput

	err := json.NewDecoder(req.Body).Decode(&notif)
	if err != nil {
		return apiError(http.StatusBadRequest, "decoding payload", err)
	}

	_, err = getRadarr(req).UpdateNotificationContext(req.Context(), &notif)
	if err != nil {
		return apiError(http.StatusServiceUnavailable, "updating notification", err)
	}

	return http.StatusOK, mnd.Success
}

// @Description  Creates a new Radarr Notification.
// @Summary      Add Radarr Notification
// @Tags         Radarr
// @Produce      json
// @Accept       json
// @Param        instance  path   int64  true  "instance ID"
// @Param        POST body radarr.NotificationInput true "new item content"
// @Success      200  {object} apps.Respond.apiResponse{message=int64} "new notification ID"
// @Failure      400  {object} apps.Respond.apiResponse{message=string} "json input error"
// @Failure      503  {object} apps.Respond.apiResponse{message=string} "instance error"
// @Failure      404  {object} string "bad token or api key"
// @Router       /api/radarr/{instance}/notification [post]
// @Security     ApiKeyAuth
func radarrAddNotification(req *http.Request) (int, interface{}) {
	var notif radarr.NotificationInput

	err := json.NewDecoder(req.Body).Decode(&notif)
	if err != nil {
		return apiError(http.StatusBadRequest, "decoding payload", err)
	}

	id, err := getRadarr(req).AddNotificationContext(req.Context(), &notif)
	if err != nil {
		return apiError(http.StatusServiceUnavailable, "adding notification", err)
	}

	return http.StatusOK, id
}

// @Description  Delete items from the activity queue.
// @Summary      Delete Queue Items
// @Tags         Radarr
// @Produce      json
// @Param        instance         path    int64  true  "instance ID"
// @Param        queueID          path    int64  true  "queue ID to delete"
// @Param        removeFromClient query   bool  false  "remove download from download client?"
// @Param        blocklist        query   bool  false  "add item to blocklist?"
// @Param        skipRedownload   query   bool  false  "skip downloading this again?"
// @Param        changeCategory   query   bool  false  "tell download client to change categories?"
// @Success      200  {object} apps.Respond.apiResponse{message=string}  "ok"
// @Failure      500  {object} apps.Respond.apiResponse{message=string} "instance error"
// @Failure      404  {object} string "bad token or api key"
// @Failure      423  {object} string "rate limit reached"
// @Router       /api/radarr/{instance}/queue/{queueID} [delete]
// @Security     ApiKeyAuth
func radarrDeleteQueue(req *http.Request) (int, interface{}) {
	idString := mux.Vars(req)["queueID"]
	queueID, _ := strconv.ParseInt(idString, mnd.Base10, mnd.Bits64)
	removeFromClient := req.URL.Query().Get("removeFromClient") == mnd.True
	opts := &starr.QueueDeleteOpts{
		RemoveFromClient: &removeFromClient,
		BlockList:        req.URL.Query().Get("blocklist") == mnd.True,
		SkipRedownload:   req.URL.Query().Get("skipRedownload") == mnd.True,
		ChangeCategory:   req.URL.Query().Get("changeCategory") == mnd.True,
	}

	err := getRadarr(req).DeleteQueueContext(req.Context(), queueID, opts)
	if err != nil {
		return apiError(http.StatusInternalServerError, "deleting queue", err)
	}

	return http.StatusOK, mnd.Deleted + idString
}

// @Description  Delete Movies from Radarr.
// @Summary      Remove Radarr Movies
// @Tags         Radarr
// @Produce      json
// @Param        instance  path   int64  true  "instance ID"
// @Param        movieID  path   int64  true  "movie ID to delete"
// @Success      200  {object} apps.Respond.apiResponse{message=string}  "ok"
// @Failure      500  {object} apps.Respond.apiResponse{message=string} "instance error"
// @Failure      404  {object} string "bad token or api key"
// @Failure      423  {object} string "rate limit reached"
// @Router       /api/radarr/{instance}/delete/{movieID} [post]
// @Security     ApiKeyAuth
func radarrDeleteMovie(req *http.Request) (int, interface{}) {
	idString := mux.Vars(req)["movieID"]
	movieID, _ := strconv.ParseInt(idString, mnd.Base10, mnd.Bits64)
	deleteFiles := req.PostFormValue("deleteFiles")
	addExclusion := req.PostFormValue("addImportExclusion")

	if !getRadarr(req).DelOK() {
		return http.StatusLocked, ErrRateLimit
	}

	err := getRadarr(req).DeleteMovieContext(req.Context(), movieID, deleteFiles == "true", addExclusion == "true")
	if err != nil {
		return apiError(http.StatusInternalServerError, "deleting movie", err)
	}

	return http.StatusOK, mnd.Deleted + idString
}

// @Description  Delete Movie files from Radarr without deleting the movie.
// @Summary      Remove Radarr movie files
// @Tags         Radarr
// @Produce      json
// @Param        instance  path   int64  true  "instance ID"
// @Param        movieFileID  path   int64  true  "movie file ID to delete"
// @Success      200  {object} apps.Respond.apiResponse{message=string}  "ok"
// @Failure      500  {object} apps.Respond.apiResponse{message=string} "instance error"
// @Failure      404  {object} string "bad token or api key"
// @Failure      423  {object} string "rate limit reached"
// @Router       /api/radarr/{instance}/delete/{movieFileID} [delete]
// @Security     ApiKeyAuth
func radarrDeleteContent(req *http.Request) (int, interface{}) {
	idString := mux.Vars(req)["movieFileID"]
	movieFileID, _ := strconv.ParseInt(idString, mnd.Base10, mnd.Bits64)

	if !getRadarr(req).DelOK() {
		return http.StatusLocked, ErrRateLimit
	}

	err := getRadarr(req).DeleteMovieFilesContext(req.Context(), movieFileID)
	if err != nil {
		return apiError(http.StatusInternalServerError, "deleting movie file", err)
	}

	return http.StatusOK, mnd.Deleted + idString
}
