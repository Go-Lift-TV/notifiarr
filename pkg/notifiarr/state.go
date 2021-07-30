package notifiarr

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Notifiarr/notifiarr/pkg/apps"
	"golift.io/starr/radarr"
)

/* This file sends state of affairs to notifiarr.com */
// That is, it collects library data and downloader data.

// How many "upcoming" or "newest" items to send.
const (
	showNext   = 10
	showLatest = 5
)

// Sortable holds data about any Starr item. Kind of a generic data store.
type Sortable struct {
	id      int64
	Name    string    `json:"name"`
	Sub     string    `json:"subName,omitempty"`
	Date    time.Time `json:"date"`
	Season  int64     `json:"season,omitempty"`
	Episode int64     `json:"episode,omitempty"`
}

type SortableList []*Sortable

// State is partially filled out once for each app instance.
type State struct {
	// Shared
	Error    string        `json:"error"`
	Instance int           `json:"instance"`
	Missing  int64         `json:"missing,omitempty"`
	Size     int64         `json:"size"`
	Percent  float64       `json:"percent,omitempty"`
	Upcoming int64         `json:"upcoming,omitempty"`
	Next     SortableList  `json:"next,omitempty"`
	Latest   SortableList  `json:"latest,omitempty"`
	Elapsed  time.Duration `json:"elapsed"` // How long it took.
	Name     string        `json:"name"`
	// Radarr
	Movies int64 `json:"movies,omitempty"`
	// Sonarr
	Shows    int64 `json:"shows,omitempty"`
	Episodes int64 `json:"episodes,omitempty"`
	// Readarr
	Authors  int   `json:"authors,omitempty"`
	Books    int64 `json:"books,omitempty"`
	Editions int   `json:"editions,omitempty"`
	// Lidarr
	Artists int   `json:"artists,omitempty"`
	Albums  int64 `json:"albums,omitempty"`
	Tracks  int64 `json:"tracks,omitempty"`
	// Downloader
	Downloads   int   `json:"downloads,omitempty"`
	Uploaded    int64 `json:"uploaded,omitempty"`
	Incomplete  int64 `json:"incomplete,omitempty"`
	Downloaded  int64 `json:"downloaded,omitempty"`
	Uploading   int64 `json:"uploading,omitempty"`
	Downloading int64 `json:"downloading,omitempty"`
	Seeding     int64 `json:"seeding,omitempty"`
	Paused      int64 `json:"paused,omitempty"`
	Errors      int64 `json:"errors,omitempty"`
}

type States struct {
	Lidarr  []*State `json:"lidarr"`
	Radarr  []*State `json:"radarr"`
	Readarr []*State `json:"readarr"`
	Sonarr  []*State `json:"sonarr"`
	Qbit    []*State `json:"qbit"`
	Deluge  []*State `json:"deluge"`
}

func (c *Config) GetState() {
	c.stateNow <- struct{}{}
}

func (c *Config) getState() {
	start := time.Now()
	states := c.getStates()
	apps := time.Since(start).Round(time.Millisecond)

	//nolint:bodyclose // already closed
	switch resp, body, err := c.SendData(c.BaseURL+DashRoute, states, true); {
	case err != nil:
		c.Errorf("Sending Dashboard State Data (apps:%s total:%s): %v",
			apps, time.Since(start).Round(time.Millisecond), err)
	case resp.StatusCode != http.StatusOK:
		c.Errorf("Sending Dashboard State Data (apps:%s total:%s): %v: %s",
			apps, time.Since(start).Round(time.Millisecond), ErrNon200, string(body))
	default:
		c.Debugf("Sent Dashboard State Data! Elapsed: apps:%s total:%s",
			apps, time.Since(start).Round(time.Millisecond))
	}
}

