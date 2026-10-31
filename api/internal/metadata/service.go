package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"toto/api/internal/scanner"
	"toto/api/internal/settings"
	"toto/api/internal/store"
)

const (
	fallbackLanguage = "en-US"
	maxCast          = 15
	nfoSamplePaths   = 5
	thumbTimeout     = 60 * time.Second
)

// Prefetch the posters shown in library grids so the first visit does not wait on TMDB.
const listPosterSize = "w342"

var ErrNoAPIKey = errors.New("no TMDB API key configured")

type Service struct {
	pool     *pgxpool.Pool
	q        *store.Queries
	settings *settings.Store
	cacheDir string
	ffmpeg   string
	baseURL  string

	mu        sync.Mutex
	client    *TMDB
	clientKey string

	images  *ImageCache
	warming atomic.Bool
}

// SetImages enables poster prefetching.
func (s *Service) SetImages(c *ImageCache) { s.images = c }

// WarmImages downloads the grid posters of every library that are not cached yet.
func (s *Service) WarmImages(ctx context.Context) {
	if s.images == nil || !s.warming.CompareAndSwap(false, true) {
		return
	}
	defer s.warming.Store(false)
	paths, err := s.q.ListTMDBPosterPaths(ctx)
	if err != nil {
		if ctx.Err() == nil {
			slog.Warn("list posters to prefetch", "err", err)
		}
		return
	}
	files := make([]string, 0, len(paths))
	for _, p := range paths {
		files = append(files, strings.TrimPrefix(p, "/"))
	}
	if n := s.images.Prefetch(ctx, listPosterSize, files); n > 0 {
		slog.Info("posters prefetched", "count", n)
	}
}

func NewService(pool *pgxpool.Pool, st *settings.Store, cacheDir, ffmpeg, tmdbBaseURL string) *Service {
	return &Service{pool: pool, q: store.New(pool), settings: st, cacheDir: cacheDir, ffmpeg: ffmpeg, baseURL: tmdbBaseURL}
}

