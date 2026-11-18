package api

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	_ "image/jpeg"
	"image/png"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"toto/api/internal/auth"
	"toto/api/internal/httpx"
	"toto/api/internal/store"
)

const (
	avatarSize      = 256
	maxAvatarBytes  = 2 << 20
	maxAvatarPixels = 4096 * 4096
)

var (
	errAvatarFormat = errors.New("avatar must be a JPEG or PNG image")
	errAvatarPixels = errors.New("avatar image dimensions are too large")
)

// putAvatar replaces the caller's avatar. The upload is decoded, cropped to a square and stored as a PNG.
func (s *Server) putAvatar(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxAvatarBytes)
	data, err := io.ReadAll(r.Body)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			httpx.Error(w, http.StatusRequestEntityTooLarge, "validation", "avatar must be at most 2 MB")
			return
		}
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "could not read request body")
		return
	}
	avatar, err := processAvatar(data)
	if errors.Is(err, errAvatarPixels) {
		httpx.Error(w, http.StatusBadRequest, "validation", err.Error())
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusUnsupportedMediaType, "validation", err.Error())
		return
	}

	p, _ := auth.PrincipalFrom(r.Context())
	ctx := r.Context()
	now := time.Now()
	var user store.User
	err = pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		if err := q.UpsertUserAvatar(ctx, store.UpsertUserAvatarParams{UserID: p.UserID, Data: avatar}); err != nil {
			return err
		}
		u, err := q.SetUserAvatarUpdatedAt(ctx, store.SetUserAvatarUpdatedAtParams{ID: p.UserID, AvatarUpdatedAt: &now})
		user = u
		return err
	})
	s.writeAvatarResult(w, r, user, err)
}

// deleteAvatar removes the caller's avatar and returns the updated user.
func (s *Server) deleteAvatar(w http.ResponseWriter, r *http.Request) {
	p, _ := auth.PrincipalFrom(r.Context())
	ctx := r.Context()
	var user store.User
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		if err := q.DeleteUserAvatar(ctx, p.UserID); err != nil {
			return err
		}
		u, err := q.SetUserAvatarUpdatedAt(ctx, store.SetUserAvatarUpdatedAtParams{ID: p.UserID})
		user = u
		return err
	})
	s.writeAvatarResult(w, r, user, err)
}

func (s *Server) writeAvatarResult(w http.ResponseWriter, r *http.Request, user store.User, err error) {
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "user no longer exists")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toUserDTO(user))
}

// userAvatar serves an avatar by user ID. It is public like other images; the UUID is not guessable.
func (s *Server) userAvatar(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "not_found", "avatar not found")
		return
	}
	data, err := s.q.GetUserAvatarData(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, http.StatusNotFound, "not_found", "avatar not found")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", imageCacheControl)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(data)
}

// processAvatar accepts JPEG or PNG input (sniffed from the content, not the declared type)
// and returns a 256x256 PNG of the centered square.
func processAvatar(src []byte) ([]byte, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(src))
	if err != nil || (format != "jpeg" && format != "png") {
		return nil, errAvatarFormat
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width*cfg.Height > maxAvatarPixels {
		return nil, errAvatarPixels
	}
	img, _, err := image.Decode(bytes.NewReader(src))
	if err != nil {
		return nil, errAvatarFormat
	}

	b := img.Bounds()
	side := min(b.Dx(), b.Dy())
	crop := image.Rect(
		b.Min.X+(b.Dx()-side)/2,
		b.Min.Y+(b.Dy()-side)/2,
		b.Min.X+(b.Dx()-side)/2+side,
		b.Min.Y+(b.Dy()-side)/2+side,
	)
	out := image.NewRGBA(image.Rect(0, 0, avatarSize, avatarSize))
	scaleBox(out, img, crop)

	var buf bytes.Buffer
	if err := png.Encode(&buf, out); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// scaleBox downsamples (or upsamples) the crop area into dst by averaging each destination pixel's source area.
func scaleBox(dst *image.RGBA, src image.Image, crop image.Rectangle) {
	w, h := dst.Bounds().Dx(), dst.Bounds().Dy()
	for y := 0; y < h; y++ {
		y0, y1 := sourceSpan(crop.Min.Y, y, h, crop.Dy(), crop.Max.Y)
		for x := 0; x < w; x++ {
			x0, x1 := sourceSpan(crop.Min.X, x, w, crop.Dx(), crop.Max.X)
			var r, g, b, a, count uint64
			for sy := y0; sy < y1; sy++ {
				for sx := x0; sx < x1; sx++ {
					cr, cg, cb, ca := src.At(sx, sy).RGBA()
					r += uint64(cr)
					g += uint64(cg)
					b += uint64(cb)
					a += uint64(ca)
					count++
				}
			}
			dst.SetRGBA64(x, y, color.RGBA64{R: uint16(r / count), G: uint16(g / count), B: uint16(b / count), A: uint16(a / count)})
		}
	}
}

// sourceSpan maps destination index i (of n) onto the source range [start, end), covering at least one pixel.
func sourceSpan(origin, i, n, side, limit int) (int, int) {
	start := origin + i*side/n
	end := origin + (i+1)*side/n
	if end <= start {
		end = start + 1
	}
	if end > limit {
		end = limit
	}
	return start, end
}
