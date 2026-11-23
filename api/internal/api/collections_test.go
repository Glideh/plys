package api

import (
	"net/http"
	"slices"
	"testing"
)

func TestCollections(t *testing.T) {
	e := newTestEnv(t)
	admin := setupAdmin(t, e.h).body["accessToken"].(string)
	writeFiles(t, e.root,
		"films/Star.Wars.1977.mkv",
		"films/The.Empire.Strikes.Back.1980.mkv",
		"films/Return.of.the.Jedi.1983.mkv",
		"films/Alien.1979.mkv",
		"private/Rogue.One.2016.mkv",
		"series/Andor.S01E01.mkv",
	)
	films := e.createScannedLibrary(t, admin, "Films", "movies", "films", nil)
	private := e.createScannedLibrary(t, admin, "Private", "movies", "private", map[string]any{"restricted": true})
	series := e.createScannedLibrary(t, admin, "Series", "series", "series", nil)

	ids := map[string]string{}
	for _, lib := range []string{films, private, series} {
		for _, it := range do(t, e.h, http.MethodGet, "/api/v1/libraries/"+lib+"/items", admin, nil).body["items"].([]any) {
			card := it.(map[string]any)
			ids[card["title"].(string)] = card["id"].(string)
		}
	}
	starWars, empire, jedi, alien, rogue, andor := ids["Star Wars"], ids["The Empire Strikes Back"], ids["Return of the Jedi"], ids["Alien"], ids["Rogue One"], ids["Andor"]
	if starWars == "" || empire == "" || jedi == "" || alien == "" || rogue == "" || andor == "" {
		t.Fatalf("scanned titles: %v", ids)
	}

	do(t, e.h, http.MethodPost, "/api/v1/users", admin, map[string]any{"username": "bob", "password": "password123"})
	bob := do(t, e.h, http.MethodPost, "/api/v1/auth/login", "", map[string]any{"username": "bob", "password": "password123"}).body["accessToken"].(string)

	collection := func(id, token string) []string {
		t.Helper()
		r := do(t, e.h, http.MethodGet, "/api/v1/items/"+id, token, nil)
		if r.status != http.StatusOK {
			t.Fatalf("get item: %d %v", r.status, r.body)
		}
		titles := []string{}
		for _, c := range r.body["collection"].([]any) {
			titles = append(titles, c.(map[string]any)["title"].(string))
		}
		return titles
	}
	set := func(id string, members ...string) response {
		t.Helper()
		return do(t, e.h, http.MethodPut, "/api/v1/items/"+id+"/collection", admin, map[string]any{"itemIds": append([]string{}, members...)})
	}

	if got := collection(starWars, admin); len(got) != 0 {
		t.Fatalf("no collection yet: got %v", got)
	}
	if r := do(t, e.h, http.MethodPut, "/api/v1/items/"+starWars+"/collection", bob, map[string]any{"itemIds": []string{empire}}); r.status != http.StatusForbidden {
		t.Fatalf("set as user: expected 403, got %d", r.status)
	}
	for name, r := range map[string]response{
		"series member": set(starWars, andor),
		"series item":   set(andor, starWars),
		"unknown id":    set(starWars, "00000000-0000-0000-0000-000000000000"),
	} {
		if r.status != http.StatusBadRequest {
			t.Errorf("%s: expected 400, got %d %v", name, r.status, r.body)
		}
	}

	r := set(starWars, jedi, empire, rogue, starWars)
	if r.status != http.StatusOK || len(r.body["items"].([]any)) != 3 {
		t.Fatalf("set collection: %d %v", r.status, r.body)
	}
	// Release order, and titles of a library the user cannot read are left out.
	if got := collection(empire, admin); !slices.Equal(got, []string{"Star Wars", "Return of the Jedi", "Rogue One"}) {
		t.Errorf("admin sees %v", got)
	}
	if got := collection(empire, bob); !slices.Equal(got, []string{"Star Wars", "Return of the Jedi"}) {
		t.Errorf("bob sees %v", got)
	}

	// Editing from another member replaces the whole collection.
	if r := set(jedi, starWars, empire); r.status != http.StatusOK {
		t.Fatalf("edit collection: %d %v", r.status, r.body)
	}
	if got := collection(rogue, admin); len(got) != 0 {
		t.Errorf("rogue one left the collection, got %v", got)
	}
	if got := collection(starWars, admin); !slices.Equal(got, []string{"The Empire Strikes Back", "Return of the Jedi"}) {
		t.Errorf("star wars sees %v", got)
	}

	// A title taken from another collection moves, and the collection left with one title is dissolved.
	if r := set(alien, rogue); r.status != http.StatusOK {
		t.Fatal(r.status)
	}
	if r := set(starWars, empire, jedi, rogue); r.status != http.StatusOK {
		t.Fatal(r.status)
	}
	if got := collection(alien, admin); len(got) != 0 {
		t.Errorf("alien alone keeps no collection, got %v", got)
	}

	// An empty list takes the item out; two titles left keep their collection.
	if r := set(rogue); r.status != http.StatusOK || len(r.body["items"].([]any)) != 0 {
		t.Fatalf("leave collection: %d %v", r.status, r.body)
	}
	if got := collection(jedi, admin); !slices.Equal(got, []string{"Star Wars", "The Empire Strikes Back"}) {
		t.Errorf("after leaving, jedi sees %v", got)
	}
	if r := do(t, e.h, http.MethodGet, "/api/v1/items/"+jedi+"/collection", admin, nil); r.status != http.StatusOK || len(r.body["items"].([]any)) != 2 {
		t.Errorf("members: %d %v", r.status, r.body)
	}

	r = do(t, e.h, http.MethodGet, "/api/v1/items/"+starWars+"/collection/search?query=e", admin, nil)
	if r.status != http.StatusBadRequest {
		t.Errorf("short search: expected 400, got %d", r.status)
	}
	r = do(t, e.h, http.MethodGet, "/api/v1/items/"+starWars+"/collection/search?query=ro", admin, nil)
	found := []string{}
	for _, c := range r.body["items"].([]any) {
		found = append(found, c.(map[string]any)["title"].(string))
	}
	// Series are not offered, nor the item itself.
	if r.status != http.StatusOK || !slices.Equal(found, []string{"Rogue One"}) {
		t.Errorf("search: %d %v", r.status, found)
	}
}
