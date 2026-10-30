package api

import (
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"toto/api/internal/auth"
	"toto/api/internal/httpx"
	"toto/api/internal/mediafs"
	"toto/api/internal/store"
)

const (
	maxLibraryNameLen  = 100
	maxPathsPerLibrary = 20
)

var libraryKinds = []string{"movies", "series", "other"}

type pathConflictError struct {
	path, other string
}

func (e *pathConflictError) Error() string {
	return fmt.Sprintf("path %q overlaps %q, already used by another library", e.path, e.other)
}

type libraryDTO struct {
	ID            uuid.UUID        `json:"id"`
	Name          string           `json:"name"`
	Kind          string           `json:"kind"`
	Paths         []string         `json:"paths,omitempty"`
	Counts        map[string]int64 `json:"counts,omitempty"`
	LastScannedAt *time.Time       `json:"lastScannedAt"`
	CreatedAt     time.Time        `json:"createdAt"`
}

func toLibraryDTO(l store.Library, paths []string) libraryDTO {
	return libraryDTO{ID: l.ID, Name: l.Name, Kind: l.Kind, Paths: paths, LastScannedAt: l.LastScannedAt, CreatedAt: l.CreatedAt}
}

func isAdmin(r *http.Request) bool {
	p, ok := auth.PrincipalFrom(r.Context())
	return ok && p.Role == auth.RoleAdmin
}

// Paths reveal the server layout, so they are only exposed to admins.
func (s *Server) listLibraries(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	libs, err := s.q.ListLibraries(ctx)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	paths := map[uuid.UUID][]string{}
	if isAdmin(r) {
		all, err := s.q.ListAllLibraryPaths(ctx)
		if err != nil {
			httpx.InternalError(w, r, err)
			return
		}
		for _, p := range all {
			paths[p.LibraryID] = append(paths[p.LibraryID], p.Path)
		}
	}
	counts, err := s.q.CountItemsByLibrary(ctx)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	byLibrary := map[uuid.UUID]map[string]int64{}
	for _, c := range counts {
		if byLibrary[c.LibraryID] == nil {
			byLibrary[c.LibraryID] = map[string]int64{}
		}
		byLibrary[c.LibraryID][c.Kind] = c.Count
	}
	out := make([]libraryDTO, 0, len(libs))
	for _, l := range libs {
		dto := toLibraryDTO(l, paths[l.ID])
		dto.Counts = byLibrary[l.ID]
		out = append(out, dto)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": out})
}

func (s *Server) getLibrary(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "library")
	if !ok {
		return
	}
	ctx := r.Context()
	lib, err := s.q.GetLibrary(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, http.StatusNotFound, "not_found", "library not found")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	var paths []string
	if isAdmin(r) {
		if paths, err = s.q.ListLibraryPaths(ctx, id); err != nil {
			httpx.InternalError(w, r, err)
			return
		}
	}
	httpx.JSON(w, http.StatusOK, toLibraryDTO(lib, paths))
}

type createLibraryRequest struct {
	Name  string   `json:"name"`
	Kind  string   `json:"kind"`
	Paths []string `json:"paths"`
}

func (s *Server) createLibrary(w http.ResponseWriter, r *http.Request) {
	var req createLibraryRequest
	if !httpx.DecodeRequired(w, r, &req) {
		return
	}
	name, nameErr := validateLibraryName(req.Name)
	var kindErr error
	if !slices.Contains(libraryKinds, req.Kind) {
		kindErr = errors.New("kind must be one of: movies, series, other")
	}
	paths, pathsErr := s.resolveLibraryPaths(req.Paths)
	if err := errors.Join(nameErr, kindErr, pathsErr); err != nil {
		httpx.Error(w, http.StatusBadRequest, "validation", err.Error())
		return
	}

	ctx := r.Context()
	var lib store.Library
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		if err := lockAndCheckPaths(r, q, uuid.Nil, paths); err != nil {
			return err
		}
		var err error
		if lib, err = q.CreateLibrary(ctx, store.CreateLibraryParams{Name: name, Kind: req.Kind}); err != nil {
			return err
		}
		return addPaths(r, q, lib.ID, paths)
	})
	if writeLibraryMutationError(w, r, err) {
		return
	}
	s.scanner.Enqueue(lib.ID)
	httpx.JSON(w, http.StatusCreated, toLibraryDTO(lib, paths))
}

// Kind is immutable: changing it would invalidate every item already matched for the library.
type updateLibraryRequest struct {
	Name  *string   `json:"name"`
	Paths *[]string `json:"paths"`
}

func (s *Server) updateLibrary(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "library")
	if !ok {
		return
	}
	var req updateLibraryRequest
	if !httpx.DecodeRequired(w, r, &req) {
		return
	}
	var name string
	var paths []string
	var errs []error
	if req.Name != nil {
		var err error
		name, err = validateLibraryName(*req.Name)
		errs = append(errs, err)
	}
	if req.Paths != nil {
		var err error
		paths, err = s.resolveLibraryPaths(*req.Paths)
		errs = append(errs, err)
	}
	if err := errors.Join(errs...); err != nil {
		httpx.Error(w, http.StatusBadRequest, "validation", err.Error())
		return
	}

	ctx := r.Context()
	var lib store.Library
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		var err error
		if req.Name != nil {
			lib, err = q.RenameLibrary(ctx, store.RenameLibraryParams{ID: id, Name: name})
		} else {
			lib, err = q.GetLibrary(ctx, id)
		}
		if err != nil {
			return err
		}
		if req.Paths != nil {
			if err := lockAndCheckPaths(r, q, id, paths); err != nil {
				return err
			}
			if err := q.ClearLibraryPaths(ctx, id); err != nil {
				return err
			}
			if err := addPaths(r, q, id, paths); err != nil {
				return err
			}
		}
		paths, err = q.ListLibraryPaths(ctx, id)
		return err
	})
	if writeLibraryMutationError(w, r, err) {
		return
	}
	if req.Paths != nil {
		s.scanner.Enqueue(id)
	}
	httpx.JSON(w, http.StatusOK, toLibraryDTO(lib, paths))
}

