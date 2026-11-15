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

// NAS housekeeping folders (Synology, Windows, ext4), release samples and extras.
var ignoredDirs = map[string]bool{
	"@eadir": true, "#recycle": true, "#snapshot": true, "$recycle.bin": true,
	"lost+found": true, "sample": true, "samples": true,
	"bonus": true, "extras": true, "featurettes": true, "trailers": true, "supplements": true,
}

var (
	sitePrefixRe    = regexp.MustCompile(`(?i)^(?:www\.)?[a-z0-9-]+\.(?:shop|com|org|net|to|xyz|cc|me|club|site|ws|tv|io|in)\s*-\s*`)
	bracketPrefixRe = regexp.MustCompile(`^\s*\[[^\]]*\]\s*`)
	bracketGroupRe  = regexp.MustCompile(`\[[^\]]*(?:\]|$)`)
	sampleRe        = regexp.MustCompile(`(?i)(?:^|[\W_])sample(?:[\W_]|$)`)
	// The character following the year is checked in code because Go regexps have no lookahead.
	yearRe = regexp.MustCompile(`[\s._\-(\[]((?:19|20)\d{2})`)
	tagRe  = regexp.MustCompile(`(?i)[\s._\-(\[](?:2160p|1080p|720p|576p|480p|2160|1080|720|4k|uhd|hdr|hdr10|dv|multi|vff|vfq|vfi|vf|vf2|vost|vostfr|truefrench|french|subfrench|korean|japanese|chinese|german|spanish|italian|english|bluray|blu-ray|bdrip|brrip|bluray[a-z]*|webrip|web-?dl|web|hdtv|hdlight|amzn|dvdrip|dvd|x264|x265|h\.?264|h\.?265|hevc|avc|remux|10bit|dts|ac3|eac3|aac|ddp?5\.1|atmos|proper|repack|internal|integrale|complete|extended|unrated|remastered|imax)(?:[\s._\-)\]]|$)`)

	episodeRe     = regexp.MustCompile(`(?i)(?:^|[\s._\-\[(])s(\d{1,2})[\s._-]?e(\d{1,3})(?:(?:-?e|-)(\d{1,3}))?(?:\D|$)`)
	crossRe       = regexp.MustCompile(`(?i)(?:^|[\s._\-\[(])(\d{1,2})x(\d{2,3})(?:\D|$)`)
	episodeOnlyRe = regexp.MustCompile(`(?i)(?:^|[\s._\-])(?:e|ep|episode|épisode)[\s._-]?(\d{1,3})(?:\D|$)`)
	// Fansub style "[Group] Title - 01 [1080p]", optionally "Title S2 - 05".
	absoluteRe     = regexp.MustCompile(`(?i)^(.*?)(?:\s+s(\d{1,2}))?\s+-\s+(\d{1,4})(?:v\d)?(?:[\s\[(.]|$)`)
	seasonDirRe    = regexp.MustCompile(`(?i)^(?:season|saison|series|staffel|s)[\s._-]*(\d{1,2})$`)
	seasonTokenRe  = regexp.MustCompile(`(?i)(?:^|[\s._-])s(\d{1,2})(?:[\s._-]|$)`)
	seasonWordRe   = regexp.MustCompile(`(?i)(?:^|[\s._-])(?:season|saison)[\s._-]*(\d{1,2})`)
	spacesRe       = regexp.MustCompile(`\s+`)
	leadingArticle = regexp.MustCompile(`(?i)^(?:the|a|an|le|la|les|un|une|des)\s+|^l'`)

	editionRe = regexp.MustCompile(`(?i)[\s._\-(\[](director'?s[\s._-]cut|extended(?:[\s._-](?:cut|edition))?|final[\s._-]cut|theatrical(?:[\s._-]cut)?|unrated|uncut|imax|remastered|special[\s._-]edition|ultimate[\s._-]edition|criterion)(?:[\s._\-)\]]|$)`)
)

var editionLabels = map[string]string{
	"directors cut": "Director's Cut", "director's cut": "Director's Cut",
	"extended": "Extended", "extended cut": "Extended", "extended edition": "Extended",
	"final cut": "Final Cut", "theatrical": "Theatrical", "theatrical cut": "Theatrical",
	"unrated": "Unrated", "uncut": "Uncut", "imax": "IMAX", "remastered": "Remastered",
	"special edition": "Special Edition", "ultimate edition": "Ultimate Edition", "criterion": "Criterion",
}

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
	// AltTitle is text found between the year and the release tags, often the translated or original
	// title ("Alien 2 - 1986 - le retour", "John.Woo(1992).Hard.Boiled").
	AltTitle string
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
	if title, year, end := splitTrailingYear(s[:limit]); year != 0 {
		alt := strings.TrimRight(strings.TrimLeft(cleanTitle(s[end:limit]), " -)]"), " -([")
		if len(alt) < 3 {
			alt = ""
		}
		return MovieInfo{Title: title, Year: year, AltTitle: alt}
	}
	return MovieInfo{Title: cleanTitle(s[:limit])}
}

