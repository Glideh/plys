package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"toto/api/internal/httpx"
	"toto/api/internal/scanner"
	"toto/api/internal/store"
)

// Names of the fields an administrator can edit, stored in items.locked_fields so that metadata refreshes keep them.
const (
	fieldTitle         = "title"
	fieldOriginalTitle = "originalTitle"
	fieldReleaseDate   = "releaseDate"
	fieldTagline       = "tagline"
	fieldOverview      = "overview"
	fieldDirectors     = "directors"
	fieldCast          = "cast"
	fieldPoster        = "poster"
)

const (
	maxEditTitle     = 300
	maxEditTagline   = 500
	maxEditOverview  = 10000
	maxEditPerson    = 200
	maxEditDirectors = 20
	maxEditCast      = 50

	maxPosterBytes  = 10 << 20
	maxPosterPixels = 25_000_000
	posterMaxWidth  = 1000
	posterMaxHeight = 1500
	posterQuality   = 90
)

var (
	errPosterFormat = errors.New("poster must be a JPEG or PNG image")
	errPosterPixels = errors.New("poster image dimensions are too large")
)

type castEditDTO struct {
	Name      string `json:"name"`
	Character string `json:"character"`
}

// editItemRequest holds the fields to change; absent fields keep their value and their lock.
type editItemRequest struct {
	Title         *string        `json:"title"`
	OriginalTitle *string        `json:"originalTitle"`
	ReleaseDate   *string        `json:"releaseDate"`
	Tagline       *string        `json:"tagline"`
	Overview      *string        `json:"overview"`
	Directors     *[]string      `json:"directors"`
	Cast          *[]castEditDTO `json:"cast"`
}

// editItem stores an administrator's changes to a movie, series or video and locks the changed fields.
func (s *Server) editItem(w http.ResponseWriter, r *http.Request) {
	it, ok := s.loadMatchableItem(w, r)
	if !ok {
		return
	}
	var req editItemRequest
	if !httpx.DecodeRequired(w, r, &req) {
		return
	}
	p := editParams(it)
	if msg := applyEdit(&p, it, req); msg != "" {
		httpx.Error(w, http.StatusBadRequest, "validation", msg)
		return
	}
	if err := s.q.EditItemMetadata(r.Context(), p); err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	s.respondItemSummary(w, r, it.ID)
}

// applyEdit merges the request into p and returns a validation message, or "" when the request is valid.
func applyEdit(p *store.EditItemMetadataParams, it store.Item, req editItemRequest) string {
	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" || utf8.RuneCountInString(title) > maxEditTitle {
			return "title must be 1 to 300 characters"
		}
		p.Title, p.SortTitle = title, scanner.SortTitle(title)
		lockField(p, fieldTitle)
	}
	if req.OriginalTitle != nil {
		v, ok := optionalText(*req.OriginalTitle, maxEditTitle)
		if !ok {
			return "originalTitle must be at most 300 characters"
		}
		p.OriginalTitle = v
		lockField(p, fieldOriginalTitle)
	}
	if req.ReleaseDate != nil {
		raw := strings.TrimSpace(*req.ReleaseDate)
		if raw == "" {
			p.ReleaseDate = nil
		} else {
			date, err := time.Parse(time.DateOnly, raw)
			if err != nil || date.Year() < 1800 || date.Year() > 2200 {
				return "releaseDate must be a date such as 2024-05-31"
			}
			year := int32(date.Year())
			p.ReleaseDate, p.Year = &date, &year
		}
		lockField(p, fieldReleaseDate)
	}
	if req.Tagline != nil {
		v, ok := optionalText(*req.Tagline, maxEditTagline)
		if !ok {
			return "tagline must be at most 500 characters"
		}
		p.Tagline = v
		lockField(p, fieldTagline)
	}
	if req.Overview != nil {
		v, ok := optionalText(*req.Overview, maxEditOverview)
		if !ok {
			return "overview must be at most 10000 characters"
		}
		p.Overview = v
		lockField(p, fieldOverview)
	}
	if req.Directors != nil {
		if len(*req.Directors) > maxEditDirectors {
			return "directors must hold at most 20 names"
		}
		names := make([]string, 0, len(*req.Directors))
		for _, n := range *req.Directors {
			n = strings.TrimSpace(n)
			if utf8.RuneCountInString(n) > maxEditPerson {
				return "a director name must be at most 200 characters"
			}
			if n != "" && !slices.Contains(names, n) {
				names = append(names, n)
			}
		}
		edits := make([]castEditDTO, 0, len(names))
		for _, n := range names {
			edits = append(edits, castEditDTO{Name: n})
		}
		members, err := mergePeople(it.DirectorMembers, edits)
		if err != nil {
			return "stored directors could not be read"
		}
		p.Directors, p.DirectorMembers = names, members
		lockField(p, fieldDirectors)
	}
	if req.Cast != nil {
		if len(*req.Cast) > maxEditCast {
			return "cast must hold at most 50 people"
		}
		edits := make([]castEditDTO, 0, len(*req.Cast))
		for _, c := range *req.Cast {
			c.Name, c.Character = strings.TrimSpace(c.Name), strings.TrimSpace(c.Character)
			if utf8.RuneCountInString(c.Name) > maxEditPerson || utf8.RuneCountInString(c.Character) > maxEditPerson {
				return "cast names and characters must be at most 200 characters"
			}
			if c.Name != "" {
				edits = append(edits, c)
			}
		}
		members, err := mergePeople(it.CastMembers, edits)
		if err != nil {
			return "stored cast could not be read"
		}
		p.CastMembers = members
		lockField(p, fieldCast)
	}
	return ""
}