func (s *Server) deleteLibrary(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "library")
	if !ok {
		return
	}
	n, err := s.q.DeleteLibrary(r.Context(), id)
	if err == nil && n == 0 {
		err = pgx.ErrNoRows
	}
	if writeLibraryMutationError(w, r, err) {
		return
	}
	s.scanner.Forget(id)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) scanLibrary(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r, "library")
	if !ok {
		return
	}
	_, err := s.q.GetLibrary(r.Context(), id)
	if writeLibraryMutationError(w, r, err) {
		return
	}
	httpx.JSON(w, http.StatusAccepted, s.scanner.Enqueue(id))
}

// Scan statuses live in memory: they reset when the API restarts, lastScannedAt persists.
func (s *Server) listScans(w http.ResponseWriter, _ *http.Request) {
	httpx.JSON(w, http.StatusOK, map[string]any{"items": s.scanner.Statuses()})
}

func validateLibraryName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" || utf8.RuneCountInString(name) > maxLibraryNameLen {
		return "", errors.New("name must be between 1 and 100 characters")
	}
	return name, nil
}

// resolveLibraryPaths returns real, deduplicated paths that do not overlap each other.
func (s *Server) resolveLibraryPaths(raw []string) ([]string, error) {
	if len(raw) == 0 || len(raw) > maxPathsPerLibrary {
		return nil, fmt.Errorf("paths must contain between 1 and %d directories", maxPathsPerLibrary)
	}
	var out []string
	for _, p := range raw {
		real, err := s.roots.Resolve(p)
		if err != nil {
			return nil, fmt.Errorf("path %q: %w", p, err)
		}
		if slices.Contains(out, real) {
			continue
		}
		for _, other := range out {
			if mediafs.Overlaps(real, other) {
				return nil, fmt.Errorf("paths %q and %q overlap", real, other)
			}
		}
		out = append(out, real)
	}
	slices.Sort(out)
	return out, nil
}

// lockAndCheckPaths rejects paths nested in, or containing, a path of another library.
func lockAndCheckPaths(r *http.Request, q *store.Queries, self uuid.UUID, paths []string) error {
	ctx := r.Context()
	if err := q.LockLibraries(ctx); err != nil {
		return err
	}
	existing, err := q.ListAllLibraryPaths(ctx)
	if err != nil {
		return err
	}
	for _, e := range existing {
		if e.LibraryID == self {
			continue
		}
		for _, p := range paths {
			if mediafs.Overlaps(p, e.Path) {
				return &pathConflictError{path: p, other: e.Path}
			}
		}
	}
	return nil
}

func addPaths(r *http.Request, q *store.Queries, id uuid.UUID, paths []string) error {
	for _, p := range paths {
		if err := q.AddLibraryPath(r.Context(), store.AddLibraryPathParams{LibraryID: id, Path: p}); err != nil {
			return err
		}
	}
	return nil
}

func writeLibraryMutationError(w http.ResponseWriter, r *http.Request, err error) bool {
	var conflict *pathConflictError
	switch {
	case err == nil:
		return false
	case errors.Is(err, pgx.ErrNoRows):
		httpx.Error(w, http.StatusNotFound, "not_found", "library not found")
	case errors.As(err, &conflict):
		httpx.Error(w, http.StatusConflict, "path_conflict", conflict.Error())
	case isUniqueViolation(err):
		httpx.Error(w, http.StatusConflict, "name_taken", "a library with this name already exists")
	default:
		httpx.InternalError(w, r, err)
	}
	return true
}

type browseResponse struct {
	Path    *string         `json:"path"`
	Parent  *string         `json:"parent"`
	Entries []mediafs.Entry `json:"entries"`
}

// browseFS lists the media roots when no path is given, otherwise the subdirectories of path.
// A null parent means "back to the roots list".
func (s *Server) browseFS(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Query().Get("path")
	if p == "" {
		entries := []mediafs.Entry{}
		for _, root := range s.roots.List() {
			entries = append(entries, mediafs.Entry{Name: root, Path: root})
		}
		httpx.JSON(w, http.StatusOK, browseResponse{Entries: entries})
		return
	}
	real, entries, err := s.roots.ListDirs(p)
	switch {
	case errors.Is(err, mediafs.ErrNotFound):
		httpx.Error(w, http.StatusNotFound, "not_found", "directory not found")
		return
	case errors.Is(err, mediafs.ErrOutsideRoots), errors.Is(err, mediafs.ErrNotDir):
		httpx.Error(w, http.StatusBadRequest, "invalid_path", err.Error())
		return
	case err != nil:
		httpx.InternalError(w, r, err)
		return
	}
	resp := browseResponse{Path: &real, Entries: entries}
	if !s.roots.IsRoot(real) {
		parent := filepath.Dir(real)
		resp.Parent = &parent
	}
	httpx.JSON(w, http.StatusOK, resp)
}

func idParam(w http.ResponseWriter, r *http.Request, what string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "not_found", what+" not found")
		return uuid.Nil, false
	}
	return id, true
}
