package playback

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"toto/api/internal/scanner"
)

var browser = ClientCaps{VideoCodecs: []string{"h264", "hevc"}, AudioCodecs: []string{"aac", "mp3", "opus"}}

func TestDecide(t *testing.T) {
	h264 := &scanner.Stream{Index: 0, Codec: "h264", PixFmt: "yuv420p", Width: 1920, Height: 800}
	hevcHDR := &scanner.Stream{Index: 0, Codec: "hevc", PixFmt: "yuv420p10le", Width: 3840, Height: 2160, ColorTransfer: "smpte2084"}
	xvid := &scanner.Stream{Index: 0, Codec: "mpeg4", Width: 720, Height: 400}
	aac := &scanner.Stream{Index: 1, Codec: "aac", Channels: 2}
	eac3 := &scanner.Stream{Index: 1, Codec: "eac3", Channels: 6}
	pgs := &scanner.Stream{Index: 3, Codec: "hdmv_pgs_subtitle", ImageBased: true}

	cases := []struct {
		name string
		src  Source
		caps ClientCaps
		want Plan
	}{
		{"direct copy", Source{Video: h264, Audio: aac}, browser, Plan{CopyVideo: true, CopyAudio: true}},
		{"5.1 E-AC3 converted to AAC", Source{Video: h264, Audio: eac3}, browser, Plan{CopyVideo: true, AudioChannels: 6}},
		{"HDR tone mapped for an SDR client", Source{Video: hevcHDR, Audio: aac}, browser, Plan{CopyAudio: true, Tonemap: true, Width: 1920, Height: 1080}},
		{"HDR copied for an HDR client", Source{Video: hevcHDR, Audio: aac}, ClientCaps{VideoCodecs: []string{"hevc"}, AudioCodecs: []string{"aac"}, HDR: true}, Plan{CopyVideo: true, CopyAudio: true}},
		{"Xvid transcoded", Source{Video: xvid, Audio: aac}, browser, Plan{CopyAudio: true, Width: 720, Height: 400}},
		{"HEVC transcoded for Firefox", Source{Video: &scanner.Stream{Codec: "hevc", Width: 1920, Height: 1080}, Audio: aac}, ClientCaps{VideoCodecs: []string{"h264"}, AudioCodecs: []string{"aac"}}, Plan{CopyAudio: true, Width: 1920, Height: 1080}},
		{"image subtitles burned in", Source{Video: h264, Audio: aac, Subtitle: pgs}, browser, Plan{CopyAudio: true, BurnSubtitle: true, Width: 1920, Height: 800}},
		{"client height limit", Source{Video: h264, Audio: aac}, ClientCaps{VideoCodecs: []string{"h264"}, AudioCodecs: []string{"aac"}, MaxHeight: 720}, Plan{CopyAudio: true, Width: 1728, Height: 720}},
		{"FLAC when the client cannot decode AAC", Source{Video: h264, Audio: eac3}, ClientCaps{VideoCodecs: []string{"h264"}, AudioCodecs: []string{"flac"}}, Plan{CopyVideo: true, AudioChannels: 6, AudioCodec: "flac"}},
	}
	for _, c := range cases {
		if got := Decide(c.src, c.caps); got != c.want {
			t.Errorf("%s: got %+v, want %+v", c.name, got, c.want)
		}
	}
	if w, h := outputSize(3840, 1600, 0); w != 2592 || h != 1080 {
		t.Errorf("scope 4K must scale to 2592x1080, got %dx%d", w, h)
	}
}

func TestShiftVTT(t *testing.T) {
	in := "WEBVTT\n\n00:00:05.000 --> 00:00:07.000\nbefore\n\n00:01:58.500 --> 00:02:01.250\nacross\n\n01:00:00.000 --> 01:00:02.000 align:start\nafter\n"
	var out bytes.Buffer
	if err := ShiftVTT(strings.NewReader(in), &out, 120); err != nil {
		t.Fatal(err)
	}
	want := "WEBVTT\n\n00:00:00.000 --> 00:00:01.250\nacross\n\n00:58:00.000 --> 00:58:02.000 align:start\nafter\n"
	if out.String() != want {
		t.Errorf("got:\n%s\nwant:\n%s", out.String(), want)
	}
}

