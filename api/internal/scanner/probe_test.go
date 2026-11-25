package scanner

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestFFProbeOnGeneratedFile(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	path := filepath.Join(t.TempDir(), "clip.mkv")
	gen := exec.Command(ffmpeg, "-v", "error",
		"-f", "lavfi", "-i", "testsrc=duration=2:size=320x240:rate=25",
		"-f", "lavfi", "-i", "sine=duration=2",
		"-c:v", "libx264", "-c:a", "aac", "-metadata:s:a:0", "language=fra", path)
	if out, err := gen.CombinedOutput(); err != nil {
		t.Fatalf("generate clip: %v\n%s", err, out)
	}

	res, err := FFProbe{Binary: "ffprobe"}.Probe(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if res.Duration < 1.9 || res.Container != "matroska,webm" || len(res.Streams) != 2 {
		t.Fatalf("unexpected probe result %+v", res)
	}
	v, a := res.Streams[0], res.Streams[1]
	if v.Type != "video" || v.Codec != "h264" || v.Width != 320 {
		t.Errorf("unexpected video stream %+v", v)
	}
	if a.Type != "audio" || a.Codec != "aac" || a.Language != "fra" {
		t.Errorf("unexpected audio stream %+v", a)
	}
}

func TestParseFFProbeFlagsImageSubtitlesAndSkipsCovers(t *testing.T) {
	res, err := parseFFProbe([]byte(`{
		"format": {"format_name": "matroska,webm", "duration": "5400.5", "bit_rate": "8000000"},
		"streams": [
			{"index": 0, "codec_type": "video", "codec_name": "hevc", "pix_fmt": "yuv420p10le", "width": 1920, "height": 1080, "disposition": {"default": 1}},
			{"index": 1, "codec_type": "audio", "codec_name": "eac3", "channels": 6, "tags": {"language": "fre", "title": "VFF"}},
			{"index": 2, "codec_type": "subtitle", "codec_name": "hdmv_pgs_subtitle", "tags": {"language": "fre"}, "disposition": {"forced": 1}},
			{"index": 3, "codec_type": "subtitle", "codec_name": "subrip", "tags": {"language": "eng"}},
			{"index": 4, "codec_type": "video", "codec_name": "mjpeg", "disposition": {"attached_pic": 1}},
			{"index": 5, "codec_type": "attachment", "codec_name": "ttf"}
		]
	}`))
	if err != nil {
		t.Fatal(err)
	}

	if res.Duration != 5400.5 || res.Bitrate != 8000000 || len(res.Streams) != 4 {
		t.Fatalf("unexpected result %+v", res)
	}
	if !res.Streams[2].ImageBased || !res.Streams[2].Forced || res.Streams[3].ImageBased {
		t.Errorf("subtitle flags wrong: %+v / %+v", res.Streams[2], res.Streams[3])
	}
}

func TestParseFFProbeMasteringDisplay(t *testing.T) {
	res, err := parseFFProbe([]byte(`{
		"streams": [{
			"index": 0, "codec_type": "video", "codec_name": "hevc",
			"color_transfer": "smpte2084"
		}],
		"frames": [{
			"stream_index": 0,
			"side_data_list": [{"side_data_type": "Mastering display metadata"}]
		}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Streams) != 1 || !res.Streams[0].HasMasteringDisplay {
		t.Fatalf("mastering display metadata not detected: %+v", res.Streams)
	}
}
