package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	DefaultTMDBBaseURL = "https://api.themoviedb.org/3"
	// TMDB allows about 50 requests per second; staying well below keeps a shared IP safe.
	requestInterval = 50 * time.Millisecond
	maxResponseSize = 5 << 20
	maxRetries      = 3
)

var (
	ErrUnauthorized = errors.New("TMDB rejected the API key")
	ErrNotFound     = errors.New("not found on TMDB")
)

type TMDB struct {
	baseURL string
	key     string
	http    *http.Client

	mu   sync.Mutex
	next time.Time
}

func NewTMDB(key, baseURL string) *TMDB {
	if baseURL == "" {
		baseURL = DefaultTMDBBaseURL
	}
	return &TMDB{baseURL: strings.TrimRight(baseURL, "/"), key: key, http: &http.Client{Timeout: 20 * time.Second}}
}

// Read access tokens (v4) are JWTs; anything else is treated as a v3 API key.
func (c *TMDB) isBearer() bool {
	return strings.Count(c.key, ".") == 2
}

func (c *TMDB) wait(ctx context.Context) error {
	c.mu.Lock()
	now := time.Now()
	at := c.next
	if at.Before(now) {
		at = now
	}
	c.next = at.Add(requestInterval)
	c.mu.Unlock()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(time.Until(at)):
		return nil
	}
}

func (c *TMDB) get(ctx context.Context, path string, params url.Values, out any) error {
	if params == nil {
		params = url.Values{}
	}
	if !c.isBearer() {
		params.Set("api_key", c.key)
	}
	endpoint := c.baseURL + path + "?" + params.Encode()

	for attempt := 0; ; attempt++ {
		if err := c.wait(ctx); err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Accept", "application/json")
		if c.isBearer() {
			req.Header.Set("Authorization", "Bearer "+c.key)
		}
		resp, err := c.http.Do(req)
		if err != nil {
			return fmt.Errorf("TMDB %s: %w", path, err)
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
		resp.Body.Close()
		if err != nil {
			return err
		}
		switch {
		case resp.StatusCode == http.StatusTooManyRequests && attempt < maxRetries:
			delay, _ := strconv.Atoi(resp.Header.Get("Retry-After"))
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(min(max(delay, 1), 10)) * time.Second):
			}
			continue
		case resp.StatusCode == http.StatusUnauthorized:
			return ErrUnauthorized
		case resp.StatusCode == http.StatusNotFound:
			return ErrNotFound
		case resp.StatusCode >= 300:
			return fmt.Errorf("TMDB %s: HTTP %d", path, resp.StatusCode)
		}
		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("decode TMDB %s: %w", path, err)
		}
		return nil
	}
}

type SearchResult struct {
	ID            int     `json:"id"`
	Title         string  `json:"title"`
	OriginalTitle string  `json:"original_title"`
	Name          string  `json:"name"`
	OriginalName  string  `json:"original_name"`
	ReleaseDate   string  `json:"release_date"`
	FirstAirDate  string  `json:"first_air_date"`
	Overview      string  `json:"overview"`
	PosterPath    string  `json:"poster_path"`
	Popularity    float64 `json:"popularity"`
}

// Titles returns the localized and original titles, whatever the media type.
func (r SearchResult) Titles() (string, string) {
	if r.Title != "" || r.OriginalTitle != "" {
		return r.Title, r.OriginalTitle
	}
	return r.Name, r.OriginalName
}

func (r SearchResult) Year() int {
	return yearOf(r.ReleaseDate + r.FirstAirDate)
}

type Genre struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type CastMember struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Character   string `json:"character"`
	ProfilePath string `json:"profile_path"`
	Order       int    `json:"order"`
}

type Credits struct {
	Cast []CastMember `json:"cast"`
	Crew []struct {
		ID          int    `json:"id"`
		Job         string `json:"job"`
		Name        string `json:"name"`
		ProfilePath string `json:"profile_path"`
	} `json:"crew"`
}

