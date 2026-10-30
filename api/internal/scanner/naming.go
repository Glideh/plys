package scanner

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

var videoExtensions = map[string]bool{
	".mkv": true, ".mp4": true, ".m4v": true, ".avi": true, ".mov": true, ".wmv": true,
	".ts": true, ".m2ts": true, ".mts": true, ".webm": true, ".mpg": true, ".mpeg": true,
	".flv": true, ".ogv": true, ".divx": true,
}

// NAS housekeeping folders (Synology, Windows, ext4) and release samples.
var ignoredDirs = map[string]bool{
	"@eadir": true, "#recycle": true, "#snapshot": true, "$recycle.bin": true,
	"lost+found": true, "sample": true, "samples": true,
}

var (
	sitePrefixRe    = regexp.MustCompile(`(?i)^(?:www\.)?[a-z0-9-]+\.(?:shop|com|org|net|to|xyz|cc|me|club|site|ws|tv|io|in)\s*-\s*`)
	bracketPrefixRe = regexp.MustCompile(`^\s*\[[^\]]*\]\s*`)
	sampleRe        = regexp.MustCompile(`(?i)(?:^|[\W_])sample(?:[\W_]|$)`)
	// The character following the year is checked in code because Go regexps have no lookahead.
	yearRe = regexp.MustCompile(`[\s._\-(\[]((?:19|20)\d{2})`)
	tagRe  = regexp.MustCompile(`(?i)[\s._\-(\[](?:2160p|1080p|720p|576p|480p|4k|uhd|hdr|hdr10|dv|multi|vff|vfq|vfi|vf|vf2|vostfr|truefrench|french|subfrench|bluray|blu-ray|bdrip|brrip|bluray[a-z]*|webrip|web-?dl|web|hdtv|dvdrip|dvd|x264|x265|h\.?264|h\.?265|hevc|avc|remux|10bit|dts|ac3|eac3|aac|ddp?5\.1|atmos|proper|repack|internal|integrale|complete|extended|unrated|remastered|imax)(?:[\s._\-)\]]|$)`)

	episodeRe      = regexp.MustCompile(`(?i)(?:^|[\s._\-\[(])s(\d{1,2})[\s._-]?e(\d{1,3})(?:(?:-?e|-)(\d{1,3}))?(?:\D|$)`)
	crossRe        = regexp.MustCompile(`(?i)(?:^|[\s._\-\[(])(\d{1,2})x(\d{2,3})(?:\D|$)`)
	episodeOnlyRe  = regexp.MustCompile(`(?i)(?:^|[\s._\-])(?:e|ep|episode|épisode)[\s._-]?(\d{1,3})(?:\D|$)`)
	seasonDirRe    = regexp.MustCompile(`(?i)^(?:season|saison|series|staffel|s)[\s._-]*(\d{1,2})$`)
	seasonTokenRe  = regexp.MustCompile(`(?i)(?:^|[\s._-])s(\d{1,2})(?:[\s._-]|$)`)
	spacesRe       = regexp.MustCompile(`\s+`)
	leadingArticle = regexp.MustCompile(`(?i)^(?:the|a|an|le|la|les|un|une|des)\s+|^l'`)
)

func isVideo(name string) bool {
	return videoExtensions[strings.ToLower(filepath.Ext(name))] && !sampleRe.MatchString(stem(name))
}

func isIgnoredDir(name string) bool {
	return strings.HasPrefix(name, ".") || ignoredDirs[strings.ToLower(name)]
}

func stem(name string) string {
	return strings.TrimSuffix(name, filepath.Ext(name))
}

type MovieInfo struct {
	Title string
	Year  int
}

// ParseMovie reads the file name, falling back to the parent folder when the file name carries no year.
func ParseMovie(relPath string) MovieInfo {
	info := parseMovieName(stem(filepath.Base(relPath)))
	if info.Year == 0 {
		if dir := filepath.Base(filepath.Dir(relPath)); dir != "." && dir != "/" {
			if d := parseMovieName(dir); d.Year != 0 {
				return d
			}
		}
	}
	return info
}

func parseMovieName(name string) MovieInfo {
	s := stripPrefixes(name)
	limit := len(s)
	if loc := tagRe.FindStringIndex(s); loc != nil && loc[0] > 0 {
		limit = loc[0] + 1
	}
	if title, year := splitTrailingYear(s[:limit]); year != 0 {
		return MovieInfo{Title: title, Year: year}
	}
	return MovieInfo{Title: cleanTitle(s[:limit])}
}

