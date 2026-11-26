package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"toto/api/internal/auth"
	"toto/api/internal/httpx"
	"toto/api/internal/playback"
	"toto/api/internal/scanner"
)

// A subtitle request waits this long for extraction before answering 202, so the player can retry.
const subtitleWait = 25 * time.Second

type startPlaybackRequest struct {
	VersionID     uuid.UUID `json:"versionId"`
	AudioIndex    *int      `json:"audioIndex"`
	SubtitleIndex *int      `json:"subtitleIndex"`
	// ExternalSubtitleID picks a downloaded subtitle instead of a track of the file.
	ExternalSubtitleID *uuid.UUID          `json:"externalSubtitleId"`
	Start              float64             `json:"start"`
	Caps               playback.ClientCaps `json:"caps"`
}

type playbackSessionDTO struct {
	ID            string  `json:"id"`
	PlaylistURL   string  `json:"playlistUrl"`
	Start         float64 `json:"start"`
	Duration      float64 `json:"duration"`
	CopyVideo     bool    `json:"copyVideo"`
	CopyAudio     bool    `json:"copyAudio"`
	Accel         string  `json:"accel"`
	AudioIndex    *int    `json:"audioIndex"`
	SubtitleIndex *int    `json:"subtitleIndex"`
	// ExternalSubtitleID is the downloaded subtitle served at SubtitleURL.
	ExternalSubtitleID *uuid.UUID `json:"externalSubtitleId"`
	// Set for text subtitles, served as WebVTT aligned on Start; image subtitles are burned into the video.
	SubtitleURL *string `json:"subtitleUrl"`
	Burned      bool    `json:"burnedSubtitle"`
}

func (s *Server) startPlayback(w http.ResponseWriter, r *http.Request) {
	var req startPlaybackRequest
	if !httpx.DecodeRequired(w, r, &req) {
		return
	}
	ctx := r.Context()
	f, err := s.q.GetPlaybackFile(ctx, req.VersionID)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, http.StatusNotFound, "not_found", "version not found or unreadable")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	allowed, err := s.canAccessLibrary(r, f.LibraryID)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	if !allowed {
		httpx.Error(w, http.StatusNotFound, "not_found", "version not found or unreadable")
		return
	}

	var streams []scanner.Stream
	if err := json.Unmarshal(f.Streams, &streams); err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	src := playback.Source{Path: f.Path}
	if f.DurationSeconds != nil {
		src.Duration = *f.DurationSeconds
	}
	var textSubs []int
	for i := range streams {
		st := &streams[i]
		switch st.Type {
		case "video":
			if src.Video == nil {
				src.Video = st
			}
		case "subtitle":
			if !st.ImageBased {
				textSubs = append(textSubs, st.Index)
			}
		}
	}
	if src.Audio, err = pickAudio(streams, req.AudioIndex); err != nil {
		httpx.Error(w, http.StatusBadRequest, "validation", err.Error())
		return
	}
	var textSub *int
	var selectedSubtitle *scanner.Stream
	if req.SubtitleIndex != nil && req.ExternalSubtitleID != nil {
		httpx.Error(w, http.StatusBadRequest, "validation", "subtitleIndex and externalSubtitleId are exclusive")
		return
	}
	if req.ExternalSubtitleID != nil {
		sub, err := s.q.GetExternalSubtitle(ctx, *req.ExternalSubtitleID)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && sub.FileID != f.ID) {
			httpx.Error(w, http.StatusBadRequest, "validation", "externalSubtitleId is not a subtitle of this version")
			return
		}
		if err != nil {
			httpx.InternalError(w, r, err)
			return
		}
	}
	if req.SubtitleIndex != nil {
		idx := slices.IndexFunc(streams, func(st scanner.Stream) bool { return st.Type == "subtitle" && st.Index == *req.SubtitleIndex })
		if idx < 0 {
			httpx.Error(w, http.StatusBadRequest, "validation", "subtitleIndex is not a subtitle track of this version")
			return
		}
		selectedSubtitle = &streams[idx]
		if selectedSubtitle.ImageBased {
			src.Subtitle = selectedSubtitle
		} else {
			textSub = req.SubtitleIndex
			// Without a cached extraction, the session writes the track while streaming.
			if _, cached := s.Subtitles.Cached(f.ID, f.Size, *textSub); !cached {
				src.TextSubtitle = selectedSubtitle
			}
		}
	}

	p, _ := auth.PrincipalFrom(ctx)
	start := playback.StartRequest{
		UserID: p.UserID, FileID: f.ID, Source: src, Size: f.Size, TextSubtitles: textSubs, Caps: req.Caps, Start: req.Start,
		Title: f.Title, Container: f.Container, SelectedSubtitle: selectedSubtitle,
	}
	if req.ExternalSubtitleID != nil {
		start.ExternalSubtitle = *req.ExternalSubtitleID
	}
	sess, err := s.Playback.Start(ctx, start)
	if errors.Is(err, playback.ErrBusy) {
		httpx.Error(w, http.StatusServiceUnavailable, "transcoder_busy", "too many videos are being transcoded, try again later")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}

	dto := playbackSessionDTO{
		ID: sess.ID, PlaylistURL: "/api/v1/stream/" + sess.ID + "/index.m3u8", Start: sess.Start, Duration: src.Duration,
		CopyVideo: sess.Plan.CopyVideo, CopyAudio: sess.Plan.CopyAudio, Accel: string(s.Playback.Encoder.VideoAccel(sess.Plan, src)),
		SubtitleIndex: req.SubtitleIndex, Burned: sess.Plan.BurnSubtitle,
	}
	if src.Audio != nil {
		dto.AudioIndex = &src.Audio.Index
	}
	if textSub != nil {
		u := "/api/v1/stream/" + sess.ID + "/subtitles/" + strconv.Itoa(*textSub) + ".vtt"
		dto.SubtitleURL = &u
		s.Subtitles.Prepare(f.ID, f.Size, f.Path, *textSub)
	}
	if req.ExternalSubtitleID != nil {
		u := "/api/v1/stream/" + sess.ID + "/subtitles/" + externalSubtitleTrack
		dto.SubtitleURL = &u
		dto.ExternalSubtitleID = req.ExternalSubtitleID
	}
	httpx.JSON(w, http.StatusCreated, dto)
}