// getStates fires a routine for each app type and tries to get a lot of data fast!
func (c *Config) getStates() *States {
	s := &States{}

	var wg sync.WaitGroup

	wg.Add(6) //nolint:gomnd // we are polling 6 apps.

	go func() {
		s.Deluge = c.getDelugeStates()
		wg.Done() //nolint:wsl
	}()
	go func() {
		s.Lidarr = c.getLidarrStates()
		wg.Done() //nolint:wsl
	}()
	go func() {
		s.Qbit = c.getQbitStates()
		wg.Done() //nolint:wsl
	}()
	go func() {
		s.Radarr = c.getRadarrStates()
		wg.Done() //nolint:wsl
	}()
	go func() {
		s.Readarr = c.getReadarrStates()
		wg.Done() //nolint:wsl
	}()
	go func() {
		s.Sonarr = c.getSonarrStates()
		wg.Done() //nolint:wsl
	}()
	wg.Wait()

	return s
}

func (c *Config) getDelugeStates() []*State {
	states := []*State{}

	for instance, d := range c.Apps.Deluge {
		if d.Deluge.URL == "" {
			continue
		}

		c.Debugf("Getting Deluge State: %d:%s", instance+1, d.Deluge.URL)

		state, err := c.getDelugeState(instance+1, d)
		if err != nil {
			state.Error = err.Error()
			c.Errorf("Getting Deluge Data from %d:%s: %v", instance+1, d.Deluge.URL, err)
		}

		states = append(states, state)
	}

	return states
}

func (c *Config) getLidarrStates() []*State {
	states := []*State{}

	for instance, r := range c.Apps.Lidarr {
		if r.URL == "" {
			continue
		}

		c.Debugf("Getting Lidarr State: %d:%s", instance+1, r.URL)

		state, err := c.getLidarrState(instance+1, r)
		if err != nil {
			state.Error = err.Error()
			c.Errorf("Getting Lidarr Queue from %d:%s: %v", instance+1, r.URL, err)
		}

		states = append(states, state)
	}

	return states
}

func (c *Config) getRadarrStates() []*State {
	states := []*State{}

	for instance, r := range c.Apps.Radarr {
		if r.URL == "" {
			continue
		}

		c.Debugf("Getting Radarr State: %d:%s", instance+1, r.URL)

		state, err := c.getRadarrState(instance+1, r)
		if err != nil {
			state.Error = err.Error()
			c.Errorf("Getting Radarr Queue from %d:%s: %v", instance+1, r.URL, err)
		}

		states = append(states, state)
	}

	return states
}

func (c *Config) getReadarrStates() []*State {
	states := []*State{}

	for instance, r := range c.Apps.Readarr {
		if r.URL == "" {
			continue
		}

		c.Debugf("Getting Readarr State: %d:%s", instance+1, r.URL)

		state, err := c.getReadarrState(instance+1, r)
		if err != nil {
			state.Error = err.Error()
			c.Errorf("Getting Readarr Queue from %d:%s: %v", instance+1, r.URL, err)
		}

		states = append(states, state)
	}

	return states
}

func (c *Config) getQbitStates() []*State {
	states := []*State{}

	for instance, q := range c.Apps.Qbit {
		if q.URL == "" {
			continue
		}

		c.Debugf("Getting Qbit State: %d:%s", instance+1, q.URL)

		state, err := c.getQbitState(instance+1, q)
		if err != nil {
			state.Error = err.Error()
			c.Errorf("Getting Qbit Data from %d:%s: %v", instance+1, q.URL, err)
		}

		states = append(states, state)
	}

	return states
}

func (c *Config) getSonarrStates() []*State {
	states := []*State{}

	for instance, s := range c.Apps.Sonarr {
		if s.URL == "" {
			continue
		}

		c.Debugf("Getting Sonarr State: %d:%s", instance+1, s.URL)

		state, err := c.getSonarrState(instance+1, s)
		if err != nil {
			state.Error = err.Error()
			c.Errorf("Getting Sonarr Queue from %d:%s: %v", instance+1, s.URL, err)
		}

		states = append(states, state)
	}

	return states
}

