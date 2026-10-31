package api

import (
	"net/http"
	"testing"
)

func (e testEnv) get(t *testing.T, token, path string) map[string]any {
	t.Helper()
	r := do(t, e.h, http.MethodGet, path, token, nil)
	if r.status != http.StatusOK {
		t.Fatalf("GET %s: %d %v", path, r.status, r.body)
	}
	return r.body
}

func (e testEnv) putProgress(t *testing.T, token, id string, position, duration float64) response {
	t.Helper()
	return do(t, e.h, http.MethodPut, "/api/v1/items/"+id+"/progress", token, map[string]any{"positionSeconds": position, "durationSeconds": duration})
}

func railTitles(body map[string]any) []string {
	var out []string
	for _, it := range body["items"].([]any) {
		out = append(out, it.(map[string]any)["title"].(string))
	}
	return out
}

func TestProgressContinueAndNextUp(t *testing.T) {
	e := newTestEnv(t)
	admin := setupAdmin(t, e.h).body["accessToken"].(string)
	writeFiles(t, e.root, "series/Show.S01E01.mkv", "series/Show.S01E02.mkv", "series/Show.S02E01.mkv", "films/Alien.1979.mkv")
	series := e.createScannedLibrary(t, admin, "Séries", "series", "series", nil)
	films := e.createScannedLibrary(t, admin, "Films", "movies", "films", nil)

	show := e.get(t, admin, "/api/v1/libraries/"+series+"/items")["items"].([]any)[0].(map[string]any)
	showID := show["id"].(string)
	var episodes []string
	for _, season := range e.get(t, admin, "/api/v1/items/"+showID)["children"].([]any) {
		for _, ep := range e.get(t, admin, "/api/v1/items/"+season.(map[string]any)["id"].(string))["children"].([]any) {
			episodes = append(episodes, ep.(map[string]any)["id"].(string))
		}
	}
	if len(episodes) != 3 {
		t.Fatalf("expected 3 episodes, got %v", episodes)
	}
	e1, e2, e3 := episodes[0], episodes[1], episodes[2]

	if got := railTitles(e.get(t, admin, "/api/v1/me/continue-watching")); len(got) != 0 {
		t.Fatalf("nothing started yet, got %v", got)
	}
	if target := e.get(t, admin, "/api/v1/items/"+showID)["playTarget"].(map[string]any); target["id"] != e1 || target["resume"] != false {
		t.Errorf("an unwatched series starts at its first episode: %v", target)
	}

	// Started: resumable and listed.
	if r := e.putProgress(t, admin, e1, 100, 7200); r.status != http.StatusOK || r.body["watched"] != false {
		t.Fatalf("progress: %d %v", r.status, r.body)
	}
	cont := e.get(t, admin, "/api/v1/me/continue-watching")["items"].([]any)
	if len(cont) != 1 || cont[0].(map[string]any)["id"] != e1 || cont[0].(map[string]any)["showTitle"] != "Show" || cont[0].(map[string]any)["positionSeconds"].(float64) != 100 {
		t.Fatalf("continue watching: %v", cont)
	}
	if d := e.get(t, admin, "/api/v1/items/"+e1); d["positionSeconds"].(float64) != 100 || d["watched"] != false {
		t.Errorf("episode detail must give the resume point: %v", d)
	}
	if target := e.get(t, admin, "/api/v1/items/"+showID)["playTarget"].(map[string]any); target["id"] != e1 || target["resume"] != true {
		t.Errorf("a started episode is resumed from the series: %v", target)
	}

	// Under the threshold a short sample is not offered for resuming.
	e.putProgress(t, admin, e2, 10, 7200)
	if got := e.get(t, admin, "/api/v1/me/continue-watching")["items"].([]any); len(got) != 1 {
		t.Errorf("10 s is not worth resuming: %v", got)
	}

	// Finished: leaves "continue", the next episode becomes "next up".
	if r := e.putProgress(t, admin, e1, 6600, 7200); r.body["watched"] != true {
		t.Fatalf("90%% must mark as watched: %v", r.body)
	}
	if got := e.get(t, admin, "/api/v1/me/continue-watching")["items"].([]any); len(got) != 0 {
		t.Errorf("a watched episode must leave continue watching: %v", got)
	}
	next := e.get(t, admin, "/api/v1/me/next-up")["items"].([]any)
	if len(next) != 1 || next[0].(map[string]any)["id"] != e2 {
		t.Fatalf("next up must be episode 2: %v", next)
	}
	if d := e.get(t, admin, "/api/v1/items/"+e1); d["watched"] != true || d["positionSeconds"].(float64) != 0 || d["next"].(map[string]any)["id"] != e2 {
		t.Errorf("watched episode detail: %v", d)
	}
	if target := e.get(t, admin, "/api/v1/items/"+showID)["playTarget"].(map[string]any); target["id"] != e2 || target["resume"] != false {
		t.Errorf("the series continues with episode 2: %v", target)
	}
	card := e.get(t, admin, "/api/v1/libraries/"+series+"/items")["items"].([]any)[0].(map[string]any)
	if card["unwatchedCount"].(float64) != 2 || card["watched"] != false {
		t.Errorf("series card: %v", card)
	}

	// Started again: shown in "continue" only.
	e.putProgress(t, admin, e2, 600, 7200)
	if got := e.get(t, admin, "/api/v1/me/next-up")["items"].([]any); len(got) != 0 {
		t.Errorf("an episode in progress is not duplicated in next up: %v", got)
	}

	// Another user starts from scratch.
	if r := do(t, e.h, http.MethodPost, "/api/v1/users", admin, map[string]any{"username": "alice", "password": "password123"}); r.status != http.StatusCreated {
		t.Fatalf("create user: %d %v", r.status, r.body)
	}
	alice := do(t, e.h, http.MethodPost, "/api/v1/auth/login", "", map[string]any{"username": "alice", "password": "password123", "tokenDelivery": "body"}).body["accessToken"].(string)
	if got := e.get(t, alice, "/api/v1/me/continue-watching")["items"].([]any); len(got) != 0 {
		t.Errorf("progress is per user: %v", got)
	}
	if d := e.get(t, alice, "/api/v1/items/"+e1); d["watched"] != false {
		t.Errorf("watched state is per user: %v", d)
	}

	// Marking a series watched covers all its episodes; unmarking resets them.
	if r := do(t, e.h, http.MethodPost, "/api/v1/items/"+showID+"/watched", admin, nil); r.status != http.StatusNoContent {
		t.Fatalf("mark watched: %d", r.status)
	}
	if d := e.get(t, admin, "/api/v1/items/"+showID); d["watched"] != true || d["playTarget"].(map[string]any)["rewatch"] != true {
		t.Errorf("a watched series is offered again from the start: %v", d)
	}
	if e.get(t, admin, "/api/v1/libraries/"+series+"/items")["items"].([]any)[0].(map[string]any)["watched"] != true {
		t.Error("series card must be watched")
	}
	if got := e.get(t, admin, "/api/v1/me/continue-watching")["items"].([]any); len(got) != 0 {
		t.Errorf("marking watched clears the resume points: %v", got)
	}
	if r := do(t, e.h, http.MethodDelete, "/api/v1/items/"+showID+"/watched", admin, nil); r.status != http.StatusNoContent {
		t.Fatalf("mark unwatched: %d", r.status)
	}
	if d := e.get(t, admin, "/api/v1/items/"+e3); d["watched"] != false {
		t.Errorf("unmarked episode: %v", d)
	}

	// Movies carry their own state.
	alien := e.get(t, admin, "/api/v1/libraries/"+films+"/items")["items"].([]any)[0].(map[string]any)["id"].(string)
	e.putProgress(t, admin, alien, 7000, 7200)
	if e.get(t, admin, "/api/v1/libraries/"+films+"/items")["items"].([]any)[0].(map[string]any)["watched"] != true {
		t.Error("movie card must be watched")
	}

	// Validation.
	for name, r := range map[string]response{
		"series":   e.putProgress(t, admin, showID, 10, 100),
		"negative": e.putProgress(t, admin, e1, -5, 100),
		"unknown":  e.putProgress(t, admin, "00000000-0000-0000-0000-000000000000", 1, 100),
	} {
		if r.status != http.StatusBadRequest && r.status != http.StatusNotFound {
			t.Errorf("%s: expected 400 or 404, got %d", name, r.status)
		}
	}
	if r := do(t, e.h, http.MethodPut, "/api/v1/items/"+e1+"/progress", "", map[string]any{"positionSeconds": 1}); r.status != http.StatusUnauthorized {
		t.Errorf("progress needs authentication: %d", r.status)
	}
}

