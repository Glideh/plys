package api

import "testing"

func TestCommonFolder(t *testing.T) {
	cases := []struct {
		paths []string
		want  string
	}{
		{nil, ""},
		{[]string{"/media/films/Alien.1979.mkv"}, "/media/films"},
		{[]string{"/media/series/Show/S01/e1.mkv", "/media/series/Show/S02/e1.mkv"}, "/media/series/Show"},
		{[]string{"/media/films/Blade/a.mkv", "/media/films/Blade Runner/b.mkv"}, "/media/films"},
		{[]string{"/media/a/x.mkv", "/other/y.mkv"}, "/"},
	}
	for _, c := range cases {
		if got := commonFolder(c.paths); got != c.want {
			t.Errorf("commonFolder(%v) = %q, want %q", c.paths, got, c.want)
		}
	}
}
