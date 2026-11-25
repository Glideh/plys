package playback

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"toto/api/internal/scanner"
)

const probeEncodeTimeout = 20 * time.Second

// DetectEncoder tries a real encode on each render node, preferring QSV, then VAAPI, then the CPU.
// override is "auto", "qsv", "vaapi" or "none".
func DetectEncoder(ctx context.Context, ffmpeg, override string) Encoder {
	if override == string(AccelNone) {
		return Encoder{Accel: AccelNone}
	}
	devices, _ := filepath.Glob("/dev/dri/renderD*")
	candidates := []Accel{AccelQSV, AccelVAAPI}
	if override == string(AccelQSV) || override == string(AccelVAAPI) {
		candidates = []Accel{Accel(override)}
	}
	for _, accel := range candidates {
		for _, dev := range devices {
			enc := Encoder{Accel: accel, Device: dev}
			if err := testEncode(ctx, ffmpeg, enc); err != nil {
				slog.Debug("hardware encoder unavailable", "accel", accel, "device", dev, "err", err)
				continue
			}
			detectHDR(ctx, ffmpeg, &enc)
			slog.Info("hardware transcoding enabled", "accel", accel, "device", dev,
				"hevc_vaapi", enc.HEVCVAAPI, "vaapi_tonemap", enc.VAAPITonemap, "vulkan_tonemap", enc.VulkanTonemap)
			return enc
		}
	}
	slog.Info("no hardware encoder usable, transcoding on the CPU", "render_nodes", devices)
	return Encoder{Accel: AccelNone}
}

func testEncode(ctx context.Context, ffmpeg string, enc Encoder) error {
	ctx, cancel := context.WithTimeout(ctx, probeEncodeTimeout)
	defer cancel()
	args := []string{"-hide_banner", "-loglevel", "error", "-nostdin"}
	switch enc.Accel {
	case AccelQSV:
		args = append(args, "-init_hw_device", "vaapi=va:"+enc.Device, "-init_hw_device", "qsv=qs@va", "-filter_hw_device", "qs")
	case AccelVAAPI:
		args = append(args, "-init_hw_device", "vaapi=va:"+enc.Device, "-filter_hw_device", "va")
	}
	args = append(args, "-f", "lavfi", "-i", "testsrc2=size=640x360:rate=25", "-frames:v", "10",
		"-vf", "format=nv12,hwupload=extra_hw_frames=16")
	args = append(args, encoderArgs(enc.Accel)...)
	args = append(args, "-f", "null", "-")
	return runProbe(ctx, ffmpeg, args)
}

