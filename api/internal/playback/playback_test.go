package playback

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
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
		{"Dolby passthrough remains copied", Source{Audio: eac3}, ClientCaps{AudioCodecs: []string{"aac", "eac3"}, PreferredAudioCodec: "ac3"}, Plan{CopyAudio: true}},
		{"DTS 5.1 converted to HDMI AC3", Source{Audio: &scanner.Stream{Codec: "dts", Channels: 6}}, ClientCaps{AudioCodecs: []string{"aac", "ac3"}, PreferredAudioCodec: "ac3"}, Plan{AudioChannels: 6, AudioCodec: "ac3"}},
		{"DTS 5.1 converted to HDMI EAC3", Source{Audio: &scanner.Stream{Codec: "dts", Channels: 6}}, ClientCaps{AudioCodecs: []string{"aac", "eac3"}, PreferredAudioCodec: "eac3"}, Plan{AudioChannels: 6, AudioCodec: "eac3"}},
		{"AAC 5.1 converted for direct HDMI output", Source{Audio: &scanner.Stream{Codec: "aac", Channels: 6}}, ClientCaps{AudioCodecs: []string{"aac", "ac3"}, PreferredAudioCodec: "ac3"}, Plan{AudioChannels: 6, AudioCodec: "ac3"}},
		{"stereo AAC stays copied", Source{Audio: aac}, ClientCaps{AudioCodecs: []string{"aac", "ac3"}, PreferredAudioCodec: "ac3"}, Plan{CopyAudio: true}},
		{"7.1 transcode remains capped at six channels", Source{Audio: &scanner.Stream{Codec: "truehd", Channels: 8}}, ClientCaps{AudioCodecs: []string{"aac", "ac3"}, PreferredAudioCodec: "ac3"}, Plan{AudioChannels: 6, AudioCodec: "ac3"}},
		{"stereo stays AAC when transcoding", Source{Audio: &scanner.Stream{Codec: "dts", Channels: 2}}, ClientCaps{AudioCodecs: []string{"aac", "ac3"}, PreferredAudioCodec: "ac3"}, Plan{AudioChannels: 2}},
		{"unsupported preference ignored", Source{Audio: &scanner.Stream{Codec: "dts", Channels: 6}}, ClientCaps{AudioCodecs: []string{"aac"}, PreferredAudioCodec: "ac3"}, Plan{AudioChannels: 6}},
		{"unrecognized preference ignored", Source{Audio: &scanner.Stream{Codec: "dts", Channels: 6}}, ClientCaps{AudioCodecs: []string{"aac"}, PreferredAudioCodec: "unexpected"}, Plan{AudioChannels: 6}},
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

