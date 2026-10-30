package scanner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const probeTimeout = 60 * time.Second

// Bitmap subtitle codecs cannot be converted to WebVTT and must be burned in.
var imageSubtitleCodecs = map[string]bool{
	"hdmv_pgs_subtitle": true, "dvd_subtitle": true, "dvb_subtitle": true, "xsub": true,
}

type Stream struct {
	Index    int    `json:"index"`
	Type     string `json:"type"`
	Codec    string `json:"codec"`
	Profile  string `json:"profile,omitempty"`
	Language string `json:"language,omitempty"`
	Title    string `json:"title,omitempty"`
	Default  bool   `json:"default"`
	Forced   bool   `json:"forced"`
	Channels int    `json:"channels,omitempty"`
	Width    int    `json:"width,omitempty"`
	Height   int    `json:"height,omitempty"`
	PixFmt   string `json:"pixFmt,omitempty"`
	// smpte2084 (HDR10/Dolby Vision) or arib-std-b67 (HLG) mark HDR video.
	ColorTransfer string `json:"colorTransfer,omitempty"`
	ImageBased    bool   `json:"imageBased,omitempty"`
}

type ProbeResult struct {
	Duration  float64
	Container string
	Bitrate   int64
	Streams   []Stream
}

type Prober interface {
	Probe(ctx context.Context, path string) (ProbeResult, error)
}

type FFProbe struct {
	Binary string
}

func (p FFProbe) Probe(ctx context.Context, path string) (ProbeResult, error) {
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, p.Binary, "-v", "error", "-print_format", "json", "-show_format", "-show_streams", "--", path)
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return ProbeResult{}, errors.New(msg)
	}
	return parseFFProbe(stdout.Bytes())
}

type ffprobeOutput struct {
	Format struct {
		FormatName string `json:"format_name"`
		Duration   string `json:"duration"`
		BitRate    string `json:"bit_rate"`
	} `json:"format"`
	Streams []struct {
		Index       int               `json:"index"`
		CodecType   string            `json:"codec_type"`
		CodecName   string            `json:"codec_name"`
		Profile     string            `json:"profile"`
		Channels    int               `json:"channels"`
		Width       int               `json:"width"`
		Height      int               `json:"height"`
		PixFmt      string            `json:"pix_fmt"`
		ColorTrc    string            `json:"color_transfer"`
		Tags        map[string]string `json:"tags"`
		Disposition map[string]int    `json:"disposition"`
	} `json:"streams"`
}

func parseFFProbe(data []byte) (ProbeResult, error) {
	var out ffprobeOutput
	if err := json.Unmarshal(data, &out); err != nil {
		return ProbeResult{}, fmt.Errorf("decode ffprobe output: %w", err)
	}
	res := ProbeResult{Container: out.Format.FormatName, Streams: []Stream{}}
	res.Duration, _ = strconv.ParseFloat(out.Format.Duration, 64)
	res.Bitrate, _ = strconv.ParseInt(out.Format.BitRate, 10, 64)
	for _, s := range out.Streams {
		// Cover art is exposed as a video stream with the attached_pic disposition.
		if s.Disposition["attached_pic"] == 1 {
			continue
		}
		if s.CodecType != "video" && s.CodecType != "audio" && s.CodecType != "subtitle" {
			continue
		}
		res.Streams = append(res.Streams, Stream{
			Index:         s.Index,
			Type:          s.CodecType,
			Codec:         s.CodecName,
			Profile:       s.Profile,
			Language:      s.Tags["language"],
			Title:         s.Tags["title"],
			Default:       s.Disposition["default"] == 1,
			Forced:        s.Disposition["forced"] == 1,
			Channels:      s.Channels,
			Width:         s.Width,
			Height:        s.Height,
			PixFmt:        s.PixFmt,
			ColorTransfer: s.ColorTrc,
			ImageBased:    s.CodecType == "subtitle" && imageSubtitleCodecs[s.CodecName],
		})
	}
	return res, nil
}
