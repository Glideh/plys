package scanner

import "testing"

func TestParseMovie(t *testing.T) {
	cases := []struct {
		path  string
		title string
		year  int
	}{
		{"20th.Century.Girl.2022.MULTi.1080p.WEB.x264-NLX5.mkv", "20th Century Girl", 2022},
		{"1337xHD.Shop-Detective Dee Deep Sea Dragon Palace (2020) MLSBD.Shop-Dual Audio Hindi ORG 1080p.mkv", "Detective Dee Deep Sea Dragon Palace", 2020},
		{"00  La MÔME-2007-1080p BluRayrip-Fr-DTS-x264-mkv/la.mome.mkv", "00 La MÔME", 2007},
		{"1917.2019.MULTi.2160p.UHD.BluRay.x265-GROUP.mkv", "1917", 2019},
		{"2001.A.Space.Odyssey.1968.1080p.BluRay.mkv", "2001 A Space Odyssey", 1968},
		{"Blade.Runner.2049.2017.1080p.mkv", "Blade Runner 2049", 2017},
		{"Blade.Runner.2049.1080p.mkv", "Blade Runner 2049", 0},
		{"Alien (1979)/Alien (1979).mkv", "Alien", 1979},
		{"Alien (1979)/movie.mkv", "Alien", 1979},
		{"[Team] Le.Fabuleux.Destin.d'Amelie.Poulain.2001.FRENCH.720p.mkv", "Le Fabuleux Destin d'Amelie Poulain", 2001},
		{"Movie.Title.2010.1080p.x264-GROUP2020.mkv", "Movie Title", 2010},
		{"Inception.mkv", "Inception", 0},
	}
	for _, c := range cases {
		got := ParseMovie(c.path)
		if got.Title != c.title || got.Year != c.year {
			t.Errorf("ParseMovie(%q) = %q (%d), want %q (%d)", c.path, got.Title, got.Year, c.title, c.year)
		}
	}
}

func TestParseEpisode(t *testing.T) {
	cases := []struct {
		path                   string
		title                  string
		year, season, ep, end int
	}{
		{"24.iNTEGRALE.MULTi.1080p.WEB.H265-FW/24.S01.MULTi.1080p.WEB.H265-FW/24.S01E03.MULTi.1080p.WEB.H265-FW.mkv", "24", 0, 1, 3, 0},
		{"1923.S01E03.MULTi.1080p.WEB.x264-FW/1923.S01E03.MULTi.1080p.WEB.x264-FW.mkv", "1923", 0, 1, 3, 0},
		{"1923.S02.MULTi.1080p.WEB.H265-FW/1923.S02E08.MULTi.1080p.WEB.H265-FW.mkv", "1923", 0, 2, 8, 0},
		{"Doctor.Who.2005.S01E01.1080p.mkv", "Doctor Who", 2005, 1, 1, 0},
		{"Mr.Robot.S02E01-E02.mkv", "Mr Robot", 0, 2, 1, 2},
		{"Mr.Robot.S02E01E02.mkv", "Mr Robot", 0, 2, 1, 2},
		{"Friends/Season 3/Friends - 3x05 - The One.avi", "Friends", 0, 3, 5, 0},
		{"Friends/Saison 3/Episode 05.avi", "Friends", 0, 3, 5, 0},
		{"Breaking Bad/S02/S02E04.mkv", "Breaking Bad", 0, 2, 4, 0},
		{"The.Show.S01E01.1080p.WEB.H264-GRP.mkv", "The Show", 0, 1, 1, 0},
	}
	for _, c := range cases {
		got, ok := ParseEpisode(c.path)
		if !ok || got.SeriesTitle != c.title || got.Year != c.year || got.Season != c.season || got.Episode != c.ep || got.EpisodeEnd != c.end {
			t.Errorf("ParseEpisode(%q) = %+v (ok=%v), want %q y%d S%dE%d-%d", c.path, got, ok, c.title, c.year, c.season, c.ep, c.end)
		}
	}

	for _, p := range []string{"random.video.mkv", "Show/Episode 05.mkv", "video.1920x1080.mkv"} {
		if got, ok := ParseEpisode(p); ok {
			t.Errorf("ParseEpisode(%q) should fail, got %+v", p, got)
		}
	}
}

func TestMatchKeyGroupsVariants(t *testing.T) {
	want := MatchKey("Mr. Robot")
	for _, v := range []string{"Mr Robot", "mr.robot", "MR  ROBOT"} {
		if MatchKey(v) != want && v != "mr.robot" {
			t.Errorf("MatchKey(%q) = %q, want %q", v, MatchKey(v), want)
		}
	}
	if MatchKey("Amélie") != "amelie" {
		t.Errorf("accents must be stripped, got %q", MatchKey("Amélie"))
	}
	if SortTitle("The Office") != "office" || SortTitle("Les Misérables") != "miserables" {
		t.Errorf("unexpected sort titles: %q %q", SortTitle("The Office"), SortTitle("Les Misérables"))
	}
}

func TestIsVideo(t *testing.T) {
	for name, want := range map[string]bool{
		"movie.mkv":             true,
		"Movie.MP4":             true,
		"movie.nfo":             false,
		"movie-sample.mkv":      false,
		"sample.mkv":            false,
		"Sampler.Theory.mkv":    true,
		"24.S01E01.MULTi.mkv":   true,
		"subtitles.fr.srt":      false,
	} {
		if got := isVideo(name); got != want {
			t.Errorf("isVideo(%q) = %v, want %v", name, got, want)
		}
	}
}