// pickAudio returns the requested track, else the default one, else the first.
func pickAudio(streams []scanner.Stream, index *int) (*scanner.Stream, error) {
	var first, def *scanner.Stream
	for i := range streams {
		st := &streams[i]
		if st.Type != "audio" {
			continue
		}
		if index != nil && st.Index == *index {
			return st, nil
		}
		if first == nil {
			first = st
		}
		if st.Default && def == nil {
			def = st
		}
	}
	if index != nil {
		return nil, errors.New("audioIndex is not an audio track of this version")
	}
	if def != nil {
		return def, nil
	}
	return first, nil
}

func (s *Server) stopPlayback(w http.ResponseWriter, r *http.Request) {
	sess, err := s.Playback.Get(chi.URLParam(r, "id"))
	p, _ := auth.PrincipalFrom(r.Context())
	if err != nil || sess.UserID != p.UserID {
		httpx.Error(w, http.StatusNotFound, "not_found", "playback session not found")
		return
	}
	s.Playback.Stop(sess.ID)
	w.WriteHeader(http.StatusNoContent)
}

var streamContentTypes = map[string]string{
	".m3u8": "application/vnd.apple.mpegurl",
	".mp4":  "video/mp4",
	".m4s":  "video/iso.segment",
}

// streamFile is public: the 128-bit session id is the capability, since players cannot add bearer tokens
// to every segment request.
func (s *Server) streamFile(w http.ResponseWriter, r *http.Request) {
	sess, err := s.Playback.Get(chi.URLParam(r, "sid"))
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "not_found", "playback session not found")
		return
	}
	name := chi.URLParam(r, "file")
	path, err := sess.File(r.Context(), name)
	switch {
	case errors.Is(err, playback.ErrNotReady):
		w.Header().Set("Retry-After", "2")
		httpx.Error(w, http.StatusServiceUnavailable, "not_ready", "stream not ready yet")
		return
	case errors.Is(err, playback.ErrNotFound):
		httpx.Error(w, http.StatusNotFound, "not_found", "stream file not found")
		return
	case err != nil:
		return
	}
	ext := name[strings.LastIndex(name, "."):]
	w.Header().Set("Content-Type", streamContentTypes[ext])
	if ext == ".m3u8" {
		w.Header().Set("Cache-Control", "no-cache")
	} else {
		w.Header().Set("Cache-Control", "private, max-age=3600")
	}
	http.ServeFile(w, r, path)
}

func (s *Server) streamSubtitle(w http.ResponseWriter, r *http.Request) {
	sess, err := s.Playback.Get(chi.URLParam(r, "sid"))
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "not_found", "playback session not found")
		return
	}
	if chi.URLParam(r, "track") == externalSubtitleTrack {
		s.streamExternalSubtitle(w, r, sess)
		return
	}
	index, err := strconv.Atoi(strings.TrimSuffix(chi.URLParam(r, "track"), ".vtt"))
	if err != nil || !slices.Contains(sess.TextSubtitles, index) {
		httpx.Error(w, http.StatusNotFound, "not_found", "subtitle track not found")
		return
	}
	w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")

	// Until the full extraction is cached, the cues written by the session so far are served; the
	// header tells the player to fetch again later.
	if _, cached := s.Subtitles.Cached(sess.FileID, sess.SourceSize, index); !cached && sess.LiveSubtitleIndex == index {
		w.Header().Set("X-Subtitles-Complete", "false")
		if live, ok := sess.LiveSubtitle(); ok {
			http.ServeFile(w, r, live)
		} else {
			_, _ = w.Write([]byte("WEBVTT\n\n"))
		}
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), subtitleWait)
	defer cancel()
	path, err := s.Subtitles.Wait(ctx, sess.FileID, sess.SourceSize, sess.SourcePath, index)
	if errors.Is(err, playback.ErrNotReady) {
		// Extraction reads the whole file; the player polls until it is done.
		w.Header().Set("Retry-After", "3")
		w.WriteHeader(http.StatusAccepted)
		return
	}
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	f, err := os.Open(path)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	defer f.Close()
	w.Header().Set("X-Subtitles-Complete", "true")
	_ = playback.ShiftVTT(f, w, sess.Start)
}

// externalSubtitleTrack names the downloaded subtitle of a session among its subtitle files.
const externalSubtitleTrack = "external.vtt"

func (s *Server) streamExternalSubtitle(w http.ResponseWriter, r *http.Request, sess *playback.Session) {
	if sess.ExternalSubtitle == uuid.Nil {
		httpx.Error(w, http.StatusNotFound, "not_found", "subtitle track not found")
		return
	}
	vtt, err := s.q.GetExternalSubtitleVTT(r.Context(), sess.ExternalSubtitle)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, http.StatusNotFound, "not_found", "subtitle track not found")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Subtitles-Complete", "true")
	_ = playback.ShiftVTT(strings.NewReader(vtt), w, sess.Start)
}

func (s *Server) transcodingInfo(w http.ResponseWriter, _ *http.Request) {
	httpx.JSON(w, http.StatusOK, map[string]any{
		"accel":    s.Playback.Encoder.Accel,
		"device":   s.Playback.Encoder.Device,
		"sessions": s.Playback.Count(),
	})
}