// Client returns a TMDB client for the configured key, or ErrNoAPIKey.
func (s *Service) Client(ctx context.Context) (*TMDB, string, error) {
	key, err := s.settings.TMDBKey(ctx)
	if err != nil {
		return nil, "", err
	}
	lang, err := s.settings.MetadataLanguage(ctx)
	if err != nil {
		return nil, "", err
	}
	if key == "" {
		return nil, lang, ErrNoAPIKey
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.client == nil || s.clientKey != key {
		s.client, s.clientKey = NewTMDB(key, s.baseURL), key
	}
	return s.client, lang, nil
}

// ValidateKey checks a key against TMDB before it is saved.
func (s *Service) ValidateKey(ctx context.Context, key string) error {
	return NewTMDB(key, s.baseURL).Validate(ctx)
}

// RefreshLibrary fetches metadata for new items and for items whose language changed. Without an API key,
// movies and series stay pending until one is configured; thumbnails of personal videos are still extracted.
func (s *Service) RefreshLibrary(ctx context.Context, lib store.Library, progress func(done, total int)) error {
	defer func() { go s.WarmImages(ctx) }()

	client, lang, err := s.Client(ctx)
	if err != nil && !errors.Is(err, ErrNoAPIKey) {
		return err
	}
	items, err := s.q.ListItemsToMatch(ctx, store.ListItemsToMatchParams{LibraryID: lib.ID, Language: lang})
	if err != nil {
		return err
	}
	var firstErr error
	for i, it := range items {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		var err error
		switch {
		case it.Kind == "video":
			err = s.thumbnail(ctx, it.ID)
		case client == nil:
		case it.Kind == "movie":
			err = s.refreshMovie(ctx, client, lang, lib, it.ID, nil)
		case it.Kind == "series":
			err = s.refreshSeries(ctx, client, lang, lib, it.ID, nil)
		}
		if errors.Is(err, ErrUnauthorized) {
			return err
		}
		// The item may have been merged into another one earlier in this pass.
		if errors.Is(err, pgx.ErrNoRows) {
			err = nil
		}
		if err != nil && ctx.Err() == nil {
			slog.Warn("metadata refresh failed", "item_id", it.ID, "title", it.Title, "err", err)
			if firstErr == nil {
				firstErr = err
			}
		}
		progress(i+1, len(items))
	}
	return firstErr
}

// RefreshItem refreshes one movie or series now. A non-nil tmdbID is an admin choice and locks the match.
func (s *Service) RefreshItem(ctx context.Context, itemID uuid.UUID, tmdbID *int) error {
	it, err := s.q.GetItem(ctx, itemID)
	if err != nil {
		return err
	}
	lib, err := s.q.GetLibrary(ctx, it.LibraryID)
	if err != nil {
		return err
	}
	if it.Kind == "video" {
		return s.thumbnail(ctx, it.ID)
	}
	client, lang, err := s.Client(ctx)
	if err != nil {
		return err
	}
	switch it.Kind {
	case "movie":
		return s.refreshMovie(ctx, client, lang, lib, it.ID, tmdbID)
	case "series":
		return s.refreshSeries(ctx, client, lang, lib, it.ID, tmdbID)
	}
	return fmt.Errorf("items of kind %q have no metadata of their own", it.Kind)
}

// Search lists TMDB candidates for the manual matching screen.
func (s *Service) Search(ctx context.Context, kind, query string, year int) ([]SearchResult, error) {
	client, lang, err := s.Client(ctx)
	if err != nil {
		return nil, err
	}
	if kind == "series" {
		return client.SearchTV(ctx, query, year, lang)
	}
	return client.SearchMovie(ctx, query, year, lang)
}

func (s *Service) refreshMovie(ctx context.Context, c *TMDB, lang string, lib store.Library, itemID uuid.UUID, forced *int) error {
	it, err := s.q.GetItem(ctx, itemID)
	if err != nil {
		return err
	}
	status := "matched"
	id := 0
	switch {
	case forced != nil:
		id, status = *forced, "manual"
	case it.TmdbID != nil && (it.MatchStatus == "matched" || it.MatchStatus == "manual"):
		id, status = int(*it.TmdbID), it.MatchStatus
	default:
		if id, err = s.identifyMovie(ctx, c, lang, it); err != nil {
			return err
		}
	}
	if id == 0 {
		return s.markUnmatched(ctx, it.ID)
	}

	m, err := c.Movie(ctx, id, lang)
	if err != nil {
		return err
	}
	if m.Overview == "" && lang != fallbackLanguage {
		if en, err := c.Movie(ctx, id, fallbackLanguage); err == nil {
			m.Overview, m.Tagline = en.Overview, cmpOr(m.Tagline, en.Tagline)
		}
	}
	cast, err := castJSON(m.Credits.Cast)
	if err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		err := q.UpdateItemMetadata(ctx, store.UpdateItemMetadataParams{
			ID: it.ID, TmdbID: ptr(int32(id)), ImdbID: opt(m.ImdbID),
			Title: m.Title, SortTitle: scanner.SortTitle(m.Title), OriginalTitle: opt(m.OriginalTitle),
			Year: optYear(m.ReleaseDate), Overview: opt(m.Overview), Tagline: opt(m.Tagline),
			ReleaseDate: parseDate(m.ReleaseDate), RuntimeMinutes: optInt(m.Runtime), Rating: optRating(m.VoteAverage),
			Genres: genreNames(m.Genres), CastMembers: cast, Directors: crewNames(m.Credits, "Director"),
			PosterPath: opt(m.PosterPath), BackdropPath: opt(m.BackdropPath),
			MatchStatus: status, MetadataLanguage: &lang,
		})
		if err != nil {
			return err
		}
		if err := q.SetChildrenMatchStatus(ctx, store.SetChildrenMatchStatusParams{ParentID: &it.ID, MatchStatus: status}); err != nil {
			return err
		}
		_, err = mergeDuplicate(ctx, q, lib.ID, "movie", int32(id), it.ID)
		return err
	})
}