// splitTrailingYear returns the last plausible year of s and the non-empty title preceding it.
func splitTrailingYear(s string) (string, int) {
	maxYear := time.Now().Year() + 1
	matches := yearRe.FindAllStringSubmatchIndex(s, -1)
	for i := len(matches) - 1; i >= 0; i-- {
		m := matches[i]
		if m[3] < len(s) && !strings.ContainsRune(" ._-)]", rune(s[m[3]])) {
			continue
		}
		year, _ := strconv.Atoi(s[m[2]:m[3]])
		if year > maxYear {
			continue
		}
		if title := cleanTitle(s[:m[0]]); title != "" {
			return title, year
		}
	}
	return "", 0
}

type EpisodeInfo struct {
	SeriesTitle string
	Year        int
	Season      int
	Episode     int
	EpisodeEnd  int
}

// ParseEpisode reads SxxEyy (or 1x02) from the file name; the series title comes from the file name,
// or from the closest folder that is not a season folder.
func ParseEpisode(relPath string) (EpisodeInfo, bool) {
	base := stem(filepath.Base(relPath))
	var dirs []string
	for _, d := range strings.Split(filepath.Dir(relPath), string(filepath.Separator)) {
		if d != "" && d != "." {
			dirs = append(dirs, d)
		}
	}

	var info EpisodeInfo
	var titlePart string
	switch {
	case episodeRe.MatchString(base):
		m := episodeRe.FindStringSubmatchIndex(base)
		info.Season = atoi(base[m[2]:m[3]])
		info.Episode = atoi(base[m[4]:m[5]])
		if m[6] >= 0 {
			info.EpisodeEnd = atoi(base[m[6]:m[7]])
		}
		titlePart = base[:m[0]]
	case crossRe.MatchString(base):
		m := crossRe.FindStringSubmatchIndex(base)
		info.Season = atoi(base[m[2]:m[3]])
		info.Episode = atoi(base[m[4]:m[5]])
		titlePart = base[:m[0]]
	case episodeOnlyRe.MatchString(base) && len(dirs) > 0:
		season, ok := seasonOf(dirs[len(dirs)-1])
		if !ok {
			return info, false
		}
		m := episodeOnlyRe.FindStringSubmatchIndex(base)
		info.Season = season
		info.Episode = atoi(base[m[2]:m[3]])
		titlePart = base[:m[0]]
	default:
		return info, false
	}
	if info.EpisodeEnd <= info.Episode {
		info.EpisodeEnd = 0
	}

	info.SeriesTitle, info.Year = seriesTitle(titlePart)
	for i := len(dirs) - 1; info.SeriesTitle == "" && i >= 0; i-- {
		if _, isSeason := seasonOf(dirs[i]); isSeason {
			continue
		}
		info.SeriesTitle, info.Year = seriesTitle(dirs[i])
	}
	return info, info.SeriesTitle != ""
}

func seasonOf(dir string) (int, bool) {
	if m := seasonDirRe.FindStringSubmatch(strings.TrimSpace(dir)); m != nil {
		return atoi(m[1]), true
	}
	return 0, false
}

func seriesTitle(raw string) (string, int) {
	s := stripPrefixes(raw)
	if loc := seasonTokenRe.FindStringIndex(s); loc != nil {
		s = s[:loc[0]]
	}
	if loc := tagRe.FindStringIndex(s); loc != nil {
		s = s[:loc[0]]
	}
	if title, year := splitTrailingYear(s); year != 0 {
		return title, year
	}
	return cleanTitle(s), 0
}

func stripPrefixes(s string) string {
	s = sitePrefixRe.ReplaceAllString(s, "")
	return bracketPrefixRe.ReplaceAllString(s, "")
}

func cleanTitle(s string) string {
	s = strings.NewReplacer(".", " ", "_", " ").Replace(s)
	s = spacesRe.ReplaceAllString(s, " ")
	return strings.Trim(s, " -([")
}

// MatchKey normalizes a title so that "Mr.Robot", "Mr Robot" and "mr. robot" group together.
func MatchKey(title string) string {
	var b strings.Builder
	space := false
	for _, r := range norm.NFD.String(strings.ToLower(title)) {
		switch {
		case unicode.Is(unicode.Mn, r):
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if space && b.Len() > 0 {
				b.WriteByte(' ')
			}
			space = false
			b.WriteRune(r)
		default:
			space = true
		}
	}
	return b.String()
}

func SortTitle(title string) string {
	return leadingArticle.ReplaceAllString(MatchKey(title), "")
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