func (c *Config) getDelugeState(instance int, d *apps.DelugeConfig) (*State, error) {
	state := &State{Instance: instance, Name: d.Name}
	start := time.Now()

	xfers, err := d.GetXfersCompat()
	state.Elapsed = time.Since(start)

	if err != nil {
		return state, fmt.Errorf("getting transfers from instance %d: %w", instance, err)
	}

	for _, xfer := range xfers {
		state.Size += int64(xfer.TotalSize)
		state.Uploaded += int64(xfer.TotalUploaded)
		state.Downloaded += int64(xfer.AllTimeDownload)
		state.Downloads++

		if xfer.UploadPayloadRate > 0 {
			state.Uploading++
		}

		if xfer.DownloadPayloadRate > 0 {
			state.Downloading++
		}

		if !xfer.IsFinished {
			state.Incomplete++
		}

		if xfer.IsSeed {
			state.Seeding++
		}

		if xfer.Paused {
			state.Paused++
		}

		if xfer.Message != "OK" {
			state.Errors++
		}
	}

	return state, nil
}

func (c *Config) getLidarrState(instance int, l *apps.LidarrConfig) (*State, error) {
	state := &State{Instance: instance, Next: []*Sortable{}, Name: l.Name}
	start := time.Now()

	albums, err := l.GetAlbum("") // all albums
	state.Elapsed = time.Since(start)

	if err != nil {
		return state, fmt.Errorf("getting albums from instance %d: %w", instance, err)
	}

	artistIDs := make(map[int64]struct{})

	for _, album := range albums {
		have := false
		state.Albums++

		if album.Statistics != nil {
			artistIDs[album.ArtistID] = struct{}{}
			state.Percent += album.Statistics.PercentOfTracks
			state.Size += int64(album.Statistics.SizeOnDisk)
			state.Tracks += int64(album.Statistics.TotalTrackCount)
			state.Missing += int64(album.Statistics.TrackCount - album.Statistics.TrackFileCount)
			have = album.Statistics.TrackCount-album.Statistics.TrackFileCount < 1
		}

		if album.ReleaseDate.After(time.Now()) && album.Monitored && !have {
			state.Next = append(state.Next, &Sortable{
				id:   album.ID,
				Name: album.Title,
				Date: album.ReleaseDate,
				Sub:  album.Artist.ArtistName,
			})
		}
	}

	state.Percent /= float64(state.Tracks)
	state.Artists = len(artistIDs)
	sort.Sort(dateSorter(state.Next))
	state.Next.Shrink(showNext)

	if state.Latest, err = c.getLidarrHistory(l); err != nil {
		return state, fmt.Errorf("instance %d: %w", instance, err)
	}

	return state, nil
}

// getLidarrHistory is not done.
func (c *Config) getLidarrHistory(l *apps.LidarrConfig) ([]*Sortable, error) {
	history, err := l.GetHistory(showLatest * 40) //nolint:gomnd
	if err != nil {
		return nil, fmt.Errorf("getting history: %w", err)
	}

	table := []*Sortable{}

	for _, rec := range history.Records {
		if len(table) >= showLatest {
			break
		} else if rec.EventType != "downloadImported" {
			continue
		}

		// An error here gets swallowed.
		if album, err := l.GetAlbumByID(rec.AlbumID); err == nil {
			table = append(table, &Sortable{
				Name: album.Title,
				Sub:  album.Artist.ArtistName,
				Date: rec.Date,
			})
		}
	}

	return table, nil
}

func (c *Config) getQbitState(instance int, q *apps.QbitConfig) (*State, error) {
	state := &State{Instance: instance, Name: q.Name}
	start := time.Now()

	xfers, err := q.GetXfers()
	state.Elapsed = time.Since(start)

	if err != nil {
		return state, fmt.Errorf("getting transfers from instance %d: %w", instance, err)
	}

	for _, xfer := range xfers {
		state.Size += xfer.Size
		state.Uploaded += xfer.Uploaded
		state.Downloaded += int64(xfer.Downloaded)
		state.Downloads++

		switch strings.ToLower(strings.TrimSpace(xfer.State)) {
		case "stalledup", "moving", "forcedup":
			state.Seeding++
		case "downloading", "forceddl":
			state.Downloading++
		case "uploading":
			state.Uploading++
		case "pausedup", "pauseddl":
			state.Paused++
		case "queuedup", "checkingup", "allocating", "metadl", "queueddl", "stalleddl", "checkingdl", "checkingresumedata":
			state.Incomplete++
		case "unknown", "missingfiles", "error":
			state.Errors++
		default:
			state.Errors++
		}
	}

	return state, nil
}

