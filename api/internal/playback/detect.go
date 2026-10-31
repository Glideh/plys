package playback

import (
	"context"
	"log/slog"
	"os/exec"
	"path/filepath"
	"time"
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
			slog.Info("hardware transcoding enabled", "accel", accel, "device", dev)
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
	return exec.CommandContext(ctx, ffmpeg, args...).Run()
}