// Image is one picture of a title's image gallery; Language is null for pictures without text.
type Image struct {
	FilePath    string  `json:"file_path"`
	Language    *string `json:"iso_639_1"`
	VoteAverage float64 `json:"vote_average"`
	Width       int     `json:"width"`
}

type Images struct {
	Logos []Image `json:"logos"`
}

type Movie struct {
	Images              Images `json:"images"`
	VoteCount           *int32 `json:"vote_count"`
	OriginalLanguage    string `json:"original_language"`
	ProductionCountries []struct {
		Code string `json:"iso_3166_1"`
		Name string `json:"name"`
	} `json:"production_countries"`
	Keywords struct {
		Keywords []Genre `json:"keywords"`
	} `json:"keywords"`
	ID            int     `json:"id"`
	Title         string  `json:"title"`
	OriginalTitle string  `json:"original_title"`
	Overview      string  `json:"overview"`
	Tagline       string  `json:"tagline"`
	ReleaseDate   string  `json:"release_date"`
	Runtime       int     `json:"runtime"`
	VoteAverage   float64 `json:"vote_average"`
	Genres        []Genre `json:"genres"`
	PosterPath    string  `json:"poster_path"`
	BackdropPath  string  `json:"backdrop_path"`
	ImdbID        string  `json:"imdb_id"`
	Credits       Credits `json:"credits"`
}

type TVSeasonSummary struct {
	SeasonNumber int `json:"season_number"`
	EpisodeCount int `json:"episode_count"`
}

type TV struct {
	Images           Images   `json:"images"`
	VoteCount        *int32   `json:"vote_count"`
	OriginalLanguage string   `json:"original_language"`
	OriginCountry    []string `json:"origin_country"`
	Keywords         struct {
		Results []Genre `json:"results"`
	} `json:"keywords"`
	ID             int               `json:"id"`
	Name           string            `json:"name"`
	OriginalName   string            `json:"original_name"`
	Overview       string            `json:"overview"`
	Tagline        string            `json:"tagline"`
	FirstAirDate   string            `json:"first_air_date"`
	EpisodeRunTime []int             `json:"episode_run_time"`
	VoteAverage    float64           `json:"vote_average"`
	Genres         []Genre           `json:"genres"`
	PosterPath     string            `json:"poster_path"`
	BackdropPath   string            `json:"backdrop_path"`
	Seasons        []TVSeasonSummary `json:"seasons"`
	CreatedBy      []struct {
		ID          int    `json:"id"`
		Name        string `json:"name"`
		ProfilePath string `json:"profile_path"`
	} `json:"created_by"`
	Credits     Credits `json:"credits"`
	ExternalIDs struct {
		ImdbID string `json:"imdb_id"`
	} `json:"external_ids"`
}

type Season struct {
	SeasonNumber int       `json:"season_number"`
	Name         string    `json:"name"`
	Overview     string    `json:"overview"`
	PosterPath   string    `json:"poster_path"`
	AirDate      string    `json:"air_date"`
	Episodes     []Episode `json:"episodes"`
}

type Episode struct {
	EpisodeNumber int     `json:"episode_number"`
	Name          string  `json:"name"`
	Overview      string  `json:"overview"`
	StillPath     string  `json:"still_path"`
	AirDate       string  `json:"air_date"`
	Runtime       int     `json:"runtime"`
	VoteAverage   float64 `json:"vote_average"`
}

type FindResult struct {
	MovieResults     []SearchResult `json:"movie_results"`
	TVResults        []SearchResult `json:"tv_results"`
	TVEpisodeResults []struct {
		ShowID int `json:"show_id"`
	} `json:"tv_episode_results"`
}

func (c *TMDB) Validate(ctx context.Context) error {
	var out map[string]any
	return c.get(ctx, "/configuration", nil, &out)
}

