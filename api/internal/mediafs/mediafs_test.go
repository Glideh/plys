package mediafs

import (
	"os"
	"path/filepath"
	"testing"
)

func setup(t *testing.T) (root, outside string) {
	t.Helper()
	base := t.TempDir()
	root = filepath.Join(base, "media")
	outside = filepath.Join(base, "secret")
	for _, d := range []string{"media/Films/Alien (1979)", "media/Series", "media/.hidden", "secret"} {
		if err := os.MkdirAll(filepath.Join(base, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "file.mkv"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	mustSymlink(t, outside, filepath.Join(root, "escape"))
	mustSymlink(t, filepath.Join(root, "Series"), filepath.Join(root, "Shows"))
	return root, outside
}

func mustSymlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
}

func TestResolveConfinesToRoots(t *testing.T) {
	root, outside := setup(t)
	r, errs := NewRoots([]string{root, filepath.Join(root, "missing")})
	if len(errs) != 1 {
		t.Fatalf("expected one error for the missing root, got %v", errs)
	}

	cases := map[string]error{
		root:                                nil,
		filepath.Join(root, "Films"):        nil,
		filepath.Join(root, "Shows"):        nil,
		filepath.Join(root, "..", "secret"): ErrOutsideRoots,
		outside:                             ErrOutsideRoots,
		filepath.Join(root, "escape"):       ErrOutsideRoots,
		filepath.Join(root, "nope"):         ErrNotFound,
		filepath.Join(root, "file.mkv"):     ErrNotDir,
		"relative/path":                     ErrOutsideRoots,
	}
	for p, want := range cases {
		if _, err := r.Resolve(p); err != want {
			t.Errorf("Resolve(%q) = %v, want %v", p, err, want)
		}
	}
}

func TestListDirsHidesFilesHiddenAndEscapingLinks(t *testing.T) {
	root, _ := setup(t)
	r, _ := NewRoots([]string{root})
	_, entries, err := r.ListDirs(root)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name)
	}
	want := []string{"Films", "Series", "Shows"}
	if len(names) != len(want) {
		t.Fatalf("got %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("got %v, want %v", names, want)
		}
	}
}

func TestOverlaps(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"/media/films", "/media/films", true},
		{"/media", "/media/films", true},
		{"/media/films/a", "/media/films", true},
		{"/media/films", "/media/films2", false},
		{"/media/films", "/media/series", false},
	}
	for _, c := range cases {
		if got := Overlaps(c.a, c.b); got != c.want {
			t.Errorf("Overlaps(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}