func (s *Service) identifyMovie(ctx context.Context, c *TMDB, lang string, it store.Item) (int, error) {
	paths, err := s.q.ListItemFilePaths(ctx, store.ListItemFilePathsParams{ItemID: it.ID, MaxPaths: nfoSamplePaths})
	if err != nil {
		return 0, err
	}
	if ids := findNFOIDs(paths); ids.TMDB > 0 && ids.Kind == "movie" {
		return ids.TMDB, nil
	} else if ids.IMDb != "" {
		found, err := c.FindIMDb(ctx, ids.IMDb)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return 0, err
		}
		if len(found.MovieResults) > 0 {
			return found.MovieResults[0].ID, nil
		}
	}
	// File names come first: the stored title may already come from an earlier, possibly wrong, match.
	var titles []string
	year := 0
	for _, p := range paths {
		// The folder is included because releases often carry the year in it.
		info := scanner.ParseMovie(filepath.Join(filepath.Base(filepath.Dir(p)), filepath.Base(p)))
		titles = append(titles, info.Title, info.AltTitle)
		if year == 0 {
			year = info.Year
		}
	}
	titles = append(titles, it.Title)
	if year == 0 && it.Year != nil {
		year = int(*it.Year)
	}
	fileMinutes := 0
	if src, err := s.q.GetItemThumbSource(ctx, &it.ID); err == nil && src.DurationSeconds != nil {
		fileMinutes = int(*src.DurationSeconds / 60)
	}
	return s.search(searchRequest{
		titles: titles,
		year:   year,
		find:   func(q string, y int) ([]SearchResult, error) { return c.SearchMovie(ctx, q, y, lang) },
		runtime: func(id int) (int, error) {
			m, err := c.Movie(ctx, id, lang)
			return m.Runtime, err
		},
		fileMinutes: fileMinutes,
	})
}

func (s *Service) refreshSeries(ctx context.Context, c *TMDB, lang string, lib store.Library, itemID uuid.UUID, forced *int) error {
	it, err := s.q.GetItem(ctx, itemID)
	if err != nil {
		return err
	}
	status := "matched"
	id := 0
	switch {
	case forced != nil:
		id, status = *forced, "manual"
	case it.TmdbID != nil && (it.MatchStatus == "matched" || it.MatchStatus == "manual"):
		id, status = int(*it.TmdbID), it.MatchStatus
	default:
		if id, err = s.identifySeries(ctx, c, lang, it); err != nil {
			return err
		}
	}
	if id == 0 {
		return s.markUnmatched(ctx, it.ID)
	}

	tv, err := c.TV(ctx, id, lang)
	if err != nil {
		return err
	}
	if tv.Overview == "" && lang != fallbackLanguage {
		if en, err := c.TV(ctx, id, fallbackLanguage); err == nil {
			tv.Overview = en.Overview
		}
	}
	cast, err := castJSON(tv.Credits.Cast)
	if err != nil {
		return err
	}
	creators := make([]string, 0, len(tv.CreatedBy))
	for _, cr := range tv.CreatedBy {
		creators = append(creators, cr.Name)
	}
	runtime := 0
	if len(tv.EpisodeRunTime) > 0 {
		runtime = tv.EpisodeRunTime[0]
	}

	var winner uuid.UUID
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		err := q.UpdateItemMetadata(ctx, store.UpdateItemMetadataParams{
			ID: it.ID, TmdbID: ptr(int32(id)), ImdbID: opt(tv.ExternalIDs.ImdbID),
			Title: tv.Name, SortTitle: scanner.SortTitle(tv.Name), OriginalTitle: opt(tv.OriginalName),
			Year: optYear(tv.FirstAirDate), Overview: opt(tv.Overview), Tagline: opt(tv.Tagline),
			ReleaseDate: parseDate(tv.FirstAirDate), RuntimeMinutes: optInt(runtime), Rating: optRating(tv.VoteAverage),
			Genres: genreNames(tv.Genres), CastMembers: cast, Directors: creators,
			PosterPath: opt(tv.PosterPath), BackdropPath: opt(tv.BackdropPath),
			MatchStatus: status, MetadataLanguage: &lang,
		})
		if err != nil {
			return err
		}
		winner, err = mergeDuplicate(ctx, q, lib.ID, "series", int32(id), it.ID)
		return err
	})
	if err != nil {
		return err
	}
	return s.refreshSeasons(ctx, c, lang, id, winner)
}