// editParams starts an edit from the item's current values.
func editParams(it store.Item) store.EditItemMetadataParams {
	p := store.EditItemMetadataParams{
		ID: it.ID, Title: it.Title, SortTitle: it.SortTitle, OriginalTitle: it.OriginalTitle, Year: it.Year,
		ReleaseDate: it.ReleaseDate, Tagline: it.Tagline, Overview: it.Overview, Directors: it.Directors,
		DirectorMembers: it.DirectorMembers, CastMembers: it.CastMembers, PosterPath: it.PosterPath,
		LockedFields: slices.Clone(it.LockedFields),
	}
	if p.Directors == nil {
		p.Directors = []string{}
	}
	if len(p.DirectorMembers) == 0 {
		p.DirectorMembers = []byte("[]")
	}
	if len(p.CastMembers) == 0 {
		p.CastMembers = []byte("[]")
	}
	if p.LockedFields == nil {
		p.LockedFields = []string{}
	}
	return p
}

func lockField(p *store.EditItemMetadataParams, field string) {
	if !slices.Contains(p.LockedFields, field) {
		p.LockedFields = append(p.LockedFields, field)
	}
}

// optionalText trims v and maps an empty value to nil.
func optionalText(v string, max int) (*string, bool) {
	v = strings.TrimSpace(v)
	if utf8.RuneCountInString(v) > max {
		return nil, false
	}
	if v == "" {
		return nil, true
	}
	return &v, true
}

// mergePeople builds the stored people list from the edited names. A person already listed keeps their TMDB ID
// and picture, so their photo and their catalog link survive the edit.
func mergePeople(stored []byte, edits []castEditDTO) ([]byte, error) {
	var current []storedCast
	if len(stored) > 0 {
		if err := json.Unmarshal(stored, &current); err != nil {
			return nil, err
		}
	}
	type person struct {
		ID          int    `json:"id,omitempty"`
		Name        string `json:"name"`
		Character   string `json:"character,omitempty"`
		ProfilePath string `json:"profilePath,omitempty"`
	}
	out := make([]person, 0, len(edits))
	used := make([]bool, len(current))
	for _, e := range edits {
		p := person{Name: e.Name, Character: e.Character}
		for i, c := range current {
			if !used[i] && strings.EqualFold(c.Name, e.Name) {
				used[i] = true
				p.ID, p.ProfilePath = c.ID, c.ProfilePath
				break
			}
		}
		out = append(out, p)
	}
	return json.Marshal(out)
}

