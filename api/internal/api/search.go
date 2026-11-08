package api

import (
	"cmp"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"sync"
	"unicode/utf8"

	"toto/api/internal/auth"
	"toto/api/internal/httpx"
)

const (
	// minSearchLength keeps a search from matching most of the catalog of every server.
	minSearchLength    = 3
	defaultSearchLimit = 12
	maxSearchLimit     = 50
)

// searchGroupDTO holds the best matches of one library kind.
type searchGroupDTO struct {
	Kind  string           `json:"kind"`
	Items []catalogCardDTO `json:"items"`
	// Total counts every matching title of the kind, beyond the returned items.
	Total int `json:"total"`
}

// search finds titles of every kind by name, on this server and on its paired peers.
func (s *Server) search(w http.ResponseWriter, r *http.Request) {
	raw := strings.TrimSpace(r.URL.Query().Get("query"))
	if utf8.RuneCountInString(raw) < minSearchLength {
		httpx.Error(w, http.StatusBadRequest, "validation", "query must be at least 3 characters")
		return
	}
	limit, err := intParam(r.URL.Query().Get("limit"), defaultSearchLimit, 1, maxSearchLimit)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "validation", "limit "+err.Error())
		return
	}

	ctx := r.Context()
	p, _ := auth.PrincipalFrom(ctx)
	serverName, err := s.Settings.ServerName(ctx)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	publicURL, err := s.Settings.ServerPublicURL(ctx)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	pattern := "%" + likeEscaper.Replace(raw) + "%"
	noFilters := json.RawMessage(`{}`)

	type kindResult struct {
		cards       []sourcedCard
		unavailable int
		err         error
	}
	results := make([]kindResult, len(libraryKinds))
	var wg sync.WaitGroup
	for i, kind := range libraryKinds {
		wg.Add(1)
		go func() {
			defer wg.Done()
			local, _, err := s.kindCards(ctx, p.UserID, p.Role == auth.RoleAdmin, kind, nil, pattern, "title", 0, allRows, noFilters)
			if err != nil {
				results[i].err = err
				return
			}
			cards := make([]sourcedCard, 0, len(local))
			for _, c := range local {
				cards = append(cards, sourcedCard{federatedCard: c, source: cardSourceDTO{
					ServerID: s.Identity.ID, ServerName: serverName, ServerURL: publicURL, ItemID: c.ID,
				}})
			}
			peerCards, unavailable, err := s.peerKindCards(ctx, p.UserID, kind, raw, "title", noFilters)
			results[i] = kindResult{cards: append(cards, peerCards...), unavailable: unavailable, err: err}
		}()
	}
	wg.Wait()

	groups := []searchGroupDTO{}
	unavailable := 0
	for i, kind := range libraryKinds {
		res := results[i]
		if res.err != nil {
			httpx.InternalError(w, r, res.err)
			return
		}
		// Each kind asks every peer, so an unreachable peer is counted once per kind.
		unavailable = max(unavailable, res.unavailable)
		slices.SortStableFunc(res.cards, func(a, b sourcedCard) int { return compareCards(a, b, "title") })
		cards := rankSearchResults(groupCards(res.cards), raw)
		if len(cards) == 0 {
			continue
		}
		groups = append(groups, searchGroupDTO{Kind: kind, Items: cards[:min(limit, len(cards))], Total: len(cards)})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"groups": groups, "unavailableServers": unavailable})
}

// rankSearchResults puts the titles that start with the query first and keeps the title order otherwise.
func rankSearchResults(cards []catalogCardDTO, query string) []catalogCardDTO {
	prefix := strings.ToLower(query)
	rank := func(c catalogCardDTO) int {
		if strings.HasPrefix(strings.ToLower(c.Title), prefix) {
			return 0
		}
		return 1
	}
	slices.SortStableFunc(cards, func(a, b catalogCardDTO) int { return cmp.Compare(rank(a), rank(b)) })
	return cards
}