func TestDolbyTranscodingPreservesSixChannels(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	input := filepath.Join(t.TempDir(), "surround.flac")
	if output, err := exec.Command(ffmpeg, "-v", "error", "-f", "lavfi", "-i", "anullsrc=r=48000:cl=5.1", "-t", "0.5", "-c:a", "flac", input).CombinedOutput(); err != nil {
		t.Fatalf("generate surround input: %v %s", err, output)
	}
	for _, codec := range []string{"ac3", "eac3"} {
		t.Run(codec, func(t *testing.T) {
			source := probe(t, input)
			plan := Decide(source, ClientCaps{AudioCodecs: []string{"aac", "flac", codec}, PreferredAudioCodec: codec})
			if plan.CopyAudio || plan.AudioChannels != 6 || plan.AudioCodec != codec {
				t.Fatalf("plan: %+v", plan)
			}
			directory := t.TempDir()
			if output, err := exec.Command(ffmpeg, Args(plan, source, 0, Encoder{Accel: AccelNone}, directory)...).CombinedOutput(); err != nil {
				t.Fatalf("transcode surround: %v %s", err, output)
			}
			init, err := os.ReadFile(filepath.Join(directory, "init.mp4"))
			if err != nil {
				t.Fatal(err)
			}
			segment, err := os.ReadFile(filepath.Join(directory, "seg_00000.m4s"))
			if err != nil {
				t.Fatal(err)
			}
			combined := filepath.Join(t.TempDir(), "audio.mp4")
			if err := os.WriteFile(combined, append(init, segment...), 0o600); err != nil {
				t.Fatal(err)
			}
			output := probe(t, combined)
			if output.Audio == nil || output.Audio.Codec != codec || output.Audio.Channels != 6 {
				t.Fatalf("output audio: %+v", output.Audio)
			}
		})
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
			select {
			case <-copied.done:
			case <-time.After(5 * time.Second):
				t.Fatal("FFmpeg did not finish producing a short copied stream")
			}
			if items := m.Diagnostics(); len(items) != 1 || items[0].Process != ProcessFinished ||
				len(items[0].Reasons) != 0 || items[0].VideoAccel != "" || items[0].OutputAudioCodec != "" {
				t.Errorf("unexpected copy diagnostics: %+v", items)
			}
			if !copied.progress.sampled() {
				t.Error("FFmpeg progress was not parsed")
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
			if d := transcoded.diagnostic(); d.VideoAccel != enc.Accel || d.OutputAudioCodec != "aac" ||
				d.Source.Video.Codec != "mpeg4" || d.Source.Audio.Codec != "ac3" || len(d.Reasons) != 3 {
				t.Errorf("unexpected transcode diagnostics: %+v", d)
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

// TestSeekStartsEveryStreamOnTheKeyframe covers sessions started past zero: with B-frames, ffmpeg seeks
// before -ss, so asked for the keyframe itself it landed on the previous one. A copied stream then started
// seconds early while a decoded one started on time, putting audio and video seconds apart.
func TestSeekStartsEveryStreamOnTheKeyframe(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	input := filepath.Join(t.TempDir(), "movie.mkv")
	// Keyframes every 2 s, B-frames on (x264 default), so 6 s is a keyframe preceded by another at 4 s.
	if output, err := exec.Command(ffmpeg, "-v", "error", "-f", "lavfi", "-i", "testsrc2=duration=12:size=320x180:rate=25",
		"-f", "lavfi", "-i", "sine=duration=12", "-c:v", "libx264", "-preset", "ultrafast", "-tune", "film", "-bf", "3",
		"-g", "50", "-keyint_min", "50", "-sc_threshold", "0", "-c:a", "aac", input).CombinedOutput(); err != nil {
		t.Fatalf("generate input: %v %s", err, output)
	}
	source := probe(t, input)
	cases := map[string]ClientCaps{
		"copy video and audio":                      {VideoCodecs: []string{"h264"}, AudioCodecs: []string{"aac"}},
		"copy video, transcode audio":               {VideoCodecs: []string{"h264"}, AudioCodecs: []string{"opus"}},
		"transcode video, copy audio":               {VideoCodecs: []string{"av1"}, AudioCodecs: []string{"aac"}},
		"transcode video and audio (no seek nudge)": {VideoCodecs: []string{"av1"}, AudioCodecs: []string{"opus"}},
	}
	for name, caps := range cases {
		t.Run(name, func(t *testing.T) {
			plan := Decide(source, caps)
			directory := t.TempDir()
			if output, err := exec.Command(ffmpeg, Args(plan, source, 6, Encoder{Accel: AccelNone}, directory)...).CombinedOutput(); err != nil {
				t.Fatalf("plan %+v: %v %s", plan, err, output)
			}
			segments, _ := filepath.Glob(filepath.Join(directory, "seg_*.m4s"))
			combined := filepath.Join(t.TempDir(), "out.mp4")
			data, err := os.ReadFile(filepath.Join(directory, "init.mp4"))
			if err != nil {
				t.Fatal(err)
			}
			for _, segment := range segments {
				chunk, err := os.ReadFile(segment)
				if err != nil {
					t.Fatal(err)
				}
				data = append(data, chunk...)
			}
			if err := os.WriteFile(combined, data, 0o600); err != nil {
				t.Fatal(err)
			}
			// The session starts at 6 s of a 12 s file: both streams must hold the last 6 s, not 8.
			for _, stream := range []string{"v", "a"} {
				if span := streamSpan(t, combined, stream); span < 5.7 || span > 6.3 {
					t.Errorf("plan %+v: stream %s spans %.2f s, want 6 s", plan, stream, span)
				}
			}
		})
	}
}

// streamSpan returns the presentation time covered by one stream (v or a) of a media file.
func streamSpan(t *testing.T, path, stream string) float64 {
	t.Helper()
	out, err := exec.Command("ffprobe", "-v", "error", "-select_streams", stream, "-show_entries", "frame=pts_time",
		"-of", "csv=p=0", path).Output()
	if err != nil {
		t.Fatal(err)
	}
	first, last := 1e9, -1e9
	for _, line := range strings.Fields(string(out)) {
		var v float64
		if _, err := fmt.Sscanf(strings.TrimSuffix(line, ","), "%f", &v); err == nil {
			first, last = min(first, v), max(last, v)
		}
	}
	return last - first
}

// Players ignore fMP4 edit lists, so the gap between the start of the video and the start of the audio must be
// carried by the timestamps themselves: otherwise the sound plays ahead of the picture.
func TestStreamKeepsAudioVideoOffsetWithoutEditList(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	dir := t.TempDir()
	plain := filepath.Join(dir, "plain.mkv")
	if output, err := exec.Command(ffmpeg, "-v", "error", "-f", "lavfi", "-i", "testsrc=s=320x240:r=24", "-f", "lavfi", "-i", "sine=r=48000",
		"-t", "3", "-c:v", "libx264", "-bf", "3", "-c:a", "aac", plain).CombinedOutput(); err != nil {
		t.Fatalf("generate input: %v %s", err, output)
	}
	// The audio starts 300 ms after the video, as in many remuxes.
	input := filepath.Join(dir, "delayed.mkv")
	if output, err := exec.Command(ffmpeg, "-v", "error", "-i", plain, "-itsoffset", "0.3", "-i", plain,
		"-map", "0:v", "-map", "1:a", "-c", "copy", input).CombinedOutput(); err != nil {
		t.Fatalf("delay audio: %v %s", err, output)
	}

	source := probe(t, input)
	for name, caps := range map[string]ClientCaps{
		"transcode": {VideoCodecs: []string{"h264"}, AudioCodecs: []string{"opus"}, MaxHeight: 120},
		"copy":      browser,
	} {
		t.Run(name, func(t *testing.T) {
			out := t.TempDir()
			if output, err := exec.Command(ffmpeg, Args(Decide(source, caps), source, 0, Encoder{Accel: AccelNone}, out)...).CombinedOutput(); err != nil {
				t.Fatalf("stream: %v %s", err, output)
			}
			init, err := os.ReadFile(filepath.Join(out, "init.mp4"))
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Contains(init, []byte("elst")) {
				t.Fatal("init.mp4 has an edit list")
			}
			segment, err := os.ReadFile(filepath.Join(out, "seg_00000.m4s"))
			if err != nil {
				t.Fatal(err)
			}
			flat := filepath.Join(out, "flat.mp4")
			if err := os.WriteFile(flat, append(init, segment...), 0o600); err != nil {
				t.Fatal(err)
			}
			output, err := exec.Command("ffprobe", "-v", "error", "-ignore_editlist", "1", "-show_entries", "packet=stream_index,pts_time",
				"-of", "csv=p=0", flat).Output()
			if err != nil {
				t.Fatal(err)
			}
			// The first audio packet may absorb the gap in its duration: the second one shows where the audio really is.
			var video, audio []float64
			for _, line := range strings.Fields(string(output)) {
				var index int
				var pts float64
				if _, err := fmt.Sscanf(strings.ReplaceAll(line, ",", " "), "%d %f", &index, &pts); err != nil {
					continue
				}
				if index == 0 {
					video = append(video, pts)
				} else {
					audio = append(audio, pts)
				}
			}
			if len(video) == 0 || len(audio) < 2 {
				t.Fatalf("packets: %s", output)
			}
			firstVideo := video[0]
			for _, v := range video {
				firstVideo = min(firstVideo, v)
			}
			if gap := audio[1] - firstVideo; gap < 0.25 || gap > 0.4 {
				t.Fatalf("audio starts %.3f s after the video, want ~0.3 s", gap)
			}
		})
	}
}

func TestToneMappingRunsAtOutputSize(t *testing.T) {
	hdr := &scanner.Stream{Codec: "hevc", PixFmt: "yuv420p10le", Width: 3840, Height: 2160, ColorTransfer: "smpte2084"}
	plan := Decide(Source{Video: hdr}, browser)
	filter := videoFilter(plan, hdr, AccelNone, false)
	if !strings.HasPrefix(filter, "scale=1920:1080,zscale=t=linear") {
		t.Fatalf("4K HDR must be downscaled before the float tone mapping: %s", filter)
	}
	if filter = videoFilter(plan, hdr, AccelVAAPI, false); !strings.HasPrefix(filter, "scale=1920:1080,zscale=t=linear") {
		t.Fatalf("VAAPI tone maps on the CPU, after downscaling: %s", filter)
	}
	if filter = videoFilter(plan, hdr, AccelQSV, false); !strings.HasPrefix(filter, "scale=1920:1080,zscale=t=linear") ||
		strings.Contains(filter, "tonemap=1") || !strings.Contains(filter, "format=nv12,hwupload") {
		t.Fatalf("QSV must upload SDR frames after software tone mapping: %s", filter)
	}

	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	input := filepath.Join(t.TempDir(), "hdr.mkv")
	if output, err := exec.Command(ffmpeg, "-v", "error", "-f", "lavfi", "-i", "testsrc2=s=640x360:r=24", "-t", "1",
		"-pix_fmt", "yuv420p10le", "-c:v", "libx265",
		"-x265-params", "log-level=error:colorprim=bt2020:transfer=smpte2084:colormatrix=bt2020nc:range=limited", input).CombinedOutput(); err != nil {
		t.Fatalf("generate HDR input: %v %s", err, output)
	}
	source := probe(t, input)
	plan = Decide(source, ClientCaps{VideoCodecs: []string{"h264"}, AudioCodecs: []string{"aac"}, MaxHeight: 180})
	if !plan.Tonemap || plan.Height != 180 {
		t.Fatalf("plan: %+v", plan)
	}
	out := t.TempDir()
	if output, err := exec.Command(ffmpeg, Args(plan, source, 0, Encoder{Accel: AccelNone}, out)...).CombinedOutput(); err != nil {
		t.Fatalf("tone map: %v %s", err, output)
	}
	init, err := os.ReadFile(filepath.Join(out, "init.mp4"))
	if err != nil {
		t.Fatal(err)
	}
	segment, err := os.ReadFile(filepath.Join(out, "seg_00000.m4s"))
	if err != nil {
		t.Fatal(err)
	}
	flat := filepath.Join(out, "flat.mp4")
	if err := os.WriteFile(flat, append(init, segment...), 0o600); err != nil {
		t.Fatal(err)
	}
	video := probe(t, flat).Video
	if video == nil || video.Codec != "h264" || video.Width != 320 || video.Height != 180 || video.PixFmt != "yuv420p" {
		t.Fatalf("output video: %+v", video)
	}
}

func TestVideoPipelineMatchesFFmpegRoute(t *testing.T) {
	tests := []struct {
		name     string
		source   Source
		plan     Plan
		encoder  Encoder
		want     VideoPipeline
		wantArgs []string
	}{
		{
			name:     "software",
			source:   Source{Video: &scanner.Stream{Codec: "h264"}},
			plan:     Plan{Width: 1280, Height: 720},
			encoder:  Encoder{Accel: AccelNone},
			want:     VideoPipeline{Decode: ProcessingCPU, Scale: ProcessingCPU, Encode: ProcessingCPU},
			wantArgs: []string{"-c:v libx264"},
		},
		{
			name:     "qsv hardware decode",
			source:   Source{Video: &scanner.Stream{Codec: "h264"}},
			plan:     Plan{Width: 1280, Height: 720},
			encoder:  Encoder{Accel: AccelQSV},
			want:     VideoPipeline{Decode: ProcessingQSV, Scale: ProcessingQSV, Encode: ProcessingQSV},
			wantArgs: []string{"-hwaccel qsv", "-c:v h264_qsv"},
		},
		{
			name:     "qsv software hevc decode",
			source:   Source{Video: &scanner.Stream{Codec: "hevc"}},
			plan:     Plan{Width: 1280, Height: 720},
			encoder:  Encoder{Accel: AccelQSV},
			want:     VideoPipeline{Decode: ProcessingCPU, Scale: ProcessingQSV, Encode: ProcessingQSV},
			wantArgs: []string{"-c:v h264_qsv"},
		},
		{
			name:     "software tone mapping with qsv encode",
			source:   Source{Video: &scanner.Stream{Codec: "hevc"}},
			plan:     Plan{Tonemap: true, Width: 1280, Height: 720},
			encoder:  Encoder{Accel: AccelQSV},
			want:     VideoPipeline{Decode: ProcessingCPU, ToneMap: ProcessingCPU, Scale: ProcessingCPU, Encode: ProcessingQSV},
			wantArgs: []string{"zscale=t=linear", "-c:v h264_qsv"},
		},
		{
			name: "vaapi hdr route",
			source: Source{Video: &scanner.Stream{
				Codec: "hevc", HasMasteringDisplay: true,
			}},
			plan: Plan{Tonemap: true, Width: 1280, Height: 720},
			encoder: Encoder{
				Accel: AccelVAAPI, HEVCVAAPI: true, VAAPITonemap: true,
			},
			want: VideoPipeline{
				Decode: ProcessingVAAPI, ToneMap: ProcessingVAAPI, Scale: ProcessingVAAPI, Encode: ProcessingVAAPI,
			},
			wantArgs: []string{"-hwaccel vaapi", "tonemap_vaapi", "-c:v h264_vaapi"},
		},
		{
			name:    "vulkan hdr fallback",
			source:  Source{Video: &scanner.Stream{Codec: "hevc"}},
			plan:    Plan{Tonemap: true, Width: 1280, Height: 720},
			encoder: Encoder{Accel: AccelVAAPI, HEVCVAAPI: true, VulkanTonemap: true},
			want: VideoPipeline{
				Decode: ProcessingVAAPI, ToneMap: ProcessingVulkan, Scale: ProcessingVulkan, Encode: ProcessingVAAPI,
			},
			wantArgs: []string{"-hwaccel vaapi", "libplacebo", "-c:v h264_vaapi"},
		},
		{
			name: "subtitle burn-in",
			source: Source{
				Video: &scanner.Stream{Codec: "h264"}, Subtitle: &scanner.Stream{Index: 2},
			},
			plan:     Plan{BurnSubtitle: true, Width: 1280, Height: 720},
			encoder:  Encoder{Accel: AccelQSV},
			want:     VideoPipeline{Decode: ProcessingCPU, BurnSubtitle: ProcessingCPU, Scale: ProcessingQSV, Encode: ProcessingQSV},
			wantArgs: []string{"overlay=eof_action=pass", "-c:v h264_qsv"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.encoder.VideoPipeline(tt.plan, tt.source)
			if got == nil {
				t.Fatal("pipeline is nil")
			}
			if got.Decode != tt.want.Decode || got.ToneMap != tt.want.ToneMap ||
				got.BurnSubtitle != tt.want.BurnSubtitle || got.Scale != tt.want.Scale ||
				got.Encode != tt.want.Encode {
				t.Fatalf("pipeline = %+v, want %+v", *got, tt.want)
			}
			args := strings.Join(Args(tt.plan, tt.source, 0, tt.encoder, "/out"), " ")
			for _, fragment := range tt.wantArgs {
				if !strings.Contains(args, fragment) {
					t.Errorf("FFmpeg args missing %q: %s", fragment, args)
				}
			}
		})
	}

	if got := (Encoder{Accel: AccelVAAPI}).VideoPipeline(Plan{CopyVideo: true}, Source{Video: &scanner.Stream{Codec: "h264"}}); got != nil {
		t.Errorf("copied video must not have a transcode pipeline: %+v", got)
	}
}

func TestQSVDecodeAndKeyframeArgs(t *testing.T) {
	for _, c := range []struct {
		codec  string
		hdr    bool
		burn   bool
		decode bool
	}{
		{"hevc", false, false, false},
		{"hevc", true, false, false},
		{"h264", false, false, true},
		{"av1", false, false, true},
		{"av1", true, false, false},
		{"h264", false, true, false},
	} {
		t.Run(fmt.Sprintf("%s/hdr=%t/burn=%t", c.codec, c.hdr, c.burn), func(t *testing.T) {
			v := &scanner.Stream{Codec: c.codec, PixFmt: "yuv420p10le", Width: 3840, Height: 2160}
			src := Source{Video: v, Subtitle: &scanner.Stream{Index: 2}}
			plan := Plan{Tonemap: c.hdr, BurnSubtitle: c.burn, Width: 1920, Height: 1080}
			args := strings.Join(Args(plan, src, 4, Encoder{Accel: AccelQSV, Device: "/dev/dri/renderD128"}, "/out"), " ")
			if strings.Contains(args, "-hwaccel qsv") != c.decode {
				t.Fatalf("unexpected decoding path: %s", args)
			}
			if !strings.Contains(args, "-forced_idr 1") {
				t.Fatalf("QSV needs forced IDR frames for independent HLS segments: %s", args)
			}
			if c.hdr && !strings.Contains(args, "-color_primaries bt709 -color_trc bt709 -colorspace bt709 -color_range tv") {
				t.Fatalf("tone-mapped output must be tagged SDR: %s", args)
			}
		})
	}
	src := Source{Video: &scanner.Stream{Codec: "hevc"}}
	args := strings.Join(Args(Plan{CopyVideo: true}, src, 0, Encoder{Accel: AccelQSV}, "/out"), " ")
	if strings.Contains(args, "hwaccel") || strings.Contains(args, "forced_idr") || strings.Contains(args, "bt709") {
		t.Fatalf("copied video must not be changed: %s", args)
	}
	src.Video.ColorTransfer = "smpte2084"
	src.Video.HasMasteringDisplay = true
	enc := Encoder{Accel: AccelQSV, Device: "/dev/dri/renderD128", HEVCVAAPI: true, VulkanTonemap: true}
	args = strings.Join(Args(Plan{Tonemap: true, Width: 1920, Height: 1080}, src, 0, enc, "/out"), " ")
	for _, want := range []string{
		"-hwaccel vaapi", "libplacebo=w=1920:h=1080", "h264_vaapi", "-bsf:v filter_units=remove_types=6",
		"-color_primaries bt709 -color_trc bt709 -colorspace bt709 -color_range tv",
	} {
		if !strings.Contains(args, want) {
			t.Fatalf("GPU HDR path missing %q: %s", want, args)
		}
	}
	if strings.Contains(args, "h264_qsv") || strings.Contains(gpuHDRFilter(Plan{}, enc, false), "sidedata=") {
		t.Fatalf("GPU HDR path must encode on VAAPI without software frame filters: %s", args)
	}
	enc.VAAPITonemap = true
	args = strings.Join(Args(Plan{Tonemap: true, Width: 1920, Height: 1080}, src, 0, enc, "/out"), " ")
	if !strings.Contains(args, "tonemap_vaapi=format=nv12:matrix=bt709:primaries=bt709:transfer=bt709") ||
		strings.Contains(args, "libplacebo") || enc.VideoAccel(Plan{Tonemap: true}, src) != AccelVAAPI {
		t.Fatalf("HDR10 with mastering metadata should prefer VAAPI VPP: %s", args)
	}
	src.Video.HasMasteringDisplay = false
	enc.VulkanTonemap = false
	if gpuHDR(Plan{Tonemap: true}, src, enc) {
		t.Fatal("VAAPI tone mapping must not run without mastering display metadata")
	}
	enc.VulkanTonemap = true
	if !gpuHDR(Plan{Tonemap: true}, src, enc) {
		t.Fatal("Vulkan tone mapping should handle HDR sources without mastering metadata")
	}
	args = strings.Join(Args(Plan{Tonemap: true, Width: 1920, Height: 1080}, src, 0, enc, "/out"), " ")
	if !strings.Contains(args, "libplacebo") || !strings.Contains(args, "vulkan=vk@va") ||
		strings.Contains(args, "tonemap_vaapi") {
		t.Fatalf("HDR without mastering metadata must select the validated Vulkan route: %s", args)
	}
}

func TestHDRHLSCadenceColorsAndTiming(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	input := filepath.Join(t.TempDir(), "hdr.mkv")
	if output, err := exec.Command(ffmpeg, "-v", "error",
		"-f", "lavfi", "-i", "testsrc2=s=320x180:r=24000/1001",
		"-f", "lavfi", "-i", "sine=r=48000", "-t", "12.012",
		"-pix_fmt", "yuv420p10le", "-c:v", "libx265", "-preset", "ultrafast", "-g", "48",
		"-x265-params", "pools=2:log-level=error:colorprim=bt2020:transfer=smpte2084:colormatrix=bt2020nc:range=limited:master-display=G(13250,34500)B(7500,3000)R(34000,16000)WP(15635,16450)L(40000000,50):max-cll=1000,400",
		"-c:a", "ac3", input).CombinedOutput(); err != nil {
		t.Fatalf("generate HDR input: %v %s", err, output)
	}
	source := probe(t, input)
	if !source.Video.HasMasteringDisplay {
		t.Fatal("scanner did not detect frame-level HDR mastering metadata")
	}
	plan := Decide(source, ClientCaps{VideoCodecs: []string{"h264"}, AudioCodecs: []string{"flac"}})
	var gpu []Encoder
	for _, accel := range []Accel{AccelNone, AccelQSV, AccelVAAPI} {
		t.Run(string(accel), func(t *testing.T) {
			enc := Encoder{Accel: AccelNone}
			if accel != AccelNone {
				enc = DetectEncoder(context.Background(), ffmpeg, string(accel))
				if enc.Accel != accel {
					t.Skipf("%s encoder unavailable", accel)
				}
			}
			if enc.HEVCVAAPI && (enc.VAAPITonemap || enc.VulkanTonemap) {
				gpu = append(gpu, enc)
			}
			for _, start := range []float64{0, 4} {
				t.Run(fmt.Sprintf("start=%.0f", start), func(t *testing.T) {
					out := t.TempDir()
					output, err := exec.Command(ffmpeg, Args(plan, source, start, enc, out)...).CombinedOutput()
					if err != nil || bytes.Contains(output, []byte("Non-monotonic DTS")) {
						t.Fatalf("HDR transcode: %v %s", err, output)
					}
					playlist, err := os.ReadFile(filepath.Join(out, "index.m3u8"))
					if err != nil {
						t.Fatal(err)
					}
					var durations []float64
					for _, line := range strings.Split(string(playlist), "\n") {
						var duration float64
						if _, err := fmt.Sscanf(line, "#EXTINF:%f,", &duration); err == nil {
							durations = append(durations, duration)
						}
					}
					if len(durations) != int((12-start)/SegmentSeconds) {
						t.Fatalf("unexpected segment count: %s", playlist)
					}
					for _, duration := range durations {
						if math.Abs(duration-SegmentSeconds) > 0.05 {
							t.Fatalf("segments must be ~4 seconds: %s", playlist)
						}
					}
					data, err := os.ReadFile(filepath.Join(out, "init.mp4"))
					if err != nil {
						t.Fatal(err)
					}
					if bytes.Contains(data, []byte("elst")) {
						t.Fatal("HLS must not rely on edit lists for A/V sync")
					}
					for i := range durations {
						chunk, err := os.ReadFile(filepath.Join(out, fmt.Sprintf("seg_%05d.m4s", i)))
						if err != nil {
							t.Fatal(err)
						}
						data = append(data, chunk...)
					}
					flat := filepath.Join(out, "flat.mp4")
					if err := os.WriteFile(flat, data, 0o600); err != nil {
						t.Fatal(err)
					}
					output, err = exec.Command("ffprobe", "-v", "error", "-select_streams", "v", "-count_frames",
						"-show_streams", "-of", "json", flat).CombinedOutput()
					if err != nil {
						t.Fatalf("probe video: %v %s", err, output)
					}
					var video struct {
						Streams []struct {
							Frames    string `json:"nb_read_frames"`
							Space     string `json:"color_space"`
							Transfer  string `json:"color_transfer"`
							Primaries string `json:"color_primaries"`
							SideData  []struct {
								Type string `json:"side_data_type"`
							} `json:"side_data_list"`
						} `json:"streams"`
					}
					if err := json.Unmarshal(output, &video); err != nil {
						t.Fatal(err)
					}
					if len(video.Streams) != 1 {
						t.Fatalf("unexpected video: %s", output)
					}
					v := video.Streams[0]
					if v.Frames != fmt.Sprint(int(288-start*24)) || v.Space != "bt709" ||
						v.Transfer != "bt709" || v.Primaries != "bt709" {
						t.Fatalf("wrong cadence or SDR tags: %s", output)
					}
					for _, side := range v.SideData {
						if strings.Contains(side.Type, "Mastering display") || strings.Contains(side.Type, "Content light") ||
							strings.Contains(side.Type, "HDR") || strings.Contains(side.Type, "DOVI") {
							t.Fatalf("SDR must not retain HDR metadata: %s", output)
						}
					}
					output, err = exec.Command("ffprobe", "-v", "error", "-show_entries",
						"packet=stream_index,pts_time,dts_time", "-of", "json", flat).CombinedOutput()
					if err != nil {
						t.Fatalf("probe timestamps: %v %s", err, output)
					}
					var packets struct {
						Packets []struct {
							Index int    `json:"stream_index"`
							PTS   string `json:"pts_time"`
							DTS   string `json:"dts_time"`
						} `json:"packets"`
					}
					if err := json.Unmarshal(output, &packets); err != nil {
						t.Fatal(err)
					}
					lastDTS := [2]float64{math.Inf(-1), math.Inf(-1)}
					lastPTS := lastDTS
					for _, packet := range packets.Packets {
						if packet.Index < 0 || packet.Index > 1 {
							t.Fatalf("unexpected packet: %+v", packet)
						}
						var pts, dts float64
						if _, err := fmt.Sscan(packet.PTS, &pts); err != nil {
							t.Fatal(err)
						}
						if _, err := fmt.Sscan(packet.DTS, &dts); err != nil {
							t.Fatal(err)
						}
						if dts <= lastDTS[packet.Index] {
							t.Fatalf("non-monotonic timestamps: %+v", packet)
						}
						lastDTS[packet.Index] = dts
						lastPTS[packet.Index] = max(lastPTS[packet.Index], pts)
					}
					if math.IsInf(lastPTS[0], -1) || math.IsInf(lastPTS[1], -1) ||
						math.Abs(lastPTS[0]-lastPTS[1]) > 0.15 {
						t.Fatalf("A/V end timestamps diverge: %v", lastPTS)
					}
				})
			}
		})
	}
	if len(gpu) > 0 {
		t.Run("gpu-tone-map", func(t *testing.T) {
			source := probe(t, input)
			plan := Decide(source, ClientCaps{VideoCodecs: []string{"h264"}, AudioCodecs: []string{"aac"}})
			out := t.TempDir()
			output, err := exec.Command(ffmpeg, Args(plan, source, 0.5, gpu[0], out)...).CombinedOutput()
			if err != nil {
				t.Fatalf("GPU HDR transcode: %v %s", err, output)
			}
			playlist, err := os.ReadFile(filepath.Join(out, "index.m3u8"))
			if err != nil {
				t.Fatal(err)
			}
			output, err = exec.Command(ffmpeg, "-v", "error", "-i", filepath.Join(out, "index.m3u8"),
				"-map", "0:v:0", "-vf", "format=p010le", "-fps_mode", "passthrough", "-f", "framemd5", "-").Output()
			if err != nil {
				t.Fatalf("probe GPU HLS output: %v", err)
			}
			frames, err := parseProbeFrames(string(output))
			if err != nil || len(frames) != 276 {
				t.Fatalf("GPU HLS frames: got %d: %v\n%s\n%s", len(frames), err, playlist, output)
			}
			for i, frame := range frames {
				if frame.pts != int64(i) {
					t.Fatalf("GPU HLS timestamps are discontinuous at frame %d: %+v", i, frame)
				}
			}
			var durations []float64
			for _, line := range strings.Split(string(playlist), "\n") {
				var duration float64
				if _, err := fmt.Sscanf(line, "#EXTINF:%f,", &duration); err == nil {
					durations = append(durations, duration)
				}
			}
			if len(durations) != 3 || math.Abs(durations[0]-SegmentSeconds) > 0.05 ||
				math.Abs(durations[1]-SegmentSeconds) > 0.05 {
				t.Fatalf("GPU HLS must segment on four-second IDRs: %s", playlist)
			}
			output, err = exec.Command("ffprobe", "-v", "error", "-show_streams", "-of", "json",
				filepath.Join(out, "index.m3u8")).CombinedOutput()
			if err != nil {
				t.Fatalf("probe GPU HLS metadata: %v %s", err, output)
			}
			var result struct {
				Streams []struct {
					Space     string `json:"color_space"`
					Transfer  string `json:"color_transfer"`
					Primaries string `json:"color_primaries"`
					SideData  []struct {
						Type string `json:"side_data_type"`
					} `json:"side_data_list"`
				} `json:"streams"`
			}
			if err := json.Unmarshal(output, &result); err != nil {
				t.Fatal(err)
			}
			if len(result.Streams) == 0 || result.Streams[0].Space != "bt709" ||
				result.Streams[0].Transfer != "bt709" || result.Streams[0].Primaries != "bt709" {
				t.Fatalf("GPU output must be tagged SDR BT.709: %s", output)
			}
			for _, side := range result.Streams[0].SideData {
				if strings.Contains(side.Type, "Mastering display") || strings.Contains(side.Type, "Content light") ||
					strings.Contains(side.Type, "HDR") || strings.Contains(side.Type, "DOVI") {
					t.Fatalf("GPU SDR output must not retain HDR metadata: %s", output)
				}
			}
		})
	}
}