// putPoster replaces the poster of a movie or series with an uploaded image.
func (s *Server) putPoster(w http.ResponseWriter, r *http.Request) {
	it, ok := s.loadMatchableItem(w, r)
	if !ok {
		return
	}
	if it.Kind == "video" {
		httpx.Error(w, http.StatusBadRequest, "not_matchable", "only movies and series have a poster")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxPosterBytes)
	data, err := io.ReadAll(r.Body)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			httpx.Error(w, http.StatusRequestEntityTooLarge, "validation", "poster must be at most 10 MB")
			return
		}
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "could not read request body")
		return
	}
	poster, err := processPoster(data)
	if errors.Is(err, errPosterPixels) {
		httpx.Error(w, http.StatusBadRequest, "validation", err.Error())
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusUnsupportedMediaType, "validation", err.Error())
		return
	}
	// A new name per upload: image URLs are cached as immutable.
	name := "local/" + uuid.NewString() + ".jpg"
	if err := s.Images.SaveLocal(name, poster); err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	p := editParams(it)
	p.PosterPath = &name
	lockField(&p, fieldPoster)
	if err := s.q.EditItemMetadata(r.Context(), p); err != nil {
		_ = s.Images.RemoveLocal(name)
		httpx.InternalError(w, r, err)
		return
	}
	s.removeReplacedPoster(r.Context(), it.PosterPath, &name)
	s.respondItemSummary(w, r, it.ID)
}

// revertItemEdits unlocks the edited fields and refreshes the item, so that TMDB values (or the extracted
// thumbnail of a video) replace the edits.
func (s *Server) revertItemEdits(w http.ResponseWriter, r *http.Request) {
	it, ok := s.loadMatchableItem(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	// Checked first so that a missing key does not unlock the fields without refreshing them.
	if it.Kind != "video" {
		if _, _, err := s.Metadata.Client(ctx); s.writeMetadataError(w, r, err) {
			return
		}
	}
	if err := s.q.UnlockItemFields(ctx, it.ID); err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	if s.writeMetadataError(w, r, s.Metadata.RefreshItem(ctx, it.ID, nil)) {
		return
	}
	// A merge may have removed the item: its poster is then unused.
	var current *string
	if updated, err := s.q.GetItem(ctx, it.ID); err == nil {
		current = updated.PosterPath
	}
	s.removeReplacedPoster(ctx, it.PosterPath, current)
	s.respondItemSummary(w, r, it.ID)
}

// removeReplacedPoster deletes an uploaded poster that the item no longer uses.
func (s *Server) removeReplacedPoster(ctx context.Context, old, current *string) {
	if old == nil || !strings.HasPrefix(*old, "local/") || (current != nil && *current == *old) {
		return
	}
	if err := s.Images.RemoveLocal(*old); err != nil {
		slog.WarnContext(ctx, "remove replaced poster", "path", *old, "err", err)
	}
}

// processPoster accepts JPEG or PNG input (sniffed from the content) and returns a JPEG that fits in
// posterMaxWidth x posterMaxHeight, keeping its aspect ratio.
func processPoster(src []byte) ([]byte, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(src))
	if err != nil || (format != "jpeg" && format != "png") {
		return nil, errPosterFormat
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width*cfg.Height > maxPosterPixels {
		return nil, errPosterPixels
	}
	img, _, err := image.Decode(bytes.NewReader(src))
	if err != nil {
		return nil, errPosterFormat
	}
	b := img.Bounds()
	width, height := b.Dx(), b.Dy()
	if width > posterMaxWidth || height > posterMaxHeight {
		scale := min(float64(posterMaxWidth)/float64(width), float64(posterMaxHeight)/float64(height))
		width, height = max(1, int(float64(width)*scale)), max(1, int(float64(height)*scale))
	}
	out := image.NewRGBA(image.Rect(0, 0, width, height))
	scaleBox(out, img, b)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, out, &jpeg.Options{Quality: posterQuality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// respondItemSummary answers with the item after a change, or {"merged": true} when a refresh merged it away.
func (s *Server) respondItemSummary(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	updated, err := s.q.GetItem(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.JSON(w, http.StatusOK, map[string]any{"merged": true})
		return
	}
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toItemSummary(updated))
}
