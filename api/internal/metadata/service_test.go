package metadata

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"toto/api/internal/scanner"
	"toto/api/internal/settings"
	"toto/api/internal/store"
	"toto/api/internal/testdb"
)

type fakeProber struct{}

func (fakeProber) Probe(context.Context, string) (scanner.ProbeResult, error) {
	return scanner.ProbeResult{Duration: 60, Container: "matroska,webm", Streams: []scanner.Stream{}}, nil
}

// fakeTMDB answers the few endpoints used by the service, keyed by path and query.
func fakeTMDB(t *testing.T) *httptest.Server {
	t.Helper()
	respond := func(w http.ResponseWriter, v any) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v)
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("api_key") != "good-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		q, lang := r.URL.Query().Get("query"), r.URL.Query().Get("language")
		switch {
		case r.URL.Path == "/configuration":
			respond(w, map[string]any{})
		case r.URL.Path == "/search/movie" && q == "Alien":
			respond(w, map[string]any{"results": []map[string]any{
				{"id": 999, "title": "Alien Nation", "original_title": "Alien Nation", "release_date": "1988-10-07"},
				{"id": 348, "title": "Alien, le huitième passager", "original_title": "Alien", "release_date": "1979-05-25"},
			}})
		case r.URL.Path == "/search/movie" && q == "Blade Runner":
			respond(w, map[string]any{"results": []map[string]any{
				{"id": 78, "title": "Blade Runner", "original_title": "Blade Runner", "release_date": "1982-06-25"},
			}})
		case r.URL.Path == "/search/movie":
			respond(w, map[string]any{"results": []any{}})
		case r.URL.Path == "/movie/348":
			title, overview := "Alien, le huitième passager", ""
			if lang == "en-US" {
				title, overview = "Alien", "In deep space, the crew of the Nostromo..."
			}
			respond(w, map[string]any{
				"id": 348, "title": title, "original_title": "Alien", "overview": overview,
				"release_date": "1979-05-25", "runtime": 117, "vote_average": 8.2, "poster_path": "/alien.jpg",
				"images": map[string]any{"logos": []map[string]any{
					{"file_path": "/alien-en.png", "iso_639_1": "en", "vote_average": 5.3},
					{"file_path": "/alien-fr.svg", "iso_639_1": "fr", "vote_average": 5.5},
					{"file_path": "/alien-fr.png", "iso_639_1": "fr", "vote_average": 5.1},
				}},
				"vote_count": 5000, "original_language": "en", "production_countries": []map[string]any{{"iso_3166_1": "GB", "name": "United Kingdom"}},
				"keywords": map[string]any{"keywords": []map[string]any{{"id": 123, "name": "space"}}},
				"genres":   []map[string]any{{"id": 27, "name": "Horreur"}, {"id": 878, "name": "Science-Fiction"}},
				"credits":  map[string]any{"cast": []map[string]any{{"id": 10205, "name": "Sigourney Weaver", "character": "Ripley", "profile_path": "/sw.jpg"}}, "crew": []map[string]any{{"id": 578, "job": "Director", "name": "Ridley Scott", "profile_path": "/rs.jpg"}}},
			})
		case r.URL.Path == "/movie/78":
			respond(w, map[string]any{"id": 78, "title": "Blade Runner", "overview": "Los Angeles, 2019.", "release_date": "1982-06-25", "imdb_id": "tt0083658"})
		case r.URL.Path == "/find/tt0083658":
			respond(w, map[string]any{"movie_results": []map[string]any{{"id": 78}}})
		case r.URL.Path == "/search/tv" && (q == "L Attaque des Titans" || q == "Shingeki No Kyojin"):
			respond(w, map[string]any{"results": []map[string]any{
				{"id": 1429, "name": "L'Attaque des Titans", "original_name": "Shingeki no Kyojin", "first_air_date": "2013-04-07"},
			}})
		case r.URL.Path == "/search/tv":
			respond(w, map[string]any{"results": []any{}})
		case r.URL.Path == "/tv/1429":
			respond(w, map[string]any{"id": 1429, "name": "L'Attaque des Titans", "overview": "Des titans...", "first_air_date": "2013-04-07", "poster_path": "/aot.jpg"})
		case r.URL.Path == "/tv/1429/season/1":
			respond(w, map[string]any{"season_number": 1, "name": "Saison 1", "episodes": []map[string]any{
				{"episode_number": 1, "name": "À toi, dans 2000 ans", "still_path": "/e1.jpg"},
				{"episode_number": 2, "name": "Ce jour-là"},
			}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

type env struct {
	pool *pgxpool.Pool
	q    *store.Queries
	svc  *Service
	scan *scanner.Scanner
	root string
}

func newEnv(t *testing.T, key string) env {
	t.Helper()
	pool := testdb.New(t, "test_metadata")
	srv := fakeTMDB(t)
	t.Cleanup(srv.Close)
	q := store.New(pool)
	st := settings.New(q)
	if err := st.SetTMDBKey(context.Background(), key); err != nil {
		t.Fatal(err)
	}
	svc := NewService(pool, st, t.TempDir(), "ffmpeg", srv.URL)
	return env{pool: pool, q: q, svc: svc, scan: scanner.New(pool, fakeProber{}, svc, 0), root: t.TempDir()}
}

func (e env) library(t *testing.T, kind string, files ...string) uuid.UUID {
	t.Helper()
	for _, f := range files {
		p := filepath.Join(e.root, f)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		content := "x"
		if strings.HasSuffix(f, ".nfo") {
			content = "Blade Runner\nIMDB: https://www.imdb.com/title/tt0083658/\n"
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ctx := context.Background()
	lib, err := e.q.CreateLibrary(ctx, store.CreateLibraryParams{Name: kind, Kind: kind})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.q.AddLibraryPath(ctx, store.AddLibraryPathParams{LibraryID: lib.ID, Path: e.root}); err != nil {
		t.Fatal(err)
	}
	if err := e.scan.ScanLibrary(ctx, lib.ID); err != nil {
		t.Fatal(err)
	}
	return lib.ID
}

type row struct {
	Kind, Title, Status string
	TMDB                *int32
	Overview            *string
	Files               int
}

func TestFilterMetadataKeepsFullCast(t *testing.T) {
	cast := make([]CastMember, 20)
	for index := range cast {
		cast[index] = CastMember{ID: index + 1, Name: "Actor"}
	}
	params := filterMetadata(uuid.New(), []Genre{{ID: 878, Name: "Science-fiction"}}, cast, nil, nil, []string{"GB"}, "en", nil)
	if len(params.ActorIds) != 20 || len(params.GenreIds) != 1 || params.GenreIds[0] != 878 {
		t.Fatalf("filter IDs: %+v", params)
	}
	if params.VoteCount != nil {
		t.Fatal("missing vote count must remain unknown")
	}
	var series TV
	if err := json.Unmarshal([]byte(`{"keywords":{"results":[{"id":42,"name":"time travel"}]}}`), &series); err != nil {
		t.Fatal(err)
	}
	if len(series.Keywords.Results) != 1 || series.Keywords.Results[0].ID != 42 {
		t.Fatalf("TV keywords: %+v", series.Keywords)
	}
}

func TestFilterMetadataBackfillPreservesManualMatch(t *testing.T) {
	env := newEnv(t, "good-key")
	library := env.library(t, "movies", "Alien.1979.mkv")
	var itemID uuid.UUID
	if err := env.pool.QueryRow(context.Background(), `SELECT id FROM items WHERE library_id = $1 AND kind = 'movie'`, library).Scan(&itemID); err != nil {
		t.Fatal(err)
	}
	item, err := env.q.GetItem(context.Background(), itemID)
	if err != nil {
		t.Fatal(err)
	}
	if item.VoteCount == nil || *item.VoteCount != 5000 || len(item.KeywordIds) != 1 || item.KeywordIds[0] != 123 || item.OriginalLanguage == nil || *item.OriginalLanguage != "en" || len(item.Countries) != 1 || item.Countries[0] != "GB" || item.EnrichmentVersion != 4 {
		t.Fatalf("missing enrichment: %+v", item)
	}
	if item.LogoPath == nil || *item.LogoPath != "/alien-fr.png" {
		t.Fatalf("logo in the metadata language expected: %v", item.LogoPath)
	}
	if _, err := env.pool.Exec(context.Background(), `UPDATE items SET enrichment_version = 3, match_status = 'manual', keyword_ids = '{}', director_members = '[]', logo_path = NULL WHERE id = $1`, itemID); err != nil {
		t.Fatal(err)
	}
	env.svc.BackfillFilters(context.Background())
	item, err = env.q.GetItem(context.Background(), itemID)
	if err != nil {
		t.Fatal(err)
	}
	if item.EnrichmentVersion != 4 || item.MatchStatus != "manual" || item.TmdbID == nil || *item.TmdbID != 348 || len(item.KeywordIds) != 1 {
		t.Fatalf("backfill: %+v", item)
	}
	// Titles enriched before logos (or SVG logos) were kept get one on the next pass.
	if item.LogoPath == nil || *item.LogoPath != "/alien-fr.png" {
		t.Fatalf("logo not backfilled: %v", item.LogoPath)
	}
	// Titles enriched before director pictures existed get them on the next pass.
	if !strings.Contains(string(item.DirectorMembers), `"profilePath": "/rs.jpg"`) {
		t.Fatalf("director pictures not backfilled: %s", item.DirectorMembers)
	}
}

func (e env) items(t *testing.T, libID uuid.UUID, kind string) []row {
	t.Helper()
	rows, err := e.pool.Query(context.Background(), `
		SELECT i.kind, i.title, i.match_status, i.tmdb_id, i.overview, (SELECT count(*) FROM media_files f WHERE f.item_id = i.id)
		FROM items i WHERE i.library_id = $1 AND i.kind = $2 ORDER BY i.sort_title`, libID, kind)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.Kind, &r.Title, &r.Status, &r.TMDB, &r.Overview, &r.Files); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	return out
}

func TestMoviesAreMatchedAndMerged(t *testing.T) {
	e := newEnv(t, "good-key")
	lib := e.library(t, "movies",
		"Alien.1979.1080p.BluRay.mkv",
		"Blade Runner/Blade.Runner.mkv",
		"Blade Runner/Blade.Runner.nfo",
		"Blade.Runner.1982.Final.Cut.2160p.mkv",
		"Totally.Unknown.Film.2001.mkv",
	)

	movies := e.items(t, lib, "movie")
	if len(movies) != 3 {
		t.Fatalf("expected Alien, Blade Runner (merged) and the unknown film, got %+v", movies)
	}
	byTitle := map[string]row{}
	for _, m := range movies {
		byTitle[m.Title] = m
	}
	alien := byTitle["Alien, le huitième passager"]
	if alien.TMDB == nil || *alien.TMDB != 348 || alien.Status != "matched" {
		t.Errorf("Alien must win over Alien Nation thanks to the year: %+v", alien)
	}
	if alien.Overview == nil || !strings.HasPrefix(*alien.Overview, "In deep space") {
		t.Errorf("empty localized overview must fall back to English: %+v", alien.Overview)
	}
	if br := byTitle["Blade Runner"]; br.TMDB == nil || *br.TMDB != 78 || br.Files != 2 {
		t.Errorf("both Blade Runner versions must end up on one movie: %+v", br)
	}
	if u := byTitle["Totally Unknown Film"]; u.Status != "unmatched" {
		t.Errorf("unknown film must be unmatched: %+v", u)
	}
}

func TestLibraryLanguageOverridesServerLanguage(t *testing.T) {
	e := newEnv(t, "good-key")
	ctx := context.Background()
	lib := e.library(t, "movies", "Alien.1979.1080p.BluRay.mkv")
	if m := e.items(t, lib, "movie"); len(m) != 1 || m[0].Title != "Alien, le huitième passager" {
		t.Fatalf("without an override the server language (fr-FR) applies: %+v", m)
	}

	english := "en-US"
	library, err := e.q.SetLibraryMetadataLanguage(ctx, store.SetLibraryMetadataLanguageParams{ID: lib, MetadataLanguage: &english})
	if err != nil {
		t.Fatal(err)
	}
	todo, err := e.q.ListItemsToMatch(ctx, store.ListItemsToMatchParams{LibraryID: lib, Language: english})
	if err != nil || len(todo) != 1 {
		t.Fatalf("the movie must be refreshed in the new language, got %+v (%v)", todo, err)
	}
	if err := e.svc.RefreshLibrary(ctx, library, func(int, int) {}); err != nil {
		t.Fatal(err)
	}
	m := e.items(t, lib, "movie")
	if len(m) != 1 || m[0].Title != "Alien" || m[0].Overview == nil || !strings.HasPrefix(*m[0].Overview, "In deep space") {
		t.Fatalf("the library override (en-US) must win: %+v", m)
	}
	results, err := e.svc.Search(ctx, library, "movie", "Alien", 1979)
	if err != nil || len(results) == 0 {
		t.Fatalf("search with the library language: %+v (%v)", results, err)
	}

	// Removing the override goes back to the server language.
	if library, err = e.q.SetLibraryMetadataLanguage(ctx, store.SetLibraryMetadataLanguageParams{ID: lib}); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.RefreshLibrary(ctx, library, func(int, int) {}); err != nil {
		t.Fatal(err)
	}
	if m := e.items(t, lib, "movie"); len(m) != 1 || m[0].Title != "Alien, le huitième passager" {
		t.Fatalf("without the override the server language applies again: %+v", m)
	}
}

func TestSeriesAreMatchedMergedAndEpisodesFilled(t *testing.T) {
	e := newEnv(t, "good-key")
	lib := e.library(t, "series",
		"L.Attaque.des.Titans.S01E01.mkv",
		"Shingeki No Kyojin (L'Attaque des Titans)/Shingeki.No.Kyojin.S01E02.mkv",
		"L.Attaque.des.Titans.S02E01.mkv",
	)

	series := e.items(t, lib, "series")
	if len(series) != 1 || series[0].Title != "L'Attaque des Titans" || *series[0].TMDB != 1429 {
		t.Fatalf("both folders must merge into one matched series: %+v", series)
	}
	episodes := e.items(t, lib, "episode")
	if len(episodes) != 3 {
		t.Fatalf("expected 3 episodes, got %+v", episodes)
	}
	statuses := map[string]int{}
	for _, ep := range episodes {
		statuses[ep.Status]++
	}
	if statuses["matched"] != 2 || statuses["unmatched"] != 1 {
		t.Errorf("season 1 episodes come from TMDB, season 2 is unknown: %+v", episodes)
	}

	// A second pass has nothing left to do.
	todo, err := e.q.ListItemsToMatch(context.Background(), store.ListItemsToMatchParams{LibraryID: lib, Language: settings.DefaultLanguage})
	if err != nil || len(todo) != 0 {
		t.Fatalf("expected nothing pending, got %+v (%v)", todo, err)
	}
}

func TestWithoutKeyItemsStayPending(t *testing.T) {
	e := newEnv(t, "")
	lib := e.library(t, "movies", "Alien.1979.1080p.BluRay.mkv")
	if m := e.items(t, lib, "movie"); len(m) != 1 || m[0].Status != "pending" {
		t.Fatalf("without a key the movie must wait for one: %+v", m)
	}
}

func TestManualMatchIsLocked(t *testing.T) {
	e := newEnv(t, "good-key")
	lib := e.library(t, "movies", "Totally.Unknown.Film.2001.mkv")
	m, err := e.pool.Query(context.Background(), `SELECT id FROM items WHERE library_id = $1 AND kind = 'movie'`, lib)
	if err != nil {
		t.Fatal(err)
	}
	var id uuid.UUID
	m.Next()
	if err := m.Scan(&id); err != nil {
		t.Fatal(err)
	}
	m.Close()

	tmdbID := 348
	if err := e.svc.RefreshItem(context.Background(), id, &tmdbID); err != nil {
		t.Fatal(err)
	}
	it, err := e.q.GetItem(context.Background(), id)
	if err != nil || it.MatchStatus != "manual" || *it.TmdbID != 348 {
		t.Fatalf("manual match must be stored and locked: %+v (%v)", it, err)
	}
}

func pick(results []SearchResult, title string, year int) (int, bool) {
	c := rankResults(results, title, year)
	if len(c) == 0 {
		return 0, false
	}
	return c[0].id, true
}

func TestBestLogo(t *testing.T) {
	lang := func(s string) *string { return &s }
	logos := []Image{
		{FilePath: "/none.png", VoteAverage: 9},
		{FilePath: "/en-low.png", Language: lang("en"), VoteAverage: 4},
		{FilePath: "/en-high.png", Language: lang("en"), VoteAverage: 6},
		{FilePath: "/de.png", Language: lang("de"), VoteAverage: 10},
		{FilePath: "/fr.svg", Language: lang("fr"), VoteAverage: 10},
		{FilePath: "/es.jpg", Language: lang("es"), VoteAverage: 10},
	}
	cases := []struct{ lang, want string }{
		{"fr-FR", "/fr.png"}, // an SVG logo is served as the PNG TMDB renders from it
		{"de-DE", "/de.png"},
		{"es-ES", "/en-high.png"}, // neither PNG nor SVG: English comes next
		{"en-US", "/en-high.png"},
		{"ja-JP", "/en-high.png"},
	}
	for _, c := range cases {
		if got := BestLogo(logos, c.lang); got != c.want {
			t.Errorf("BestLogo(%s) = %q, want %q", c.lang, got, c.want)
		}
	}
	if got := BestLogo(logos[:1], "fr-FR"); got != "/none.png" {
		t.Errorf("a logo without text is better than none: %q", got)
	}
	if got := BestLogo(nil, "fr-FR"); got != "" {
		t.Errorf("no logo: %q", got)
	}
	if got := imageLanguages("pt-BR"); got != "pt,en,null" {
		t.Errorf("imageLanguages(pt-BR) = %q", got)
	}
}

func TestRankResults(t *testing.T) {
	results := []SearchResult{
		{ID: 1, Title: "Fury", ReleaseDate: "2014-10-15"},
		{ID: 2, Title: "Fury", ReleaseDate: "1936-05-29"},
	}
	if id, ok := pick(results, "Fury", 1936); !ok || id != 2 {
		t.Errorf("year must select the 1936 film, got %d %v", id, ok)
	}
	if id, ok := pick(results, "Fury", 0); !ok || id != 1 {
		t.Errorf("without year the exact top result wins, got %d %v", id, ok)
	}
	if _, ok := pick([]SearchResult{{ID: 3, Title: "Something Else", ReleaseDate: "2001-01-01"}}, "Hold-Up Documentaire", 0); ok {
		t.Error("an unrelated top result without year must be rejected")
	}
	if _, ok := pick(results, "Fury", 1990); ok {
		t.Error("a year far from every result must be rejected")
	}

	// Real mismatches seen on the NAS: the year alone must not be enough.
	if _, ok := pick([]SearchResult{{ID: 4, Title: "The Falling", ReleaseDate: "1986-01-01"}}, "Alien 2", 1986); ok {
		t.Error(`"Alien 2" must not match an unrelated 1986 film`)
	}
	if _, ok := pick([]SearchResult{{ID: 5, Title: "Norm of the North: King Sized Adventure", ReleaseDate: "2019-01-01"}}, "Bac Nord", 2020); ok {
		t.Error(`"Bac Nord" must not match "Norm of the North"`)
	}
	if id, ok := pick([]SearchResult{{ID: 6, Title: "Aliens, le retour", OriginalTitle: "Aliens", ReleaseDate: "1986-07-18"}}, "le retour", 1986); !ok || id != 6 {
		t.Error(`the alternative title "le retour" must find "Aliens, le retour"`)
	}
	if id, ok := pick([]SearchResult{{ID: 7, Title: "Cars 2", ReleaseDate: "2011-06-11"}}, "Cars Ii", 0); !ok || id != 7 {
		t.Error("Roman numerals must match digits")
	}
}

func TestTieBreakByRuntime(t *testing.T) {
	runtimes := map[int]int{10: 105, 11: 74}
	req := searchRequest{fileMinutes: 72, runtime: func(id int) (int, error) { return runtimes[id], nil }}
	cands := []candidate{{id: 10, score: 5, exact: true}, {id: 11, score: 4, exact: true}}
	if id, err := tieBreakByRuntime(cands, req); err != nil || id != 11 {
		t.Errorf("the 74 min animated film must win over the 105 min remake, got %d (%v)", id, err)
	}
}

func TestMovieAltTitle(t *testing.T) {
	cases := map[string]string{
		"Alien 2 - 1986 - le retour - 1080 x264 dts fr.mkv":               "le retour",
		"John.Woo(1992).Hard.Boiled(A.Toute.Epreuve).MULTi.720p-x264.mkv": "Hard Boiled(A Toute Epreuve)",
		"Prisoners.2013.1080p.BluRay.mkv":                                 "",
	}
	for name, want := range cases {
		if got := scanner.ParseMovie(name).AltTitle; got != want {
			t.Errorf("AltTitle(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestParseNFO(t *testing.T) {
	kodi := `<?xml version="1.0" encoding="UTF-8"?><movie><title>Alien</title><uniqueid type="tmdb">348</uniqueid><uniqueid type="imdb" default="true">tt0078748</uniqueid></movie>`
	if ids, ok := parseNFO(kodi); !ok || ids.Kind != "movie" || ids.TMDB != 348 || ids.IMDb != "tt0078748" {
		t.Errorf("kodi nfo: %+v %v", ids, ok)
	}
	release := "  ▄▄▄ ASCII ART ▄▄▄\n  iMDB ......: https://www.imdb.com/title/tt0083658/\n"
	if ids, ok := parseNFO(release); !ok || ids.Kind != "" || ids.IMDb != "tt0083658" {
		t.Errorf("release nfo: %+v %v", ids, ok)
	}
	if _, ok := parseNFO("no ids here"); ok {
		t.Error("nfo without ids must be ignored")
	}
}

func TestEditedFieldsSurviveRefresh(t *testing.T) {
	e := newEnv(t, "good-key")
	lib := e.library(t, "movies", "Alien.1979.1080p.BluRay.mkv")
	ctx := context.Background()
	var id uuid.UUID
	if err := e.pool.QueryRow(ctx, `SELECT id FROM items WHERE library_id = $1 AND kind = 'movie'`, lib).Scan(&id); err != nil {
		t.Fatal(err)
	}
	_, err := e.pool.Exec(ctx, `UPDATE items SET title = 'Mon Alien', sort_title = 'mon alien', poster_path = 'local/custom.jpg',
		cast_members = '[{"name": "Moi"}]', release_date = '2000-01-01', year = 2000, overview = 'old',
		locked_fields = '{title,poster,cast,releaseDate}' WHERE id = $1`, id)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.svc.RefreshItem(ctx, id, nil); err != nil {
		t.Fatal(err)
	}
	it, err := e.q.GetItem(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if it.Title != "Mon Alien" || it.SortTitle != "mon alien" || *it.PosterPath != "local/custom.jpg" || string(it.CastMembers) != `[{"name": "Moi"}]` {
		t.Errorf("edited fields must be kept: %q %q %v %s", it.Title, it.SortTitle, *it.PosterPath, it.CastMembers)
	}
	if *it.Year != 2000 || it.ReleaseDate.Year() != 2000 {
		t.Errorf("edited release date must be kept: %v %v", *it.Year, it.ReleaseDate)
	}
	if it.Overview == nil || *it.Overview == "old" || it.RuntimeMinutes == nil || *it.RuntimeMinutes != 117 {
		t.Errorf("fields that were not edited must be refreshed: %v %v", it.Overview, it.RuntimeMinutes)
	}
}