func (c *Config) getRadarrState(instance int, r *apps.RadarrConfig) (*State, error) {
	state := &State{Instance: instance, Next: []*Sortable{}, Latest: []*Sortable{}, Name: r.Name}
	start := time.Now()

	movies, err := r.GetMovie(0)
	state.Elapsed = time.Since(start)

	if err != nil {
		return state, fmt.Errorf("getting movies from instance %d: %w", instance, err)
	}

	processRadarrState(state, movies)
	sort.Sort(sort.Reverse(dateSorter(state.Latest)))
	sort.Sort(dateSorter(state.Next))
	state.Latest.Shrink(showLatest)
	state.Next.Shrink(showNext)

	return state, nil
}

func processRadarrState(state *State, movies []*radarr.Movie) {
	for _, movie := range movies {
		state.Movies++
		state.Size += movie.SizeOnDisk

		if !movie.HasFile && movie.IsAvailable {
			state.Missing++
		}

		if !movie.HasFile && !movie.IsAvailable {
			state.Upcoming++
		}

		date := movie.DigitalRelease
		if date.IsZero() || movie.PhysicalRelease.After(time.Now()) {
			date = movie.PhysicalRelease
		}

		if date.After(time.Now()) && !movie.HasFile {
			state.Next = append(state.Next, &Sortable{Name: movie.Title, Date: date})
		}

		if movie.MovieFile != nil {
			state.Latest = append(state.Latest, &Sortable{Name: movie.Title, Date: movie.MovieFile.DateAdded})
		}
	}
}

func (c *Config) getReadarrState(instance int, r *apps.ReadarrConfig) (*State, error) {
	state := &State{Instance: instance, Next: []*Sortable{}, Name: r.Name}
	start := time.Now()

	books, err := r.GetBook("") // all books
	state.Elapsed = time.Since(start)

	if err != nil {
		return state, fmt.Errorf("getting books from instance %d: %w", instance, err)
	}

	authorIDs := make(map[int64]struct{})

	for _, book := range books {
		have := false
		state.Books++

		if book.Statistics != nil {
			authorIDs[book.AuthorID] = struct{}{}
			state.Percent += book.Statistics.PercentOfBooks
			state.Size += int64(book.Statistics.SizeOnDisk)
			state.Editions += book.Statistics.TotalBookCount
			state.Missing += int64(book.Statistics.BookCount - book.Statistics.BookFileCount)
			have = book.Statistics.BookCount-book.Statistics.BookFileCount < 1
		}

		if book.ReleaseDate.After(time.Now()) && book.Monitored && !have {
			state.Next = append(state.Next, &Sortable{
				id:   book.ID,
				Name: book.Title,
				Date: book.ReleaseDate,
				Sub:  book.Author.AuthorName,
			})
		}
	}

	state.Percent /= float64(state.Editions)
	state.Authors = len(authorIDs)
	sort.Sort(dateSorter(state.Next))
	state.Next.Shrink(showNext)

	if state.Latest, err = c.getReadarrHistory(r); err != nil {
		return state, fmt.Errorf("instance %d: %w", instance, err)
	}

	return state, nil
}

// getReadarrHistory is not done.
func (c *Config) getReadarrHistory(r *apps.ReadarrConfig) ([]*Sortable, error) {
	history, err := r.GetHistory(showLatest * 20) //nolint:gomnd
	if err != nil {
		return nil, fmt.Errorf("getting history: %w", err)
	}

	table := []*Sortable{}

	for _, rec := range history.Records {
		if len(table) >= showLatest {
			break
		} else if rec.EventType != "bookFileImported" {
			continue
		}

		// An error here gets swallowed.
		if book, err := r.GetBookByID(rec.BookID); err == nil {
			table = append(table, &Sortable{
				Name: book.Title,
				Sub:  book.Author.AuthorName,
				Date: rec.Date,
			})
		}
	}

	return table, nil
}

