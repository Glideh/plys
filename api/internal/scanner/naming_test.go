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
		{"[Commie] Ookami Kodomo no Ame to Yuki [BD 720p AAC] [B4B84F9F].mkv", "Ookami Kodomo no Ame to Yuki", 0},
		{"Avatar La voie de l'eau [FR-EN] (2022).mkv", "Avatar La voie de l'eau", 2022},
		{"Soulmate.KOREAN.Vostfr.1080p.WEB-DL.AAC2.0.x264-Notag.mkv", "Soulmate", 0},
		{"[yggtorrent.com]Guerre et Paix.1966.DVD 1.MULTI.DVDRip.XVid-AC3.afrique31/Guerre et Paix CD1.avi", "Guerre et Paix", 1966},
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
		path                  string
		title                 string
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
		{"Demon Slayer/[Hakata Ramen] Kimetsu no Yaiba (Demon Slayer) - 07 [1080p][HEVC].mkv", "Demon Slayer", 0, 1, 7, 0},
		{"Vinland Saga/[Judas] Vinland Saga S2 - 14.mkv", "Vinland Saga", 0, 2, 14, 0},
		{"Fairy Tail Season 6/FT - 227 [S2-52] L@mBerT.mp4", "Fairy Tail", 0, 6, 227, 0},
		{"Tengen Toppa Gurren Lagann/Épisode 05 - Je pige que dalle à tout ça !.mkv", "Tengen Toppa Gurren Lagann", 0, 1, 5, 0},
		{"Pride & Prejudice/Pride and Prejudice (BBC-1995) 1080p/Pride and Prejudice (BBC -1995) E2 1080p H.264 (moviesbyrizzo).mp4", "Pride and Prejudice", 1995, 1, 2, 0},
		{"Suits Season 9 eps 1-10/Suits.S09E03.1080p.mkv", "Suits", 0, 9, 3, 0},
		{"Homeland Season 8/E05.mkv", "Homeland", 0, 8, 5, 0},
		{"Show/Episode 05.mkv", "Show", 0, 1, 5, 0},
	}
	for _, c := range cases {
		got, ok := ParseEpisode(c.path)
		if !ok || got.SeriesTitle != c.title || got.Year != c.year || got.Season != c.season || got.Episode != c.ep || got.EpisodeEnd != c.end {
			t.Errorf("ParseEpisode(%q) = %+v (ok=%v), want %q y%d S%dE%d-%d", c.path, got, ok, c.title, c.year, c.season, c.ep, c.end)
		}
	}

	for _, p := range []string{"random.video.mkv", "video.1920x1080.mkv", "NIKITA/1a. Movies (1990-93)/La Femme Nikita (1990 - 720p BluRay).mp4"} {
		if got, ok := ParseEpisode(p); ok {
			t.Errorf("ParseEpisode(%q) should fail, got %+v", p, got)
		}
	}
}

func TestParseFileVersion(t *testing.T) {
	cases := []struct {
		path    string
		part    int
		edition string
	}{
		{"/media/Films/[yggtorrent.com]Guerre et Paix.1966.DVD 2.MULTI.DVDRip.XVid-AC3.afrique31/Guerre et Paix.avi", 2, ""},
		{"/media/Films/Loose.Change.Final.Cut.2007.VOSTFR.DVDRIP.avi/Partie 1.avi", 1, ""},
		{"/media/Films/Movie.2001.CD2.avi", 2, ""},
		{"/media/Films/Blade.Runner.1982.Final.Cut.2160p.UHD.mkv", 0, "Final Cut"},
		{"/media/Films/Kingdom.of.Heaven.2005.Directors.Cut.1080p.mkv", 0, "Director's Cut"},
		{"/media/Films/The.Lord.of.the.Rings.2001.EXTENDED.1080p.mkv", 0, "Extended"},
		{"/media/Films/Prisoners.2013.1080p.BluRay.DVDRip.mkv", 0, ""},
	}
	for _, c := range cases {
		if got := ParseFileVersion(c.path); got.Part != c.part || got.Edition != c.edition {
			t.Errorf("ParseFileVersion(%q) = %+v, want part %d edition %q", c.path, got, c.part, c.edition)
		}
	}

	parts := []string{
		"/media/Films/[yggtorrent.com]Guerre et Paix.1966.DVD 1.MULTI.DVDRip/Guerre et Paix.avi",
		"/media/Films/[yggtorrent.com]Guerre et Paix.1966.DVD 2.MULTI.DVDRip/Guerre et Paix.avi",
	}
	if ParseFileVersion(parts[0]).Key != ParseFileVersion(parts[1]).Key {
		t.Error("parts of one version must share a version key")
	}
	versions := []string{
		"/media/Films/Disney/Clochette et la Pierre de Lune 2009 1080p.mkv",
		"/media/Films/Disney/Clochette et la Pierre de Lune 2009 1080p (1).mkv",
	}
	if ParseFileVersion(versions[0]).Key == ParseFileVersion(versions[1]).Key {
		t.Error("distinct files without part numbers are distinct versions")
	}
	if got := ParseMovie("Movie.Title.CD1.avi"); got.Title != "Movie Title" {
		t.Errorf("part token must be stripped from the title, got %q", got.Title)
	}
}

func TestMatchKeyGroupsVariants(t *testing.T) {
	want := MatchKey("Mr. Robot")
	for _, v := range []string{"Mr Robot", "mr.robot", "MR  ROBOT"} {
		if MatchKey(v) != want {
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
		"movie.mkv":           true,
		"Movie.MP4":           true,
		"movie.nfo":           false,
		"movie-sample.mkv":    false,
		"sample.mkv":          false,
		"Sampler.Theory.mkv":  true,
		"24.S01E01.MULTi.mkv": true,
		"subtitles.fr.srt":    false,
	} {
		if got := isVideo(name); got != want {
			t.Errorf("isVideo(%q) = %v, want %v", name, got, want)
		}
	}
}
