package api

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"toto/api/internal/httpx"
	"toto/api/internal/playback"
)

type playbackUserDTO struct {
	ID              uuid.UUID  `json:"id"`
	DisplayName     string     `json:"displayName"`
	AvatarUpdatedAt *time.Time `json:"avatarUpdatedAt,omitempty"`
}

type adminPlaybackSessionDTO struct {
	playback.SessionDiagnostic
	User *playbackUserDTO `json:"user"`
}

func (s *Server) listPlaybackSessions(w http.ResponseWriter, r *http.Request) {
	sessions := s.Playback.Diagnostics()
	items := make([]adminPlaybackSessionDTO, 0, len(sessions))
	if len(sessions) > 0 {
		seen := make(map[uuid.UUID]bool)
		var userIDs []uuid.UUID
		for _, session := range sessions {
			if !seen[session.UserID] {
				userIDs = append(userIDs, session.UserID)
				seen[session.UserID] = true
			}
		}
		rows, err := s.q.ListPlaybackUsers(r.Context(), userIDs)
		if err != nil {
			httpx.InternalError(w, r, err)
			return
		}
		users := make(map[uuid.UUID]*playbackUserDTO, len(rows))
		for _, row := range rows {
			users[row.ID] = &playbackUserDTO{
				ID: row.ID, DisplayName: aliasOr(row.Alias, row.Username), AvatarUpdatedAt: row.AvatarUpdatedAt,
			}
		}
		for _, session := range sessions {
			items = append(items, adminPlaybackSessionDTO{SessionDiagnostic: session, User: users[session.UserID]})
		}
	}

	w.Header().Set("Cache-Control", "no-store")
	httpx.JSON(w, http.StatusOK, struct {
		Items []adminPlaybackSessionDTO `json:"items"`
	}{Items: items})
}
