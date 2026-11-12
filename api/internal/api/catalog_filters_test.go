package api

import (
	"context"
	"net/http"
	"net/url"
	"testing"
)

func TestParseCatalogFilters(t *testing.T) {
	for _, raw := range []string{`null`, `[]`, `{"genres":[0]}`, `{"genres":[1],"excludeGenres":[1]}`, `{"resolution":"4k"}`, `{"audio":"dolby"}`, `{"resolution":2160}`, `{"yearMin":2020,"yearMax":2000}`, `{"ratingMin":11}`, `{"runtimeMin":90,"runtimeMax":80}`, `{"languages":["eng"]}`, `{"votesMin":-1}`, `{"unknown":true}`, `{} {}`} {
		if _, err := parseCatalogFilters(raw); err == nil {
			t.Errorf("accepted invalid filters: %s", raw)
		}
	}
	if got, err := parseCatalogFilters(`{"languages":["EN"],"countries":["fr"],"ratingMin":0}`); err != nil || string(got) != `{"languages":["en"],"countries":["FR"],"ratingMin":0}` {
		t.Fatalf("normalization: %s %v", got, err)
	}
	// Web fronts and peers up to 0.3 may still send genreMode: it is accepted, then dropped.
	if got, err := parseCatalogFilters(`{"genres":[27],"genreMode":"any","resolution":"1080p","audio":"atmos"}`); err != nil || string(got) != `{"genres":[27],"resolution":"1080p","audio":"atmos"}` {
		t.Fatalf("legacy genreMode and technical filters: %s %v", got, err)
	}
}

func TestAdvancedCatalogFilters(t *testing.T) {
	env := newTestEnv(t)
	admin := setupAdmin(t, env.h).body["accessToken"].(string)
	writeFiles(t, env.root, "films/Alien.1979.mkv", "films/Blade.Runner.1982.mkv")
	library := env.createScannedLibrary(t, admin, "Films", "movies", "films", nil)
	_, err := env.srv.Pool.Exec(context.Background(), `UPDATE items SET genre_ids = '{27,878}', actor_ids = '{10205}', creator_ids = '{578}', keyword_ids = '{123}', original_language = 'en', countries = '{GB,US}', vote_count = 5000, rating = 8.2, runtime_minutes = 117 WHERE library_id = $1 AND title = 'Alien'`, library)
	if err != nil {
		t.Fatal(err)
	}
	// Alien: UHD cropped to 3840x1600 with TrueHD Atmos and an AC-3 track; Blade Runner: 1920x800 with DTS-HD MA.
	_, err = env.srv.Pool.Exec(context.Background(), `UPDATE media_files f SET streams = CASE i.title
		WHEN 'Alien' THEN '[{"index":0,"type":"video","codec":"hevc","width":3840,"height":1600},{"index":1,"type":"audio","codec":"truehd","profile":"Dolby TrueHD + Dolby Atmos"},{"index":2,"type":"audio","codec":"ac3"}]'::jsonb
		ELSE '[{"index":0,"type":"video","codec":"h264","width":1920,"height":800},{"index":1,"type":"audio","codec":"dts","profile":"DTS-HD MA"},{"index":2,"type":"subtitle","codec":"subrip"}]'::jsonb END
		FROM items i WHERE i.id = f.item_id AND i.library_id = $1`, library)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		filters string
		total   int
	}{
		{`{}`, 2}, {`{"genres":[27]}`, 1}, {`{"genres":[27,18]}`, 1},
		{`{"genres":[27,18],"genreMode":"all"}`, 1},
		{`{"resolution":"2160p"}`, 1}, {`{"resolution":"1080p"}`, 1}, {`{"resolution":"720p"}`, 0},
		{`{"audio":"atmos"}`, 1}, {`{"audio":"truehd"}`, 1}, {`{"audio":"ac3"}`, 1}, {`{"audio":"dts"}`, 1},
		{`{"audio":"dtsHdMa"}`, 1}, {`{"audio":"eac3"}`, 0}, {`{"resolution":"2160p","audio":"dts"}`, 0},
		{`{"resolution":"1080p","audio":"dtsHdMa","yearMin":1980}`, 1},
		{`{"excludeGenres":[27]}`, 1}, {`{"genres":[878],"excludeGenres":[27]}`, 0},
		{`{"yearMin":1980}`, 1}, {`{"ratingMin":8.2,"votesMin":5000}`, 1},
		{`{"ratingMin":8,"votesMin":5001}`, 0}, {`{"runtimeMin":117,"runtimeMax":117}`, 1},
		{`{"actors":[10205],"creators":[578],"keywords":[123],"languages":["en"],"countries":["GB"]}`, 1},
		{`{"languages":["fr"]}`, 0},
	} {
		for _, endpoint := range []string{"/api/v1/libraries/" + library + "/items", "/api/v1/kinds/movies/items"} {
			t.Run(endpoint+test.filters, func(t *testing.T) {
				result := do(t, env.h, http.MethodGet, endpoint+"?limit=1&filters="+url.QueryEscape(test.filters), admin, nil)
				if result.status != http.StatusOK || result.body["total"] != float64(test.total) {
					t.Fatalf("got %d %v, want total %d", result.status, result.body, test.total)
				}
			})
		}
	}
	if result := do(t, env.h, http.MethodGet, "/api/v1/kinds/movies/items?filters="+url.QueryEscape(`{"ratingMin":11}`), admin, nil); result.status != http.StatusBadRequest {
		t.Fatalf("invalid filters: %d", result.status)
	}
	if result := do(t, env.h, http.MethodGet, "/api/v1/libraries/"+library+"/items?offset=100&filters="+url.QueryEscape(`{"genres":[27]}`), admin, nil); result.status != http.StatusOK || result.body["total"] != float64(1) || len(result.body["items"].([]any)) != 0 {
		t.Fatalf("filtered offset total: %d %v", result.status, result.body)
	}
}

