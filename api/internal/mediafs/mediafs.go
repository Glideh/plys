package mediafs

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

var (
	ErrOutsideRoots = errors.New("path is outside the media roots")
	ErrNotFound     = errors.New("directory not found")
	ErrNotDir       = errors.New("path is not a directory")
)

// Roots confines browsing and library paths to the directories mounted for media.
type Roots struct {
	paths []string
}

// NewRoots resolves each root; missing roots are reported but do not prevent the others from being used.
func NewRoots(paths []string) (*Roots, []error) {
	r := &Roots{}
	var errs []error
	for _, p := range paths {
		if p == "" {
			continue
		}
		real, err := resolveDir(p)
		if err != nil {
			errs = append(errs, fmt.Errorf("media root %q: %w", p, err))
			continue
		}
		if !slices.Contains(r.paths, real) {
			r.paths = append(r.paths, real)
		}
	}
	slices.Sort(r.paths)
	return r, errs
}

func (r *Roots) List() []string {
	return slices.Clone(r.paths)
}

// Resolve returns the real path of an existing directory located inside one of the roots.
func (r *Roots) Resolve(p string) (string, error) {
	if !filepath.IsAbs(p) {
		return "", ErrOutsideRoots
	}
	real, err := resolveDir(p)
	if err != nil {
		return "", err
	}
	if !r.contains(real) {
		return "", ErrOutsideRoots
	}
	return real, nil
}

// IsRoot reports whether a resolved path is one of the roots.
func (r *Roots) IsRoot(real string) bool {
	return slices.Contains(r.paths, real)
}

type Entry struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// ListDirs lists visible subdirectories of p, following symlinks only when they stay inside the roots.
func (r *Roots) ListDirs(p string) (string, []Entry, error) {
	real, err := r.Resolve(p)
	if err != nil {
		return "", nil, err
	}
	items, err := os.ReadDir(real)
	if err != nil {
		return "", nil, err
	}
	entries := []Entry{}
	for _, it := range items {
		name := it.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		full := filepath.Join(real, name)
		if it.Type()&fs.ModeSymlink != 0 {
			if _, err := r.Resolve(full); err != nil {
				continue
			}
		} else if !it.IsDir() {
			continue
		}
		entries = append(entries, Entry{Name: name, Path: full})
	}
	slices.SortFunc(entries, func(a, b Entry) int {
		return cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	return real, entries, nil
}

func (r *Roots) contains(real string) bool {
	return slices.ContainsFunc(r.paths, func(root string) bool { return Within(root, real) })
}

// Within reports whether child equals parent or is located below it.
func Within(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, "../")
}

// Overlaps reports whether one path contains the other.
func Overlaps(a, b string) bool {
	return Within(a, b) || Within(b, a)
}

func resolveDir(p string) (string, error) {
	real, err := filepath.EvalSymlinks(filepath.Clean(p))
	if errors.Is(err, fs.ErrNotExist) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	info, err := os.Stat(real)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", ErrNotDir
	}
	return real, nil
}
