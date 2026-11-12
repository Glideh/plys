// Package opensubtitles searches and downloads subtitles on opensubtitles.com (REST API v1).
package opensubtitles

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	DefaultBaseURL = "https://api.opensubtitles.com/api/v1"
	// Login tokens are valid for 24 hours; renewing earlier avoids a failed download.
	tokenLifetime   = 23 * time.Hour
	maxResponseSize = 2 << 20
	// Subtitle files are small; anything larger is not a subtitle.
	maxSubtitleSize = 5 << 20
)

var (
	ErrNotConfigured = errors.New("OpenSubtitles is not configured")
	// ErrUnauthorized means the API key, or the login, was rejected.
	ErrUnauthorized = errors.New("OpenSubtitles rejected the credentials")
	ErrQuota        = errors.New("OpenSubtitles download quota reached")
)

// Credentials are read on every call so that a change of the settings applies at once.
type Credentials struct {
	APIKey   string
	Username string
	Password string
}

type CredentialsFunc func(ctx context.Context) (Credentials, error)

type Client struct {
	baseURL     string
	userAgent   string
	credentials CredentialsFunc
	http        *http.Client

	mu       sync.Mutex
	token    string
	tokenAPI string
	tokenFor Credentials
	tokenAt  time.Time
}

// New returns a client; userAgent identifies the application, as OpenSubtitles requires ("Plys v1.2.3").
func New(baseURL, userAgent string, credentials CredentialsFunc) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"), userAgent: userAgent, credentials: credentials,
		http: &http.Client{Timeout: 20 * time.Second},
	}
}

// Configured tells whether an API key is set.
func (c *Client) Configured(ctx context.Context) (bool, error) {
	cr, err := c.credentials(ctx)
	return cr.APIKey != "", err
}

// Query describes the video; the most precise identifiers available should be set.
type Query struct {
	// Language is an OpenSubtitles code: ISO 639-1 ("fr"), or "pt-BR", "zh-CN"...
	Language string
	// TMDBID identifies a movie; ParentTMDBID with Season and Episode identify an episode.
	TMDBID       int
	ParentTMDBID int
	Season       int
	Episode      int
	// Text is searched when no TMDB ID is known.
	Text      string
	MovieHash string
}

type Result struct {
	FileID            int     `json:"fileId"`
	Language          string  `json:"language"`
	Release           string  `json:"release"`
	DownloadCount     int     `json:"downloadCount"`
	HearingImpaired   bool    `json:"hearingImpaired"`
	MachineTranslated bool    `json:"machineTranslated"`
	Trusted           bool    `json:"trusted"`
	HashMatch         bool    `json:"hashMatch"`
	FPS               float64 `json:"fps,omitempty"`
	Uploader          string  `json:"uploader,omitempty"`
}

type searchResponse struct {
	Data []struct {
		Attributes struct {
			Language          string  `json:"language"`
			Release           string  `json:"release"`
			DownloadCount     int     `json:"download_count"`
			HearingImpaired   bool    `json:"hearing_impaired"`
			AITranslated      bool    `json:"ai_translated"`
			MachineTranslated bool    `json:"machine_translated"`
			FromTrusted       bool    `json:"from_trusted"`
			MovieHashMatch    bool    `json:"moviehash_match"`
			FPS               float64 `json:"fps"`
			Uploader          struct {
				Name string `json:"name"`
			} `json:"uploader"`
			Files []struct {
				FileID   int    `json:"file_id"`
				FileName string `json:"file_name"`
			} `json:"files"`
		} `json:"attributes"`
	} `json:"data"`
}

// Search returns the single-file subtitles found, those matching the file hash first, then the most downloaded.
func (c *Client) Search(ctx context.Context, q Query) ([]Result, error) {
	cr, err := c.credentials(ctx)
	if err != nil {
		return nil, err
	}
	if cr.APIKey == "" {
		return nil, ErrNotConfigured
	}
	params := url.Values{}
	// OpenSubtitles redirects requests whose parameters are not lowercase and sorted; Encode sorts them.
	params.Set("languages", strings.ToLower(q.Language))
	switch {
	case q.ParentTMDBID > 0:
		params.Set("parent_tmdb_id", strconv.Itoa(q.ParentTMDBID))
		params.Set("season_number", strconv.Itoa(q.Season))
		params.Set("episode_number", strconv.Itoa(q.Episode))
		params.Set("type", "episode")
	case q.TMDBID > 0:
		params.Set("tmdb_id", strconv.Itoa(q.TMDBID))
		params.Set("type", "movie")
	case q.Text != "":
		params.Set("query", strings.ToLower(q.Text))
	}
	if q.MovieHash != "" {
		params.Set("moviehash", q.MovieHash)
	}
	var resp searchResponse
	if err := c.do(ctx, http.MethodGet, c.baseURL+"/subtitles?"+params.Encode(), cr.APIKey, "", nil, &resp); err != nil {
		return nil, err
	}
	out := make([]Result, 0, len(resp.Data))
	for _, d := range resp.Data {
		a := d.Attributes
		// Movies split over several files (CD1, CD2) cannot be played as one track.
		if len(a.Files) != 1 {
			continue
		}
		release := a.Release
		if release == "" {
			release = a.Files[0].FileName
		}
		out = append(out, Result{
			FileID: a.Files[0].FileID, Language: a.Language, Release: release, DownloadCount: a.DownloadCount,
			HearingImpaired: a.HearingImpaired, MachineTranslated: a.AITranslated || a.MachineTranslated,
			Trusted: a.FromTrusted, HashMatch: a.MovieHashMatch, FPS: a.FPS, Uploader: a.Uploader.Name,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].HashMatch != out[j].HashMatch {
			return out[i].HashMatch
		}
		return out[i].DownloadCount > out[j].DownloadCount
	})
	return out, nil
}

