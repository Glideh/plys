package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"toto/api/internal/scanner"
	"toto/api/internal/store"
)

const (
	DefaultFanKaiBaseURL = "https://metadata.fankai.fr"
	fankaiListTTL        = 30 * time.Minute
	maxFanKaiResponse    = 10 << 20
	maxFanKaiResults     = 20
)

var ErrFanKaiNotFound = errors.New("not found on FanKai")
var ErrFanKaiUnavailable = errors.New("FanKai unavailable")

type FanKaiSeries struct {
	ID            int     `json:"id"`
	Title         string  `json:"title"`
	OriginalTitle string  `json:"original_title"`
	ShowTitle     string  `json:"show_title"`
	Year          int     `json:"year"`
	Premiered     string  `json:"premiered"`
	Plot          string  `json:"plot"`
	Genres        string  `json:"genres"`
	RatingValue   float64 `json:"rating_value"`
	PosterImage   string  `json:"poster_image"`
	FanartImage   string  `json:"fanart_image"`
}

type FanKaiSearchResult struct {
	ID            int
	Title         string
	OriginalTitle string
	Year          int
	Overview      string
}

type fanKaiClient struct {
	baseURL string
	http    *http.Client
}

func newFanKaiClient(baseURL string) *fanKaiClient {
	if baseURL == "" {
		baseURL = DefaultFanKaiBaseURL
	}
	return &fanKaiClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 20 * time.Second},
	}
}

func (c *fanKaiClient) get(ctx context.Context, endpoint string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/"+strings.TrimLeft(endpoint, "/"), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %s: %v", ErrFanKaiUnavailable, endpoint, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return ErrFanKaiNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: %s: HTTP %d", ErrFanKaiUnavailable, endpoint, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxFanKaiResponse+1))
	if err != nil {
		return fmt.Errorf("%w: read %s: %v", ErrFanKaiUnavailable, endpoint, err)
	}
	if len(body) > maxFanKaiResponse {
		return fmt.Errorf("%w: %s response exceeds %d bytes", ErrFanKaiUnavailable, endpoint, maxFanKaiResponse)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("%w: decode %s: %v", ErrFanKaiUnavailable, endpoint, err)
	}
	return nil
}

func (s *Service) SearchFanKai(ctx context.Context, query string, year int) ([]FanKaiSearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" || len([]rune(query)) > 200 {
		return nil, errors.New("FanKai search query must be between 1 and 200 characters")
	}
	s.fankaiMu.Lock()
	defer s.fankaiMu.Unlock()
	if time.Now().After(s.fankaiListExpires) || s.fankaiList == nil {
		var series []FanKaiSeries
		if err := s.fankai().get(ctx, "series?paginate=false", &series); err != nil {
			return nil, err
		}
		s.fankaiList = series
		s.fankaiListExpires = time.Now().Add(fankaiListTTL)
	}
	return rankFanKaiResults(s.fankaiList, query, year), nil
}

func rankFanKaiResults(series []FanKaiSeries, query string, year int) []FanKaiSearchResult {
	key := scanner.MatchKey(query)
	if key == "" {
		return nil
	}
	type ranked struct {
		result FanKaiSearchResult
		score  int
	}
	rankedResults := make([]ranked, 0)
	for _, serie := range series {
		title := firstFanKaiValue(serie.Title, serie.ShowTitle)
		if title == "" || serie.ID <= 0 {
			continue
		}
		best := 0
		for _, candidate := range []string{serie.Title, serie.ShowTitle, serie.OriginalTitle} {
			candidateKey := scanner.MatchKey(candidate)
			if candidateKey == "" {
				continue
			}
			distance := levenshtein(key, candidateKey)
			length := max(len([]rune(key)), len([]rune(candidateKey)))
			score := 100 - distance*100/length
			if strings.Contains(candidateKey, key) || strings.Contains(key, candidateKey) {
				score = max(score, 85)
			}
			best = max(best, score)
		}
		if best < 35 {
			continue
		}
		if year > 0 && serie.Year > 0 {
			best -= min(abs(year-serie.Year)*4, 20)
		}
		rankedResults = append(rankedResults, ranked{
			result: FanKaiSearchResult{
				ID: serie.ID, Title: title, OriginalTitle: serie.OriginalTitle,
				Year: serie.Year, Overview: serie.Plot,
			},
			score: best,
		})
	}
	slices.SortStableFunc(rankedResults, func(a, b ranked) int { return b.score - a.score })
	out := make([]FanKaiSearchResult, 0, min(len(rankedResults), maxFanKaiResults))
	for _, result := range rankedResults[:min(len(rankedResults), maxFanKaiResults)] {
		out = append(out, result.result)
	}
	return out
}

func (s *Service) fankai() *fanKaiClient {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fankaiClient == nil {
		s.fankaiClient = newFanKaiClient(s.fankaiBaseURL)
	}
	return s.fankaiClient
}

func (s *Service) refreshFanKai(ctx context.Context, itemID uuid.UUID, fanKaiID int) error {
	var serie FanKaiSeries
	if err := s.fankai().get(ctx, "series/"+strconv.Itoa(fanKaiID), &serie); err != nil {
		return err
	}
	if serie.ID != fanKaiID || strings.TrimSpace(serie.Title) == "" {
		return fmt.Errorf("%w: FanKai returned an invalid series", ErrFanKaiUnavailable)
	}

	var posterPath, backdropPath *string
	if s.images != nil {
		if serie.PosterImage != "" {
			path, err := s.images.FanKai(ctx, itemID.String()+"-poster", serie.PosterImage)
			if err != nil {
				return fmt.Errorf("%w: cache poster: %v", ErrFanKaiUnavailable, err)
			}
			posterPath = &path
		}
		if serie.FanartImage != "" {
			path, err := s.images.FanKai(ctx, itemID.String()+"-backdrop", serie.FanartImage)
			if err != nil {
				return fmt.Errorf("%w: cache backdrop: %v", ErrFanKaiUnavailable, err)
			}
			backdropPath = &path
		}
	}

	title := firstFanKaiValue(serie.Title, serie.ShowTitle)
	genres := make([]string, 0)
	for _, genre := range strings.Split(serie.Genres, ",") {
		if genre = strings.TrimSpace(genre); genre != "" {
			genres = append(genres, genre)
		}
	}
	year := serie.Year
	if year == 0 && len(serie.Premiered) >= 4 {
		year, _ = strconv.Atoi(serie.Premiered[:4])
	}
	fankaiID32 := int32(fanKaiID)
	return s.q.SetFanKaiMetadata(ctx, store.SetFanKaiMetadataParams{
		FankaiID: &fankaiID32, Title: title, SortTitle: scanner.SortTitle(title),
		OriginalTitle: opt(serie.OriginalTitle), Year: optInt(year),
		Overview: opt(serie.Plot), Tagline: nil, ReleaseDate: parseDate(serie.Premiered),
		Rating: optRating(serie.RatingValue), Genres: genres,
		PosterPath: posterPath, BackdropPath: backdropPath, ID: itemID,
	})
}

func firstFanKaiValue(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func levenshtein(a, b string) int {
	ar, br := []rune(a), []rune(b)
	previous := make([]int, len(br)+1)
	for j := range previous {
		previous[j] = j
	}
	for i, r := range ar {
		current := make([]int, len(br)+1)
		current[0] = i + 1
		for j, other := range br {
			cost := 1
			if r == other {
				cost = 0
			}
			current[j+1] = min(current[j]+1, previous[j+1]+1, previous[j]+cost)
		}
		previous = current
	}
	return previous[len(br)]
}