func (s *Service) identifySeries(ctx context.Context, c *TMDB, lang string, it store.Item) (int, error) {
	paths, err := s.q.ListItemFilePaths(ctx, store.ListItemFilePathsParams{ItemID: it.ID, MaxPaths: nfoSamplePaths})
	if err != nil {
		return 0, err
	}
	if ids := findNFOIDs(paths); ids.TMDB > 0 && ids.Kind == "tvshow" {
		return ids.TMDB, nil
	} else if ids.IMDb != "" {
		found, err := c.FindIMDb(ctx, ids.IMDb)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return 0, err
		}
		// Release NFO files of a season usually link to the show, sometimes to an episode.
		if len(found.TVResults) > 0 {
			return found.TVResults[0].ID, nil
		}
		if len(found.TVEpisodeResults) > 0 {
			return found.TVEpisodeResults[0].ShowID, nil
		}
	}
	year := 0
	if it.Year != nil {
		year = int(*it.Year)
	}
	return s.search(searchRequest{
		titles: []string{it.Title},
		year:   year,
		find:   func(q string, y int) ([]SearchResult, error) { return c.SearchTV(ctx, q, y, lang) },
	})
}

type searchRequest struct {
	// titles holds the parsed title first, then alternatives found in file names.
	titles []string
	year   int
	find   func(query string, year int) ([]SearchResult, error)
	// runtime breaks ties between same-title candidates without a year (remakes); nil disables it.
	runtime     func(id int) (int, error)
	fileMinutes int
}

// search tries each title variant with the year, then without it, and returns the first acceptable candidate.
func (s *Service) search(req searchRequest) (int, error) {
	for _, q := range queryVariants(req.titles) {
		years := []int{req.year}
		if req.year > 0 {
			years = append(years, 0)
		}
		for _, y := range years {
			results, err := req.find(q, y)
			if err != nil {
				return 0, err
			}
			cands := rankResults(results, q, req.year)
			if len(cands) == 0 {
				continue
			}
			if req.year == 0 && req.runtime != nil && req.fileMinutes > 0 {
				return tieBreakByRuntime(cands, req)
			}
			return cands[0].id, nil
		}
	}
	return 0, nil
}

// queryVariants adds shorter forms of each title: before " - " (subtitle), each side of parentheses
// ("Original (Translated)") and Roman numerals as digits ("Cars Ii" → "Cars 2").
func queryVariants(titles []string) []string {
	var out []string
	add := func(q string) {
		if q = strings.TrimSpace(q); q != "" && !slices.Contains(out, q) {
			out = append(out, q)
		}
	}
	for _, title := range titles {
		add(title)
		add(romanToDigits(title))
		if before, _, ok := strings.Cut(title, " - "); ok {
			add(before)
		}
		if before, rest, ok := strings.Cut(title, "("); ok {
			add(before)
			if inside, _, ok := strings.Cut(rest, ")"); ok {
				add(inside)
			}
		}
	}
	return out
}

type candidate struct {
	id    int
	score int
	exact bool
}

// rankResults keeps candidates whose title shares all its words with the searched title (in either
// direction) and whose year is within one year when both are known. A year alone is never enough:
// "Alien 2 (1986)" must not match an unrelated film of 1986.
func rankResults(results []SearchResult, title string, year int) []candidate {
	key := titleKey(title)
	var out []candidate
	for i, r := range results[:min(len(results), 10)] {
		localized, original := r.Titles()
		lk, ok := titleKey(localized), titleKey(original)
		exact := lk == key || ok == key
		related := exact || wordsSubset(key, lk) || wordsSubset(lk, key) || wordsSubset(key, ok) || wordsSubset(ok, key)
		if !related {
			continue
		}
		ry := r.Year()
		diff := abs(ry - year)
		yearKnown := year > 0 && ry > 0
		if yearKnown && diff > 1 {
			continue
		}
		// Without a year, a partial title match is only trusted for TMDB's top result.
		if !exact && !yearKnown && i != 0 {
			continue
		}
		score := 0
		if exact {
			score += 4
		}
		if yearKnown && diff == 0 {
			score += 2
		} else if yearKnown {
			score++
		}
		if i == 0 {
			score++
		}
		out = append(out, candidate{id: r.ID, score: score, exact: exact})
	}
	slices.SortStableFunc(out, func(a, b candidate) int { return b.score - a.score })
	return out
}

const maxRuntimeChecks = 4

