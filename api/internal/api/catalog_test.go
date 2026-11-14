package api

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

func writeFiles(t *testing.T, root string, files ...string) {
	t.Helper()
	for _, f := range files {
		p := filepath.Join(root, f)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func (e testEnv) createScannedLibrary(t *testing.T, token, name, kind, dir string, extra map[string]any) string {
	t.Helper()
	body := map[string]any{"name": name, "kind": kind, "paths": []string{filepath.Join(e.root, dir)}, "restricted": false}
	for k, v := range extra {
		body[k] = v
	}
	r := do(t, e.h, http.MethodPost, "/api/v1/libraries", token, body)
	if r.status != http.StatusCreated {
		t.Fatalf("create library: %d %v", r.status, r.body)
	}
	id := r.body["id"].(string)
	if err := e.scan.ScanLibrary(context.Background(), uuid.MustParse(id)); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestCatalogBrowsing(t *testing.T) {
	e := newTestEnv(t)
	admin := setupAdmin(t, e.h).body["accessToken"].(string)
	writeFiles(t, e.root,
		"films/Alien.1979.mkv",
		"films/Blade.Runner.1982.mkv",
		"films/Blade.Runner.1982.Final.Cut.mkv",
		"films/[ygg]Guerre et Paix.1966.DVD 1.MULTI/Guerre et Paix.avi",
		"films/[ygg]Guerre et Paix.1966.DVD 2.MULTI/Guerre et Paix.avi",
		"series/Show.S01E01.mkv",
		"series/Show.S01E02.mkv",
		"series/Show.S02E01.mkv",
	)
	films := e.createScannedLibrary(t, admin, "Films", "movies", "films", nil)
	series := e.createScannedLibrary(t, admin, "Séries", "series", "series", nil)

	r := do(t, e.h, http.MethodGet, "/api/v1/libraries/"+films+"/items?sort=year", admin, nil)
	items := r.body["items"].([]any)
	if r.status != http.StatusOK || len(items) != 3 || r.body["total"].(float64) != 3 {
		t.Fatalf("list films: %d %v", r.status, r.body)
	}
	if first := items[0].(map[string]any); first["title"] != "Blade Runner" {
		t.Errorf("sort=year must put the most recent first, got %v", first["title"])
	}
	r = do(t, e.h, http.MethodGet, "/api/v1/libraries/"+films+"/items?query=alien", admin, nil)
	if len(r.body["items"].([]any)) != 1 {
		t.Errorf("query must filter by title: %v", r.body)
	}
	if r = do(t, e.h, http.MethodGet, "/api/v1/libraries/"+films+"/items?limit=1&offset=1", admin, nil); len(r.body["items"].([]any)) != 1 || r.body["total"].(float64) != 3 {
		t.Errorf("pagination: %v", r.body)
	}
	if r = do(t, e.h, http.MethodGet, "/api/v1/libraries/"+films+"/items?sort=drop", admin, nil); r.status != http.StatusBadRequest {
		t.Errorf("invalid sort: expected 400, got %d", r.status)
	}

	byTitle := map[string]string{}
	for _, it := range do(t, e.h, http.MethodGet, "/api/v1/libraries/"+films+"/items", admin, nil).body["items"].([]any) {
		m := it.(map[string]any)
		byTitle[m["title"].(string)] = m["id"].(string)
	}

	blade := do(t, e.h, http.MethodGet, "/api/v1/items/"+byTitle["Blade Runner"], admin, nil).body
	versions := blade["versions"].([]any)
	if len(versions) != 2 {
		t.Fatalf("Blade Runner must expose two versions: %v", blade)
	}
	labels := map[string]bool{}
	for _, v := range versions {
		vm := v.(map[string]any)
		labels[vm["label"].(string)] = true
		if len(vm["audio"].([]any)) != 2 || len(vm["subtitles"].([]any)) != 1 {
			t.Errorf("tracks missing: %v", vm)
		}
		if path, _ := vm["path"].(string); filepath.Dir(path) != filepath.Join(e.root, "films") {
			t.Errorf("admins must see the file path, got %q", path)
		}
	}
	if blade["folder"] != filepath.Join(e.root, "films") {
		t.Errorf("admins must see the folder of the item, got %v", blade["folder"])
	}
	if !labels["1080p · HDR · HEVC"] || !labels["1080p · HDR · HEVC · Final Cut"] {
		t.Errorf("unexpected version labels: %v", labels)
	}

	do(t, e.h, http.MethodPost, "/api/v1/users", admin, map[string]any{"username": "bob", "password": "password123"})
	bob := do(t, e.h, http.MethodPost, "/api/v1/auth/login", "", map[string]any{"username": "bob", "password": "password123"}).body["accessToken"].(string)
	bobBlade := do(t, e.h, http.MethodGet, "/api/v1/items/"+byTitle["Blade Runner"], bob, nil).body
	if _, leaked := bobBlade["folder"]; leaked {
		t.Error("folders must not be exposed to regular users")
	}
	for _, v := range bobBlade["versions"].([]any) {
		if _, leaked := v.(map[string]any)["path"]; leaked {
			t.Error("file paths must not be exposed to regular users")
		}
	}

	war := do(t, e.h, http.MethodGet, "/api/v1/items/"+byTitle["Guerre et Paix"], admin, nil).body
	if war["folder"] != filepath.Join(e.root, "films") {
		t.Errorf("a movie split across folders must report their common folder, got %v", war["folder"])
	}
	if parts := war["children"].([]any); len(parts) != 2 || parts[0].(map[string]any)["partNumber"].(float64) != 1 {
		t.Errorf("Guerre et Paix must list its 2 parts: %v", war["children"])
	}

	show := do(t, e.h, http.MethodGet, "/api/v1/libraries/"+series+"/items", admin, nil).body["items"].([]any)[0].(map[string]any)
	if show["episodeCount"].(float64) != 3 {
		t.Errorf("series card must count episodes: %v", show)
	}
	showDetail := do(t, e.h, http.MethodGet, "/api/v1/items/"+show["id"].(string), admin, nil).body
	if showDetail["folder"] != filepath.Join(e.root, "series") {
		t.Errorf("a series must report the folder of its episodes, got %v", showDetail["folder"])
	}
	seasons := showDetail["children"].([]any)
	if len(seasons) != 2 {
		t.Fatalf("expected 2 seasons, got %v", seasons)
	}
	s1 := do(t, e.h, http.MethodGet, "/api/v1/items/"+seasons[0].(map[string]any)["id"].(string), admin, nil).body
	episodes := s1["children"].([]any)
	if len(episodes) != 2 || len(s1["ancestors"].([]any)) != 1 {
		t.Fatalf("season 1: %v", s1)
	}
	ep := do(t, e.h, http.MethodGet, "/api/v1/items/"+episodes[1].(map[string]any)["id"].(string), admin, nil).body
	if ep["episodeNumber"].(float64) != 2 || len(ep["ancestors"].([]any)) != 2 || len(ep["versions"].([]any)) != 1 {
		t.Errorf("episode 2: %v", ep)
	}
}

func TestLibraryAccessControl(t *testing.T) {
	e := newTestEnv(t)
	admin := setupAdmin(t, e.h).body["accessToken"].(string)
	writeFiles(t, e.root, "kids/Bambi.1942.mkv", "adults/Alien.1979.mkv")

	bobID := do(t, e.h, http.MethodPost, "/api/v1/users", admin, map[string]any{"username": "bob", "password": "password123"}).body["id"].(string)
	do(t, e.h, http.MethodPost, "/api/v1/users", admin, map[string]any{"username": "carol", "password": "password123"})
	bob := do(t, e.h, http.MethodPost, "/api/v1/auth/login", "", map[string]any{"username": "bob", "password": "password123"}).body["accessToken"].(string)
	carol := do(t, e.h, http.MethodPost, "/api/v1/auth/login", "", map[string]any{"username": "carol", "password": "password123"}).body["accessToken"].(string)

	e.createScannedLibrary(t, admin, "Enfants", "movies", "kids", map[string]any{"restricted": false})
	adults := e.createScannedLibrary(t, admin, "Adultes", "movies", "adults", map[string]any{"restricted": true, "userIds": []string{bobID}})

	visible := func(token string) int {
		return len(do(t, e.h, http.MethodGet, "/api/v1/libraries", token, nil).body["items"].([]any))
	}
	if visible(admin) != 2 || visible(bob) != 2 || visible(carol) != 1 {
		t.Fatalf("visibility: admin=%d bob=%d carol=%d", visible(admin), visible(bob), visible(carol))
	}

	alien := do(t, e.h, http.MethodGet, "/api/v1/libraries/"+adults+"/items", bob, nil).body["items"].([]any)[0].(map[string]any)["id"].(string)
	for _, p := range []string{"/api/v1/libraries/" + adults, "/api/v1/libraries/" + adults + "/items", "/api/v1/items/" + alien} {
		if r := do(t, e.h, http.MethodGet, p, carol, nil); r.status != http.StatusNotFound {
			t.Errorf("carol %s: expected 404, got %d", p, r.status)
		}
		if r := do(t, e.h, http.MethodGet, p, bob, nil); r.status != http.StatusOK {
			t.Errorf("bob %s: expected 200, got %d", p, r.status)
		}
	}

	r := do(t, e.h, http.MethodPatch, "/api/v1/libraries/"+adults, admin, map[string]any{"restricted": false})
	if r.status != http.StatusOK || r.body["restricted"] != false || len(r.body["userIds"].([]any)) != 1 {
		t.Fatalf("unrestrict must keep the user list: %d %v", r.status, r.body)
	}
	if visible(carol) != 2 {
		t.Error("carol must see the library once it is no longer restricted")
	}
	if r = do(t, e.h, http.MethodPatch, "/api/v1/libraries/"+adults, admin, map[string]any{"userIds": []string{uuid.NewString()}}); r.status != http.StatusBadRequest {
		t.Errorf("unknown user: expected 400, got %d", r.status)
	}
}

func TestItemDirectorCredits(t *testing.T) {
	e := newTestEnv(t)
	admin := setupAdmin(t, e.h).body["accessToken"].(string)
	writeFiles(t, e.root, "films/Alien.1979.mkv", "films/Blade.Runner.1982.mkv")
	films := e.createScannedLibrary(t, admin, "Films", "movies", "films", nil)
	_, err := e.srv.Pool.Exec(context.Background(), `UPDATE items SET directors = '{Ridley Scott}',
		director_members = '[{"name": "Ridley Scott", "profilePath": "/rs.jpg"}]' WHERE library_id = $1 AND title = 'Alien'`, films)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]string{}
	for _, it := range do(t, e.h, http.MethodGet, "/api/v1/libraries/"+films+"/items", admin, nil).body["items"].([]any) {
		m := it.(map[string]any)
		ids[m["title"].(string)] = m["id"].(string)
	}

	r := do(t, e.h, http.MethodGet, "/api/v1/items/"+ids["Alien"], admin, nil)
	credits, _ := r.body["directorCredits"].([]any)
	if r.status != http.StatusOK || len(credits) != 1 {
		t.Fatalf("director credits: %d %v", r.status, r.body["directorCredits"])
	}
	if c := credits[0].(map[string]any); c["name"] != "Ridley Scott" || c["photoUrl"] != "/api/v1/images/tmdb/w185/rs.jpg" {
		t.Errorf("director credit: %v", c)
	}
	// Titles not refreshed since director pictures were added still answer an empty list, not null.
	r = do(t, e.h, http.MethodGet, "/api/v1/items/"+ids["Blade Runner"], admin, nil)
	if credits, ok := r.body["directorCredits"].([]any); !ok || len(credits) != 0 {
		t.Errorf("missing director credits must be an empty list: %v", r.body["directorCredits"])
	}
	// The image proxy serves director pictures like cast pictures.
	if known, err := e.srv.q.IsImagePathReferenced(context.Background(), "/rs.jpg"); err != nil || !known {
		t.Errorf("director picture must be a referenced image: %v %v", known, err)
	}
}

func TestItemPeopleTMDBIDs(t *testing.T) {
	e := newTestEnv(t)
	admin := setupAdmin(t, e.h).body["accessToken"].(string)
	writeFiles(t, e.root, "films/Alien.1979.mkv")
	films := e.createScannedLibrary(t, admin, "Films", "movies", "films", nil)
	// Ridley Scott carries his ID; the cast was stored before IDs were kept and gets them from the filter labels,
	// except for a name two actors share.
	_, err := e.srv.Pool.Exec(context.Background(), `UPDATE items SET
		director_members = '[{"id": 578, "name": "Ridley Scott"}]',
		cast_members = '[{"name": "Sigourney Weaver", "character": "Ripley"}, {"name": "John Doe"}, {"name": "Nobody"}]',
		filter_labels = '{"actors": [{"id": "10205", "label": "Sigourney Weaver"}, {"id": "1", "label": "John Doe"}, {"id": "2", "label": "John Doe"}]}'
		WHERE library_id = $1`, films)
	if err != nil {
		t.Fatal(err)
	}
	id := do(t, e.h, http.MethodGet, "/api/v1/libraries/"+films+"/items", admin, nil).body["items"].([]any)[0].(map[string]any)["id"].(string)
	r := do(t, e.h, http.MethodGet, "/api/v1/items/"+id, admin, nil)
	if r.status != http.StatusOK {
		t.Fatalf("item: %d %v", r.status, r.body)
	}
	if c := r.body["directorCredits"].([]any)[0].(map[string]any); c["id"] != float64(578) {
		t.Errorf("director id: %v", c)
	}
	cast := r.body["cast"].([]any)
	if c := cast[0].(map[string]any); c["id"] != float64(10205) || c["character"] != "Ripley" {
		t.Errorf("actor id from the filter labels: %v", c)
	}
	for _, c := range cast[1:] {
		if _, ok := c.(map[string]any)["id"]; ok {
			t.Errorf("ambiguous or unknown actor must have no id: %v", c)
		}
	}
}

func TestItemLogoInheritedByEpisodes(t *testing.T) {
	e := newTestEnv(t)
	admin := setupAdmin(t, e.h).body["accessToken"].(string)
	writeFiles(t, e.root, "films/Alien.1979.mkv", "series/Show.S01E01.mkv")
	films := e.createScannedLibrary(t, admin, "Films", "movies", "films", nil)
	series := e.createScannedLibrary(t, admin, "Séries", "series", "series", nil)
	if _, err := e.srv.Pool.Exec(context.Background(), `UPDATE items SET logo_path = '/show-logo.png' WHERE library_id = $1 AND kind = 'series'`, series); err != nil {
		t.Fatal(err)
	}

	alien := do(t, e.h, http.MethodGet, "/api/v1/libraries/"+films+"/items", admin, nil).body["items"].([]any)[0].(map[string]any)
	if r := do(t, e.h, http.MethodGet, "/api/v1/items/"+alien["id"].(string), admin, nil); r.status != http.StatusOK || r.body["logoUrl"] != nil {
		t.Errorf("a title without logo answers null: %d %v", r.status, r.body["logoUrl"])
	}

	const want = "/api/v1/images/tmdb/w500/show-logo.png"
	show := do(t, e.h, http.MethodGet, "/api/v1/libraries/"+series+"/items", admin, nil).body["items"].([]any)[0].(map[string]any)
	detail := do(t, e.h, http.MethodGet, "/api/v1/items/"+show["id"].(string), admin, nil).body
	if detail["logoUrl"] != want {
		t.Errorf("series logo: %v", detail["logoUrl"])
	}
	season := do(t, e.h, http.MethodGet, "/api/v1/items/"+detail["children"].([]any)[0].(map[string]any)["id"].(string), admin, nil).body
	episode := do(t, e.h, http.MethodGet, "/api/v1/items/"+season["children"].([]any)[0].(map[string]any)["id"].(string), admin, nil).body
	if season["logoUrl"] != want || episode["logoUrl"] != want {
		t.Errorf("seasons and episodes show their series' logo: %v %v", season["logoUrl"], episode["logoUrl"])
	}
	if known, err := e.srv.q.IsImagePathReferenced(context.Background(), "/show-logo.png"); err != nil || !known {
		t.Errorf("logos must be referenced images: %v %v", known, err)
	}
}
