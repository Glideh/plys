package playback

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"time"

	"github.com/google/uuid"

	"toto/api/internal/scanner"
)

type SessionSource struct {
	Title            string          `json:"title"`
	Container        string          `json:"container"`
	Video            *scanner.Stream `json:"video,omitempty"`
	Audio            *scanner.Stream `json:"audio,omitempty"`
	Subtitle         *scanner.Stream `json:"subtitle,omitempty"`
	ExternalSubtitle bool            `json:"externalSubtitle"`
}

type ProcessState string

const (
	ProcessRunning  ProcessState = "running"
	ProcessBuffered ProcessState = "buffered"
	ProcessFinished ProcessState = "finished"
	ProcessFailed   ProcessState = "failed"
)

type SessionDiagnostic struct {
	ID               string         `json:"id"`
	UserID           uuid.UUID      `json:"-"`
	Source           SessionSource  `json:"source"`
	Caps             ClientCaps     `json:"caps"`
	Plan             Plan           `json:"plan"`
	Reasons          []Reason       `json:"reasons"`
	VideoAccel       Accel          `json:"videoAccel,omitempty"`
	VideoPipeline    *VideoPipeline `json:"videoPipeline,omitempty"`
	OutputAudioCodec string         `json:"outputAudioCodec,omitempty"`
	Process          ProcessState   `json:"process"`
	// Speed is the media time FFmpeg produces per second, set while it runs and has been measured.
	Speed      float64   `json:"speed,omitempty"`
	StartedAt  time.Time `json:"startedAt"`
	LastAccess time.Time `json:"lastAccess"`
}

func cloneStream(stream *scanner.Stream) *scanner.Stream {
	if stream == nil {
		return nil
	}
	value := *stream
	return &value
}

func sessionSource(req StartRequest) SessionSource {
	subtitle := req.SelectedSubtitle
	if subtitle == nil {
		subtitle = req.Source.Subtitle
	}
	if subtitle == nil {
		subtitle = req.Source.TextSubtitle
	}
	return SessionSource{
		Title: req.Title, Container: req.Container, Video: cloneStream(req.Source.Video),
		Audio: cloneStream(req.Source.Audio), Subtitle: cloneStream(subtitle),
		ExternalSubtitle: req.ExternalSubtitle != uuid.Nil,
	}
}

func cloneCaps(caps ClientCaps) ClientCaps {
	caps.VideoCodecs = append([]string{}, caps.VideoCodecs...)
	caps.AudioCodecs = append([]string{}, caps.AudioCodecs...)
	return caps
}

func (m *Manager) Diagnostics() []SessionDiagnostic {
	sessions := m.listSessions()
	items := make([]SessionDiagnostic, 0, len(sessions))
	for _, s := range sessions {
		items = append(items, s.diagnostic())
	}
	slices.SortFunc(items, func(a, b SessionDiagnostic) int {
		if order := b.StartedAt.Compare(a.StartedAt); order != 0 {
			return order
		}
		return cmp.Compare(a.ID, b.ID)
	})
	return items
}

func (s *Session) diagnostic() SessionDiagnostic {
	// The streaming ID grants access to the playlist; do not expose it on the dashboard.
	key := sha256.Sum256([]byte(s.ID))
	d := SessionDiagnostic{
		ID: hex.EncodeToString(key[:]), UserID: s.UserID, Source: s.source, Caps: cloneCaps(s.caps),
		Plan: s.Plan, Reasons: append([]Reason{}, s.reasons...), StartedAt: s.startedAt,
	}
	d.Source.Video = cloneStream(s.source.Video)
	d.Source.Audio = cloneStream(s.source.Audio)
	d.Source.Subtitle = cloneStream(s.source.Subtitle)
	if d.Source.Video != nil && !d.Plan.CopyVideo {
		d.VideoAccel = s.accel
		d.VideoPipeline = s.pipeline
	}
	if d.Source.Audio != nil && !d.Plan.CopyAudio {
		d.OutputAudioCodec = d.Plan.audioEncoder()
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	d.LastAccess = s.lastAccess
	select {
	case <-s.done:
		d.Process = ProcessFinished
		if s.failed {
			d.Process = ProcessFailed
		}
	default:
		d.Process = ProcessRunning
		if s.paused {
			d.Process = ProcessBuffered
		} else if speed, ok := s.progress.speed(); ok {
			d.Speed = speed
		}
	}
	return d
}