// A series matches a technical filter as soon as one of its episodes does.
func TestTechnicalFiltersOnSeries(t *testing.T) {
	env := newTestEnv(t)
	admin := setupAdmin(t, env.h).body["accessToken"].(string)
	writeFiles(t, env.root, "series/Show.S01E01.mkv", "series/Show.S01E02.mkv", "series/Other.S01E01.mkv")
	library := env.createScannedLibrary(t, admin, "Séries", "series", "series", nil)
	_, err := env.srv.Pool.Exec(context.Background(), `UPDATE media_files SET streams = CASE WHEN path LIKE '%Show.S01E02%'
		THEN '[{"index":0,"type":"video","codec":"hevc","width":3840,"height":2160},{"index":1,"type":"audio","codec":"eac3","profile":"Dolby Digital Plus + Dolby Atmos"}]'::jsonb
		ELSE '[{"index":0,"type":"video","codec":"h264","width":1280,"height":720},{"index":1,"type":"audio","codec":"aac"}]'::jsonb END
		WHERE library_id = $1`, library)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		filters string
		total   int
	}{
		{`{}`, 2}, {`{"resolution":"2160p"}`, 1}, {`{"resolution":"720p"}`, 2}, {`{"audio":"atmos"}`, 1}, {`{"audio":"eac3"}`, 1},
		{`{"audio":"aac"}`, 2}, {`{"resolution":"SD"}`, 0},
	} {
		for _, endpoint := range []string{"/api/v1/libraries/" + library + "/items", "/api/v1/kinds/series/items"} {
			result := do(t, env.h, http.MethodGet, endpoint+"?filters="+url.QueryEscape(test.filters), admin, nil)
			if result.status != http.StatusOK || result.body["total"] != float64(test.total) {
				t.Errorf("%s %s: got %d %v, want total %d", endpoint, test.filters, result.status, result.body["total"], test.total)
			}
		}
	}
}

func TestCatalogFilterSuggestionsRespectAccess(t *testing.T) {
	env := newTestEnv(t)
	admin := setupAdmin(t, env.h).body["accessToken"].(string)
	writeFiles(t, env.root, "films/Alien.1979.mkv")
	library := env.createScannedLibrary(t, admin, "Private films", "movies", "films", map[string]any{"restricted": true})
	_, err := env.srv.Pool.Exec(context.Background(), `UPDATE items SET filter_labels = '{"genres":[{"id":"878","label":"Science-fiction"},{"id":"35","label":"Comédie"}],"actors":[{"id":"10205","label":"Sigourney Weaver"}],"countries":[{"id":"GB","label":"GB"}],"languages":[{"id":"en","label":"en"}]}' WHERE library_id = $1`, library)
	if err != nil {
		t.Fatal(err)
	}
	created := do(t, env.h, http.MethodPost, "/api/v1/users", admin, map[string]any{"username": "viewer", "password": "password123", "role": "user"})
	if created.status != http.StatusCreated {
		t.Fatalf("create viewer: %d %v", created.status, created.body)
	}
	viewer := do(t, env.h, http.MethodPost, "/api/v1/auth/login", "", map[string]any{"username": "viewer", "password": "password123"}).body["accessToken"].(string)
	path := "/api/v1/kinds/movies/filters?category=actors&query=Sigourney"
	allowed := do(t, env.h, http.MethodGet, path, admin, nil)
	if allowed.status != http.StatusOK {
		t.Fatalf("suggestions: %d %v", allowed.status, allowed.body)
	}
	options := allowed.body["options"].(map[string]any)
	if len(options["actors"].([]any)) != 1 {
		t.Fatalf("missing actor: %v", options)
	}
	denied := do(t, env.h, http.MethodGet, path, viewer, nil)
	if denied.status != http.StatusOK || len(denied.body["options"].(map[string]any)) != 0 {
		t.Fatalf("private metadata leaked: %v", denied.body)
	}
	denied = do(t, env.h, http.MethodGet, path+"&library="+library, viewer, nil)
	if denied.status != http.StatusNotFound {
		t.Fatalf("private library: %d", denied.status)
	}
	selected := do(t, env.h, http.MethodGet, "/api/v1/kinds/movies/filters?category=actors&selected=10205&query=other", admin, nil)
	if selected.status != http.StatusOK || len(selected.body["options"].(map[string]any)["actors"].([]any)) != 1 {
		t.Fatalf("selected labels: %v", selected.body)
	}
	for _, test := range []struct{ category, query, locale string }{{"countries", "royaume", "fr"}, {"languages", "anglais", "fr"}, {"languages", "english", "en"}, {"countries", "reino", "pt-BR"}, {"languages", "ingl", "pt-br"},
		// Accents are ignored, both in the database labels and in the localized names.
		{"genres", "comedie", "fr"}, {"countries", "ROYAUM%C3%89", "fr"}} {
		result := do(t, env.h, http.MethodGet, "/api/v1/kinds/movies/filters?category="+test.category+"&query="+test.query+"&locale="+test.locale, admin, nil)
		if result.status != http.StatusOK || len(result.body["options"].(map[string]any)[test.category].([]any)) != 1 {
			t.Fatalf("localized search: %v", result.body)
		}
	}
}