func TestProgressRespectsLibraryAccess(t *testing.T) {
	e := newTestEnv(t)
	admin := setupAdmin(t, e.h).body["accessToken"].(string)
	writeFiles(t, e.root, "private/Bambi.1942.mkv")
	lib := e.createScannedLibrary(t, admin, "Private", "movies", "private", map[string]any{"restricted": true})
	item := e.get(t, admin, "/api/v1/libraries/"+lib+"/items")["items"].([]any)[0].(map[string]any)["id"].(string)

	if r := do(t, e.h, http.MethodPost, "/api/v1/users", admin, map[string]any{"username": "bob", "password": "password123"}); r.status != http.StatusCreated {
		t.Fatalf("create user: %d", r.status)
	}
	bob := do(t, e.h, http.MethodPost, "/api/v1/auth/login", "", map[string]any{"username": "bob", "password": "password123", "tokenDelivery": "body"}).body["accessToken"].(string)
	if r := e.putProgress(t, bob, item, 100, 7200); r.status != http.StatusNotFound {
		t.Errorf("a restricted item must look missing: %d", r.status)
	}
	e.putProgress(t, admin, item, 100, 7200)
	if got := e.get(t, bob, "/api/v1/me/continue-watching")["items"].([]any); len(got) != 0 {
		t.Errorf("bob must not see it: %v", got)
	}
}