func (c *Config) getSonarrState(instance int, s *apps.SonarrConfig) (*State, error) {
	state := &State{Instance: instance, Next: []*Sortable{}, Name: s.Name}
	start := time.Now()

	allshows, err := s.GetAllSeries()
	state.Elapsed = time.Since(start)

	if err != nil {
		return state, fmt.Errorf("getting series from instance %d: %w", instance, err)
	}

	for _, show := range allshows {
		state.Shows++
		if show.Statistics != nil {
			state.Percent += show.Statistics.PercentOfEpisodes
			state.Size += show.Statistics.SizeOnDisk
			state.Episodes += int64(show.Statistics.TotalEpisodeCount)
			state.Missing += int64(show.Statistics.EpisodeCount - show.Statistics.EpisodeFileCount)
		}

		if show.NextAiring.After(time.Now()) {
			state.Next = append(state.Next, &Sortable{
				id:   show.ID,
				Name: show.Title,
				Date: show.NextAiring,
			})
		}
	}

	state.Percent /= float64(state.Shows)

	if state.Next, err = c.getSonarrStateUpcoming(s, state.Next); err != nil {
		return state, fmt.Errorf("instance %d: %w", instance, err)
	}

	if state.Latest, err = c.getSonarrHistory(s); err != nil {
		return state, fmt.Errorf("instance %d: %w", instance, err)
	}

	return state, nil
}

func (c *Config) getSonarrHistory(s *apps.SonarrConfig) ([]*Sortable, error) {
	history, err := s.GetHistory(showLatest * 20) //nolint:gomnd
	if err != nil {
		return nil, fmt.Errorf("getting history: %w", err)
	}

	table := []*Sortable{}

	for _, rec := range history.Records {
		if len(table) >= showLatest {
			break
		} else if rec.EventType != "downloadFolderImported" {
			c.Debug(rec.EventType, rec.SourceTitle)
			continue
		}

		series, err := s.GetSeriesByID(rec.SeriesID)
		if err != nil {
			continue
		}

		// An error here gets swallowed.
		if eps, err := s.GetSeriesEpisodes(rec.SeriesID); err == nil {
			for _, ep := range eps {
				if ep.ID == rec.EpisodeID {
					table = append(table, &Sortable{
						Name:    series.Title,
						Sub:     ep.Title,
						Date:    rec.Date,
						Season:  ep.SeasonNumber,
						Episode: ep.EpisodeNumber,
					})
				}
			}
		}
	}

	return table, nil
}

func (c *Config) getSonarrStateUpcoming(s *apps.SonarrConfig, next []*Sortable) ([]*Sortable, error) {
	sort.Sort(dateSorter(next))

	redo := []*Sortable{}

	for _, item := range next {
		eps, err := s.GetSeriesEpisodes(item.id)
		if err != nil {
			return nil, fmt.Errorf("getting series ID %d (%s): %w", item.id, item.Name, err)
		}

		for _, ep := range eps {
			if ep.AirDateUtc.Year() == item.Date.Year() && ep.AirDateUtc.YearDay() == item.Date.YearDay() &&
				ep.SeasonNumber != 0 && ep.EpisodeNumber != 0 {
				redo = append(redo, &Sortable{
					Name:    item.Name,
					Sub:     ep.Title,
					Date:    ep.AirDateUtc,
					Season:  ep.SeasonNumber,
					Episode: ep.EpisodeNumber,
				})

				break
			}
		}

		if len(redo) >= showNext {
			break
		}
	}

	return redo, nil
}

type dateSorter []*Sortable

func (s dateSorter) Len() int {
	return len(s)
}

func (s dateSorter) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s dateSorter) Less(i, j int) bool {
	return s[i].Date.Before(s[j].Date)
}

// Shrink a sortable list.
func (s *SortableList) Shrink(size int) {
	if s == nil {
		return
	}

	if len(*s) > size {
		*s = (*s)[:size]
	}
}