func runProbe(ctx context.Context, ffmpeg string, args []string) error {
	out, err := exec.CommandContext(ctx, ffmpeg, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Probe decoding separately from encoding: a working H.264 encoder says nothing about HEVC or HDR.
func detectHDR(ctx context.Context, ffmpeg string, enc *Encoder) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	dir, err := os.MkdirTemp("", "plys-hdr-probe-")
	if err != nil {
		slog.Warn("hardware HDR probe unavailable, using software HDR", "err", err)
		return
	}
	defer os.RemoveAll(dir)
	input := filepath.Join(dir, "hdr.mkv")
	err = runProbe(ctx, ffmpeg, []string{"-v", "error", "-nostdin", "-f", "lavfi", "-i", "testsrc2=s=640x360:r=24",
		"-frames:v", "48", "-pix_fmt", "yuv420p10le", "-c:v", "libx265", "-preset", "ultrafast",
		"-x265-params", "pools=2:log-level=error:colorprim=bt2020:transfer=smpte2084:colormatrix=bt2020nc:range=limited:master-display=G(13250,34500)B(7500,3000)R(34000,16000)WP(15635,16450)L(40000000,50):max-cll=1000,400",
		input})
	if err != nil {
		slog.Warn("hardware HDR test source unavailable, using software HDR", "err", err)
		return
	}
	reference, err := probeFrames(ctx, ffmpeg, input, 0.5, false, enc.Device)
	if err != nil {
		slog.Warn("hardware HDR reference unavailable, using software HDR", "err", err)
		return
	}
	decoded, err := probeFrames(ctx, ffmpeg, input, 0.5, true, enc.Device)
	if err == nil {
		err = compareDecode(reference, decoded)
	}
	if err != nil {
		slog.Warn("VAAPI HEVC decoding failed validation, using software HDR", "device", enc.Device, "err", err)
		return
	}
	enc.HEVCVAAPI = true
	source := Source{Path: input, Video: &scanner.Stream{
		Index: 0, Codec: "hevc", PixFmt: "yuv420p10le", Width: 640, Height: 360, ColorTransfer: "smpte2084",
		HasMasteringDisplay: true,
	}}
	if err := validateHDRRoute(ctx, ffmpeg, source, reference, *enc, dir, true); err == nil {
		enc.VAAPITonemap = true
	} else {
		slog.Warn("VAAPI tone mapping unavailable, trying Vulkan", "device", enc.Device, "err", err)
	}
	// Vulkan is derived from the render node, not selected by index: never pick a software GPU or another card.
	candidate := *enc
	candidate.VAAPITonemap = false
	candidate.VulkanTonemap = true
	if err := validateHDRRoute(ctx, ffmpeg, source, reference, candidate, dir, false); err != nil {
		message := "GPU HDR pipeline unavailable, using software tone mapping"
		if enc.VAAPITonemap {
			message = "Vulkan HDR fallback unavailable; HDR without mastering metadata will use software tone mapping"
		}
		slog.Warn(message, "device", enc.Device, "err", err)
		return
	}
	enc.VulkanTonemap = true
}

func validateHDRRoute(ctx context.Context, ffmpeg string, source Source, reference []probeFrame, enc Encoder, dir string, vaapiTonemap bool) error {
	if vaapiTonemap {
		enc.VAAPITonemap = true
	} else {
		enc.VulkanTonemap = true
	}
	route := "vulkan"
	if vaapiTonemap {
		route = "vaapi"
	}
	outDir := filepath.Join(dir, "hls-"+route)
	if err := os.Mkdir(outDir, 0o700); err != nil {
		return fmt.Errorf("create HDR probe output: %w", err)
	}
	args := Args(Plan{Tonemap: true, Width: source.Video.Width, Height: source.Video.Height}, source, 0.5, enc, outDir)
	if err := runProbe(ctx, ffmpeg, args); err != nil {
		return fmt.Errorf("encode HDR probe: %w", err)
	}
	frames, err := probeFrames(ctx, ffmpeg, filepath.Join(outDir, "index.m3u8"), 0, false, "")
	if err != nil {
		return fmt.Errorf("probe HDR output: %w", err)
	}
	if len(frames) != len(reference) {
		return fmt.Errorf("HDR output contains %d frames, want %d", len(frames), len(reference))
	}
	for i := 1; i < len(frames); i++ {
		if frames[i].pts != frames[i-1].pts+1 {
			return fmt.Errorf("HDR output timestamps are not continuous")
		}
	}
	return nil
}

type probeFrame struct {
	pts      int64
	checksum string
}

func probeFrames(ctx context.Context, ffmpeg, input string, start float64, hardware bool, device string) ([]probeFrame, error) {
	args := []string{"-v", "error", "-nostdin"}
	filter := "format=p010le"
	if hardware {
		args = append(args, "-init_hw_device", "vaapi=va:"+device, "-hwaccel", "vaapi",
			"-hwaccel_device", "va", "-hwaccel_output_format", "vaapi")
		filter = "hwdownload,format=p010le"
	}
	if start > 0 {
		args = append(args, "-ss", formatSeconds(start))
	}
	args = append(args, "-i", input, "-map", "0:v:0", "-vf", filter,
		"-fps_mode", "passthrough", "-f", "framemd5", "-")
	var stderr strings.Builder
	cmd := exec.CommandContext(ctx, ffmpeg, args...)
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, stderr.String())
	}
	return parseProbeFrames(string(out))
}

func parseProbeFrames(out string) ([]probeFrame, error) {
	var frames []probeFrame
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Split(line, ",")
		if len(fields) != 6 {
			return nil, fmt.Errorf("invalid frame probe: %s", line)
		}
		pts, err := strconv.ParseInt(strings.TrimSpace(fields[2]), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid frame timestamp: %w", err)
		}
		frames = append(frames, probeFrame{pts: pts, checksum: strings.TrimSpace(fields[5])})
	}
	if len(frames) == 0 {
		return nil, fmt.Errorf("frame probe returned no frames")
	}
	return frames, nil
}

func compareDecode(reference, decoded []probeFrame) error {
	if len(reference) != len(decoded) {
		return fmt.Errorf("HEVC decoder returned %d frames, want %d", len(decoded), len(reference))
	}
	for i := range reference {
		if reference[i] != decoded[i] {
			return fmt.Errorf("HEVC decoder pixels or timestamps differ at frame %d", i)
		}
	}
	return nil
}