// sample generates a 12 s file whose codecs are chosen by the caller.
func sample(t *testing.T, ffmpeg, name string, extra ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	srt := filepath.Join(t.TempDir(), "s.srt")
	if err := os.WriteFile(srt, []byte("1\n00:00:01,000 --> 00:00:03,000\nBonjour\n\n2\n00:00:09,000 --> 00:00:11,000\nAu revoir\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	args := []string{"-v", "error", "-f", "lavfi", "-i", "testsrc2=duration=12:size=640x360:rate=25",
		"-f", "lavfi", "-i", "sine=duration=12", "-i", srt, "-map", "0", "-map", "1", "-map", "2"}
	args = append(args, extra...)
	args = append(args, "-c:s", "srt", "-g", "50", path)
	if out, err := exec.Command(ffmpeg, args...).CombinedOutput(); err != nil {
		t.Fatalf("generate sample: %v\n%s", err, out)
	}
	return path
}

func probe(t *testing.T, path string) Source {
	t.Helper()
	res, err := scanner.FFProbe{Binary: "ffprobe"}.Probe(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	src := Source{Path: path, Duration: res.Duration}
	for i := range res.Streams {
		s := &res.Streams[i]
		switch {
		case s.Type == "video" && src.Video == nil:
			src.Video = s
		case s.Type == "audio" && src.Audio == nil:
			src.Audio = s
		}
	}
	return src
}

func play(t *testing.T, m *Manager, src Source, start float64) *Session {
	t.Helper()
	sess, err := m.Start(context.Background(), StartRequest{UserID: uuid.New(), FileID: uuid.New(), Source: src, Caps: browser, Start: start})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	for _, f := range []string{"index.m3u8", "init.mp4", "seg_00000.m4s"} {
		if _, err := sess.File(ctx, f); err != nil {
			t.Fatalf("%s: %v (stderr: %s)", f, err, sess.stderr.String())
		}
	}
	return sess
}

func TestSessionsWithFFmpeg(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	h264 := sample(t, ffmpeg, "h264.mkv", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac")
	xvid := sample(t, ffmpeg, "xvid.mkv", "-c:v", "mpeg4", "-c:a", "ac3")

	encoders := []Encoder{{Accel: AccelNone}}
	if hw := DetectEncoder(context.Background(), ffmpeg, "auto"); hw.Accel != AccelNone {
		encoders = append(encoders, hw)
	}
	for _, enc := range encoders {
		t.Run(string(enc.Accel), func(t *testing.T) {
			m, err := NewManager(ffmpeg, "ffprobe", enc, t.TempDir(), 2)
			if err != nil {
				t.Fatal(err)
			}

			copied := play(t, m, probe(t, h264), 0)
			if !copied.Plan.CopyVideo || !copied.Plan.CopyAudio {
				t.Errorf("H.264/AAC must be copied: %+v", copied.Plan)
			}

			resumed := play(t, m, probe(t, h264), 5.2)
			if resumed.Start != 4 {
				t.Errorf("copy must start on the keyframe at 4 s, got %.3f", resumed.Start)
			}

			src := probe(t, xvid)
			transcoded := play(t, m, src, 3)
			if transcoded.Plan.CopyVideo || transcoded.Plan.CopyAudio || transcoded.Start != 3 {
				t.Errorf("Xvid/AC3 must be transcoded from 3 s exactly: %+v start=%.3f", transcoded.Plan, transcoded.Start)
			}

			dir := transcoded.dir
			m.Stop(transcoded.ID)
			if _, err := os.Stat(dir); !os.IsNotExist(err) {
				t.Error("stopping a session must remove its segments")
			}
			if _, err := m.Get(transcoded.ID); err != ErrNotFound {
				t.Error("stopped session must be forgotten")
			}
			m.StopAll()
		})
	}
}

func TestSubtitleExtraction(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	path := sample(t, ffmpeg, "subs.mkv", "-c:v", "libx264", "-c:a", "aac")

	// Live: the session writes the track alongside the segments, aligned on the stream start.
	m, err := NewManager(ffmpeg, "ffprobe", Encoder{Accel: AccelNone}, t.TempDir(), 1)
	if err != nil {
		t.Fatal(err)
	}
	src := probe(t, path)
	src.TextSubtitle = &scanner.Stream{Index: 2, Type: "subtitle", Codec: "subrip"}
	sess := play(t, m, src, 8)
	deadline := time.Now().Add(20 * time.Second)
	for {
		if live, ok := sess.LiveSubtitle(); ok {
			data, _ := os.ReadFile(live)
			if strings.Contains(string(data), "Au revoir") {
				// A few tens of milliseconds of muxer offset are tolerated.
				if !strings.Contains(string(data), "00:01.0") {
					t.Errorf("live cues must be relative to the stream start (8 s):\n%s", data)
				}
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("live subtitles never appeared")
		}
		time.Sleep(200 * time.Millisecond)
	}
	m.StopAll()

	subs, err := NewSubtitles(ffmpeg, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	vtt, err := subs.Wait(ctx, uuid.New(), 1, path, 2)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(vtt)
	if !strings.HasPrefix(string(data), "WEBVTT") || !strings.Contains(string(data), "Au revoir") {
		t.Errorf("unexpected WebVTT:\n%s", data)
	}
}
