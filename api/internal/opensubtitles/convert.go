package opensubtitles

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"
)

const hashChunk = 64 << 10

// MovieHash computes the OpenSubtitles hash of a video: its size plus the sum of the first and last 64 KiB read as
// little-endian 64-bit words. It identifies the exact release, so the subtitles found are in sync.
func MovieHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	size := info.Size()
	if size < hashChunk {
		return "", errors.New("file too small to hash")
	}
	hash := uint64(size)
	buf := make([]byte, hashChunk)
	for _, offset := range []int64{0, size - hashChunk} {
		if _, err := f.ReadAt(buf, offset); err != nil && !errors.Is(err, io.EOF) {
			return "", err
		}
		for i := 0; i < hashChunk; i += 8 {
			hash += binary.LittleEndian.Uint64(buf[i:])
		}
	}
	return fmt.Sprintf("%016x", hash), nil
}

var (
	srtTimingRe = regexp.MustCompile(`^\s*(\d+:\d{2}:\d{2})[,.](\d{3})\s*-->\s*(\d+:\d{2}:\d{2})[,.](\d{3})`)
	// SSA override blocks such as {\an8} or {\i1} that some SRT files carry.
	ssaTagRe = regexp.MustCompile(`\{\\[^}]*\}`)
)

// SRTToVTT converts SubRip text to WebVTT. Files that are not valid UTF-8 are read as Windows-1252, the most
// common legacy encoding of SRT files.
func SRTToVTT(srt []byte) (string, error) {
	text := string(srt)
	if !utf8.ValidString(text) {
		text = decodeWindows1252(srt)
	}
	text = strings.TrimPrefix(text, "\uFEFF")
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")

	var b strings.Builder
	b.WriteString("WEBVTT\n")
	cues := 0
	lines := strings.Split(text, "\n")
	for i := 0; i < len(lines); i++ {
		m := srtTimingRe.FindStringSubmatch(lines[i])
		if m == nil {
			continue
		}
		fmt.Fprintf(&b, "\n%s.%s --> %s.%s\n", padHours(m[1]), m[2], padHours(m[3]), m[4])
		for i+1 < len(lines) && strings.TrimSpace(lines[i+1]) != "" {
			i++
			// A cue line holding "-->" would end the cue text in WebVTT.
			b.WriteString(strings.ReplaceAll(ssaTagRe.ReplaceAllString(lines[i], ""), "-->", "->") + "\n")
		}
		cues++
	}
	if cues == 0 {
		return "", errors.New("no subtitle cue found")
	}
	return b.String(), nil
}

func padHours(t string) string {
	if strings.Index(t, ":") == 1 {
		return "0" + t
	}
	return t
}

// Windows-1252 differs from Latin-1 between 0x80 and 0x9F.
var cp1252 = [32]rune{
	'€', '�', '‚', 'ƒ', '„', '…', '†', '‡', 'ˆ', '‰', 'Š', '‹', 'Œ', '�', 'Ž', '�',
	'�', '‘', '’', '“', '”', '•', '–', '—', '˜', '™', 'š', '›', 'œ', '�', 'ž', 'Ÿ',
}

func decodeWindows1252(data []byte) string {
	var b strings.Builder
	b.Grow(len(data))
	for _, c := range data {
		if c >= 0x80 && c < 0xA0 {
			b.WriteRune(cp1252[c-0x80])
		} else {
			b.WriteRune(rune(c))
		}
	}
	return b.String()
}