// tieBreakByRuntime picks, among candidates with the exact title, the one whose runtime is closest to the
// file duration ("Cendrillon" 1950 animated film vs the 2015 remake).
func tieBreakByRuntime(cands []candidate, req searchRequest) (int, error) {
	var exact []candidate
	for _, c := range cands {
		if c.exact {
			exact = append(exact, c)
		}
	}
	if len(exact) < 2 {
		return cands[0].id, nil
	}
	best, bestDiff := cands[0].id, -1
	for _, c := range exact[:min(len(exact), maxRuntimeChecks)] {
		minutes, err := req.runtime(c.id)
		if err != nil {
			return 0, err
		}
		if minutes <= 0 {
			continue
		}
		if d := abs(minutes - req.fileMinutes); bestDiff < 0 || d < bestDiff {
			best, bestDiff = c.id, d
		}
	}
	return best, nil
}

var romanNumerals = map[string]string{"ii": "2", "iii": "3", "iv": "4", "vi": "6", "vii": "7", "viii": "8", "ix": "9"}

// titleKey normalizes a title for comparison, including Roman numerals ("Cars II" equals "Cars 2").
func titleKey(title string) string {
	words := strings.Fields(scanner.MatchKey(title))
	for i, w := range words {
		if d, ok := romanNumerals[w]; ok {
			words[i] = d
		}
	}
	return strings.Join(words, " ")
}

func romanToDigits(title string) string {
	words := strings.Fields(title)
	for i, w := range words {
		if d, ok := romanNumerals[strings.ToLower(w)]; ok {
			words[i] = d
		}
	}
	return strings.Join(words, " ")
}

// wordsSubset reports whether every word of a appears in b.
func wordsSubset(a, b string) bool {
	aw, bw := strings.Fields(a), strings.Fields(b)
	if len(aw) == 0 || len(bw) == 0 {
		return false
	}
	for _, w := range aw {
		if !slices.Contains(bw, w) {
			return false
		}
	}
	return true
}