type downloadResponse struct {
	Link string `json:"link"`
}

// Download returns the subtitle file converted to SRT by OpenSubtitles. Each call uses one download of the quota.
func (c *Client) Download(ctx context.Context, fileID int) ([]byte, error) {
	cr, err := c.credentials(ctx)
	if err != nil {
		return nil, err
	}
	if cr.APIKey == "" {
		return nil, ErrNotConfigured
	}
	body, _ := json.Marshal(map[string]any{"file_id": fileID, "sub_format": "srt"})
	var resp downloadResponse
	for attempt := 0; ; attempt++ {
		base, token, err := c.session(ctx, cr)
		if err != nil {
			return nil, err
		}
		err = c.do(ctx, http.MethodPost, base+"/download", cr.APIKey, token, body, &resp)
		// An expired login token is renewed once.
		if errors.Is(err, ErrUnauthorized) && token != "" && attempt == 0 {
			c.forget()
			continue
		}
		if err != nil {
			return nil, err
		}
		break
	}
	if resp.Link == "" {
		return nil, errors.New("OpenSubtitles returned no download link")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, resp.Link, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download subtitle file: status %d", res.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, maxSubtitleSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxSubtitleSize {
		return nil, errors.New("subtitle file too large")
	}
	return data, nil
}

// Validate checks an API key, and the login when one is given, before they are saved.
func (c *Client) Validate(ctx context.Context, cr Credentials) error {
	if cr.Username != "" {
		_, _, err := c.login(ctx, cr)
		return err
	}
	var discard json.RawMessage
	return c.do(ctx, http.MethodGet, c.baseURL+"/infos/formats", cr.APIKey, "", nil, &discard)
}

// session returns the API base URL and token to download with; without a login, downloads are anonymous.
func (c *Client) session(ctx context.Context, cr Credentials) (string, string, error) {
	if cr.Username == "" {
		return c.baseURL, "", nil
	}
	c.mu.Lock()
	if c.token != "" && c.tokenFor == cr && time.Since(c.tokenAt) < tokenLifetime {
		base, token := c.tokenAPI, c.token
		c.mu.Unlock()
		return base, token, nil
	}
	c.mu.Unlock()
	base, token, err := c.login(ctx, cr)
	if err != nil {
		return "", "", err
	}
	c.mu.Lock()
	c.token, c.tokenAPI, c.tokenFor, c.tokenAt = token, base, cr, time.Now()
	c.mu.Unlock()
	return base, token, nil
}

func (c *Client) forget() {
	c.mu.Lock()
	c.token = ""
	c.mu.Unlock()
}

type loginResponse struct {
	Token   string `json:"token"`
	BaseURL string `json:"base_url"`
}

// login returns the API base URL to use with the token: VIP accounts are sent to another host.
func (c *Client) login(ctx context.Context, cr Credentials) (string, string, error) {
	body, _ := json.Marshal(map[string]string{"username": cr.Username, "password": cr.Password})
	var resp loginResponse
	if err := c.do(ctx, http.MethodPost, c.baseURL+"/login", cr.APIKey, "", body, &resp); err != nil {
		return "", "", err
	}
	if resp.Token == "" {
		return "", "", ErrUnauthorized
	}
	base := c.baseURL
	if resp.BaseURL != "" && strings.HasPrefix(c.baseURL, "https://") {
		if u, err := url.Parse(c.baseURL); err == nil && u.Host != resp.BaseURL {
			base = "https://" + resp.BaseURL + u.Path
		}
	}
	return base, resp.Token, nil
}

func (c *Client) do(ctx context.Context, method, endpoint, apiKey, token string, body []byte, out any) error {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Api-Key", apiKey)
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, maxResponseSize))
	if err != nil {
		return err
	}
	switch {
	case res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden:
		return ErrUnauthorized
	// OpenSubtitles answers 406 once the daily downloads are used up.
	case res.StatusCode == http.StatusNotAcceptable || (res.StatusCode == http.StatusTooManyRequests && strings.HasSuffix(endpoint, "/download")):
		return ErrQuota
	case res.StatusCode < 200 || res.StatusCode > 299:
		return fmt.Errorf("OpenSubtitles %s %s: status %d", method, req.URL.Path, res.StatusCode)
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode OpenSubtitles response: %w", err)
	}
	return nil
}