func (c *TMDB) SearchMovie(ctx context.Context, query string, year int, lang string) ([]SearchResult, error) {
	p := url.Values{"query": {query}, "language": {lang}, "include_adult": {"false"}}
	if year > 0 {
		p.Set("year", strconv.Itoa(year))
	}
	var out struct {
		Results []SearchResult `json:"results"`
	}
	err := c.get(ctx, "/search/movie", p, &out)
	return out.Results, err
}

func (c *TMDB) SearchTV(ctx context.Context, query string, year int, lang string) ([]SearchResult, error) {
	p := url.Values{"query": {query}, "language": {lang}, "include_adult": {"false"}}
	if year > 0 {
		p.Set("first_air_date_year", strconv.Itoa(year))
	}
	var out struct {
		Results []SearchResult `json:"results"`
	}
	err := c.get(ctx, "/search/tv", p, &out)
	return out.Results, err
}

func (c *TMDB) Movie(ctx context.Context, id int, lang string) (Movie, error) {
	var out Movie
	err := c.get(ctx, "/movie/"+strconv.Itoa(id), url.Values{"language": {lang}, "append_to_response": {"credits,keywords,images"}, "include_image_language": {imageLanguages(lang)}}, &out)
	return out, err
}

func (c *TMDB) TV(ctx context.Context, id int, lang string) (TV, error) {
	var out TV
	err := c.get(ctx, "/tv/"+strconv.Itoa(id), url.Values{"language": {lang}, "append_to_response": {"credits,external_ids,keywords,images"}, "include_image_language": {imageLanguages(lang)}}, &out)
	return out, err
}

func (c *TMDB) Season(ctx context.Context, tvID, number int, lang string) (Season, error) {
	var out Season
	err := c.get(ctx, fmt.Sprintf("/tv/%d/season/%d", tvID, number), url.Values{"language": {lang}}, &out)
	return out, err
}

func (c *TMDB) FindIMDb(ctx context.Context, imdbID string) (FindResult, error) {
	var out FindResult
	err := c.get(ctx, "/find/"+url.PathEscape(imdbID), url.Values{"external_source": {"imdb_id"}}, &out)
	return out, err
}

// imageLanguages lists the image languages worth fetching for a metadata language ("fr-FR" → "fr,en,null").
func imageLanguages(lang string) string {
	code, _, _ := strings.Cut(lang, "-")
	code = strings.ToLower(code)
	if code == "" || code == "en" {
		return "en,null"
	}
	return code + ",en,null"
}

// BestLogo picks the logo to show: in the metadata language, else in English, else without text, and the best
// rated one among those. TMDB renders an SVG logo as a PNG of any size when its extension is swapped, so SVG logos
// are returned as that PNG: the image cache and the clients then only deal with raster pictures.
func BestLogo(logos []Image, lang string) string {
	code, _, _ := strings.Cut(strings.ToLower(lang), "-")
	rank := func(l Image) int {
		switch {
		case l.Language != nil && *l.Language == code:
			return 0
		case l.Language != nil && *l.Language == "en":
			return 1
		case l.Language == nil || *l.Language == "":
			return 2
		}
		return 3
	}
	best, bestRank := Image{}, 4
	for _, l := range logos {
		ext := strings.ToLower(path.Ext(l.FilePath))
		if (ext != ".png" && ext != ".svg") || !strings.HasPrefix(l.FilePath, "/") {
			continue
		}
		r := rank(l)
		if r < bestRank || (r == bestRank && (l.VoteAverage > best.VoteAverage || (l.VoteAverage == best.VoteAverage && l.Width > best.Width))) {
			best, bestRank = l, r
		}
	}
	if strings.EqualFold(path.Ext(best.FilePath), ".svg") {
		return strings.TrimSuffix(best.FilePath, path.Ext(best.FilePath)) + ".png"
	}
	return best.FilePath
}

func yearOf(date string) int {
	if len(date) < 4 {
		return 0
	}
	y, _ := strconv.Atoi(date[:4])
	return y
}

func parseDate(s string) *time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil
	}
	return &t
}