// splitTrailingYear returns the last plausible year of s, the non-empty title preceding it and the index
// following the year.
func splitTrailingYear(s string) (string, int, int) {
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
			return title, year, m[3]
		}
	}
	return "", 0, 0
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
	preferFolderTitle := false
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
	case episodeOnlyRe.MatchString(base):
		m := episodeOnlyRe.FindStringSubmatchIndex(base)
		info.Season = seasonFromDirs(dirs)
		info.Episode = atoi(base[m[2]:m[3]])
		titlePart = base[:m[0]]
	case absoluteRe.MatchString(stripPrefixes(base)):
		s := stripPrefixes(base)
		m := absoluteRe.FindStringSubmatchIndex(s)
		info.Season = seasonFromDirs(dirs)
		if m[4] >= 0 {
			info.Season = atoi(s[m[4]:m[5]])
		}
		info.Episode = atoi(s[m[6]:m[7]])
		titlePart = s[m[2]:m[3]]
		// Fansub file names often abbreviate the show ("FT - 227"); the folder name is more reliable.
		preferFolderTitle = true
	default:
		return info, false
	}
	if info.EpisodeEnd <= info.Episode {
		info.EpisodeEnd = 0
	}

	if !preferFolderTitle || len(dirs) == 0 {
		info.SeriesTitle, info.Year = seriesTitle(titlePart)
	}
	for i := len(dirs) - 1; info.SeriesTitle == "" && i >= 0; i-- {
		if _, isSeason := seasonOf(dirs[i]); isSeason {
			continue
		}
		info.SeriesTitle, info.Year = seriesTitle(dirs[i])
	}
	if info.SeriesTitle == "" {
		info.SeriesTitle, info.Year = seriesTitle(titlePart)
	}
	return info, info.SeriesTitle != ""
}

// seasonFromDirs reads the season from the closest folder ("Season 2", "Fairy Tail Season 6"), defaulting to 1.
func seasonFromDirs(dirs []string) int {
	for i := len(dirs) - 1; i >= 0; i-- {
		if n, ok := seasonOf(dirs[i]); ok {
			return n
		}
		if m := seasonWordRe.FindStringSubmatch(dirs[i]); m != nil {
			return atoi(m[1])
		}
	}
	return 1
}

func seasonOf(dir string) (int, bool) {
	if m := seasonDirRe.FindStringSubmatch(strings.TrimSpace(dir)); m != nil {
		return atoi(m[1]), true
	}
	return 0, false
}

func seriesTitle(raw string) (string, int) {
	s := stripPrefixes(raw)
	for _, re := range []*regexp.Regexp{seasonTokenRe, seasonWordRe, tagRe} {
		if loc := re.FindStringIndex(s); loc != nil {
			s = s[:loc[0]]
		}
	}
	if title, year, _ := splitTrailingYear(s); year != 0 {
		return title, year
	}
	return cleanTitle(s), 0
}

func stripPrefixes(s string) string {
	s = sitePrefixRe.ReplaceAllString(s, "")
	return bracketPrefixRe.ReplaceAllString(s, "")
}

func cleanTitle(s string) string {
	s = bracketGroupRe.ReplaceAllString(s, " ")
	s = strings.NewReplacer(".", " ", "_", " ").Replace(s)
	// An opening parenthesis left unbalanced by a year or tag cut, as in "Title (BBC -1995)".
	if i := strings.LastIndex(s, "("); i >= 0 && !strings.Contains(s[i:], ")") {
		s = s[:i]
	}
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

// ParseEdition reads the edition from what follows the title in the file name or its folder,
// so a title such as "Loose Change Final Cut" is not an edition.
func ParseEdition(path string) string {
	dir, base := filepath.Dir(path), stem(filepath.Base(path))
	for _, s := range []string{base, filepath.Base(dir)} {
		after := s
		if loc := yearRe.FindStringIndex(s); loc != nil {
			after = s[loc[1]:]
		} else if loc := tagRe.FindStringIndex(s); loc != nil {
			after = s[loc[0]:]
		} else {
			continue
		}
		if m := editionRe.FindStringSubmatch(after); m != nil {
			return editionLabels[strings.ToLower(strings.NewReplacer(".", " ", "_", " ", "-", " ").Replace(m[1]))]
		}
	}
	return ""
}
