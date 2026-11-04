package metadata

import (
	"encoding/json"
	"strconv"

	"github.com/google/uuid"
	"toto/api/internal/store"
)

type FilterOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

func filterMetadata(itemID uuid.UUID, genres []Genre, cast []CastMember, creators, keywords []Genre, countries []string, language string, votes *int32) store.UpdateItemFiltersParams {
	params := store.UpdateItemFiltersParams{
		ID: itemID, GenreIds: []int32{}, ActorIds: []int32{}, CreatorIds: []int32{}, KeywordIds: []int32{},
		Countries: append([]string{}, countries...), OriginalLanguage: opt(language), VoteCount: votes,
	}
	labels := make(map[string][]FilterOption)
	add := func(category string, entries []Genre, ids *[]int32) {
		seen := make(map[int]bool)
		for _, entry := range entries {
			if entry.ID <= 0 || seen[entry.ID] {
				continue
			}
			seen[entry.ID] = true
			*ids = append(*ids, int32(entry.ID))
			labels[category] = append(labels[category], FilterOption{ID: strconv.Itoa(entry.ID), Label: entry.Name})
		}
	}
	actors := make([]Genre, 0, len(cast))
	for _, member := range cast {
		actors = append(actors, Genre{ID: member.ID, Name: member.Name})
	}
	add("genres", genres, &params.GenreIds)
	add("actors", actors, &params.ActorIds)
	add("creators", creators, &params.CreatorIds)
	add("keywords", keywords, &params.KeywordIds)
	for _, country := range countries {
		labels["countries"] = append(labels["countries"], FilterOption{ID: country, Label: country})
	}
	if language != "" {
		labels["languages"] = []FilterOption{{ID: language, Label: language}}
	}
	params.FilterLabels, _ = json.Marshal(labels)
	return params
}
