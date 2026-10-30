package metadata

import (
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	maxNFOSize       = 512 << 10
	maxNFOFilesInDir = 3
)

var (
	imdbIDRe   = regexp.MustCompile(`\btt\d{7,8}\b`)
	kodiTMDBRe = regexp.MustCompile(`(?is)<uniqueid[^>]*type="tmdb"[^>]*>\s*(\d+)\s*<|<tmdbid>\s*(\d+)\s*<`)
	kodiRootRe = regexp.MustCompile(`(?is)^\s*(?:<\?xml[^>]*>\s*)?<(movie|tvshow|episodedetails)[\s>]`)
)

// NFOIDs are identifiers found next to the media files. Kodi NFO files are XML; release NFO files are
// free text that usually links to IMDb.
type NFOIDs struct {
	// Kind is the Kodi root element (movie, tvshow, episodedetails), empty for release NFO files.
	Kind string
	TMDB int
	IMDb string
}

// findNFOIDs looks for <video>.nfo, then other .nfo files in the folder of each video and its parent.
func findNFOIDs(videoPaths []string) NFOIDs {
	seen := map[string]bool{}
	for _, video := range videoPaths {
		dir := filepath.Dir(video)
		candidates := []string{strings.TrimSuffix(video, filepath.Ext(video)) + ".nfo"}
		for _, d := range []string{dir, filepath.Dir(dir)} {
			matches, _ := filepath.Glob(filepath.Join(globEscape(d), "*.nfo"))
			if len(matches) > maxNFOFilesInDir {
				matches = matches[:maxNFOFilesInDir]
			}
			candidates = append(candidates, matches...)
		}
		for _, c := range candidates {
			if seen[c] {
				continue
			}
			seen[c] = true
			if ids, ok := readNFO(c); ok {
				return ids
			}
		}
	}
	return NFOIDs{}
}

func readNFO(path string) (NFOIDs, bool) {
	f, err := os.Open(path)
	if err != nil {
		return NFOIDs{}, false
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxNFOSize))
	if err != nil {
		return NFOIDs{}, false
	}
	return parseNFO(string(data))
}

func parseNFO(text string) (NFOIDs, bool) {
	var ids NFOIDs
	if m := kodiRootRe.FindStringSubmatch(text); m != nil {
		ids.Kind = strings.ToLower(m[1])
		if t := kodiTMDBRe.FindStringSubmatch(text); t != nil {
			ids.TMDB, _ = strconv.Atoi(t[1] + t[2])
		}
	}
	ids.IMDb = imdbIDRe.FindString(text)
	return ids, ids.TMDB > 0 || ids.IMDb != ""
}

func globEscape(p string) string {
	return strings.NewReplacer(`[`, `\[`, `]`, `\]`, `*`, `\*`, `?`, `\?`).Replace(p)
}