func (s *Service) refreshSeasons(ctx context.Context, c *TMDB, lang string, tvID int, seriesID uuid.UUID) error {
	seasons, err := s.q.ListChildren(ctx, &seriesID)
	if err != nil {
		return err
	}
	for _, season := range seasons {
		if season.SeasonNumber == nil {
			continue
		}
		episodes, err := s.q.ListChildren(ctx, &season.ID)
		if err != nil {
			return err
		}
		sn, err := c.Season(ctx, tvID, int(*season.SeasonNumber), lang)
		if errors.Is(err, ErrNotFound) {
			if err := s.markUnmatched(ctx, season.ID); err != nil {
				return err
			}
			for _, e := range episodes {
				if err := s.markUnmatched(ctx, e.ID); err != nil {
					return err
				}
			}
			continue
		}
		if err != nil {
			return err
		}
		err = s.q.UpdateItemMetadata(ctx, store.UpdateItemMetadataParams{
			ID: season.ID, Title: sn.Name, Overview: opt(sn.Overview), Year: optYear(sn.AirDate),
			ReleaseDate: parseDate(sn.AirDate), PosterPath: opt(sn.PosterPath),
			MatchStatus: "matched", MetadataLanguage: &lang,
		})
		if err != nil {
			return err
		}
		byNumber := make(map[int]Episode, len(sn.Episodes))
		for _, e := range sn.Episodes {
			byNumber[e.EpisodeNumber] = e
		}
		for _, e := range episodes {
			ep, ok := byNumber[int(ptrValue(e.EpisodeNumber))]
			if !ok {
				if err := s.markUnmatched(ctx, e.ID); err != nil {
					return err
				}
				continue
			}
			err := s.q.UpdateItemMetadata(ctx, store.UpdateItemMetadataParams{
				ID: e.ID, Title: ep.Name, Overview: opt(ep.Overview), Year: optYear(ep.AirDate),
				ReleaseDate: parseDate(ep.AirDate), RuntimeMinutes: optInt(ep.Runtime), Rating: optRating(ep.VoteAverage),
				ThumbPath: opt(ep.StillPath), MatchStatus: "matched", MetadataLanguage: &lang,
			})
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) markUnmatched(ctx context.Context, id uuid.UUID) error {
	if err := s.q.SetItemMatchStatus(ctx, store.SetItemMatchStatusParams{ID: id, MatchStatus: "unmatched"}); err != nil {
		return err
	}
	return s.q.SetChildrenMatchStatus(ctx, store.SetChildrenMatchStatusParams{ParentID: &id, MatchStatus: "unmatched"})
}

// mergeDuplicate folds item into an older item of the library matched to the same TMDB id, such as
// "Shingeki No Kyojin" and "L'Attaque des Titans". It returns the id that remains.
func mergeDuplicate(ctx context.Context, q *store.Queries, libraryID uuid.UUID, kind string, tmdbID int32, item uuid.UUID) (uuid.UUID, error) {
	other, err := q.FindOtherItemByTmdb(ctx, store.FindOtherItemByTmdbParams{LibraryID: libraryID, Kind: kind, TmdbID: &tmdbID, Exclude: item})
	if errors.Is(err, pgx.ErrNoRows) {
		return item, nil
	}
	if err != nil {
		return uuid.Nil, err
	}
	return other, mergeInto(ctx, q, other, item)
}

// mergeInto moves the children and files of loser under winner, merging children that exist on both sides.
func mergeInto(ctx context.Context, q *store.Queries, winner, loser uuid.UUID) error {
	children, err := q.ListChildren(ctx, &loser)
	if err != nil {
		return err
	}
	for _, c := range children {
		existing, err := q.FindChild(ctx, store.FindChildParams{ParentID: &winner, Kind: c.Kind, MatchKey: c.MatchKey})
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			if err := q.MoveItem(ctx, store.MoveItemParams{ID: c.ID, ParentID: &winner}); err != nil {
				return err
			}
		case err != nil:
			return err
		default:
			if err := mergeInto(ctx, q, existing, c.ID); err != nil {
				return err
			}
		}
	}
	if err := q.MoveItemFiles(ctx, store.MoveItemFilesParams{FromItem: &loser, ToItem: &winner}); err != nil {
		return err
	}
	return q.DeleteItem(ctx, loser)
}

// thumbnail extracts a frame at 10% of the video for libraries without online metadata.
func (s *Service) thumbnail(ctx context.Context, itemID uuid.UUID) error {
	src, err := s.q.GetItemThumbSource(ctx, &itemID)
	if errors.Is(err, pgx.ErrNoRows) {
		return s.q.SetItemMatchStatus(ctx, store.SetItemMatchStatusParams{ID: itemID, MatchStatus: "skipped"})
	}
	if err != nil {
		return err
	}
	at := 5.0
	if src.DurationSeconds != nil && *src.DurationSeconds > 0 {
		at = *src.DurationSeconds / 10
	}
	name := LocalThumbName(itemID)
	out := filepath.Join(s.cacheDir, name)
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, thumbTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, s.ffmpeg, "-v", "error", "-y",
		"-ss", strconv.FormatFloat(at, 'f', 1, 64), "-i", src.Path,
		"-frames:v", "1", "-vf", "scale=640:-2", "-q:v", "4", out)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("extract thumbnail: %v: %s", err, strings.TrimSpace(string(output)))
	}
	return s.q.SetItemThumb(ctx, store.SetItemThumbParams{ID: itemID, ThumbPath: &name, MatchStatus: "skipped"})
}

func LocalThumbName(id uuid.UUID) string {
	return "local/" + id.String() + ".jpg"
}

type castMember struct {
	Name        string `json:"name"`
	Character   string `json:"character,omitempty"`
	ProfilePath string `json:"profilePath,omitempty"`
}

func castJSON(cast []CastMember) ([]byte, error) {
	out := make([]castMember, 0, min(len(cast), maxCast))
	for _, c := range cast[:min(len(cast), maxCast)] {
		out = append(out, castMember{Name: c.Name, Character: c.Character, ProfilePath: c.ProfilePath})
	}
	return json.Marshal(out)
}

func genreNames(genres []Genre) []string {
	out := make([]string, 0, len(genres))
	for _, g := range genres {
		out = append(out, g.Name)
	}
	return out
}

func crewNames(c Credits, job string) []string {
	out := []string{}
	for _, m := range c.Crew {
		if m.Job == job {
			out = append(out, m.Name)
		}
	}
	return out
}

func ptr[T any](v T) *T {
	return &v
}

func ptrValue[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}

func opt(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func optInt(n int) *int32 {
	if n <= 0 {
		return nil
	}
	return ptr(int32(n))
}

func optYear(date string) *int32 {
	return optInt(yearOf(date))
}

func optRating(v float64) *float32 {
	if v <= 0 {
		return nil
	}
	return ptr(float32(v))
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
