// Package playback turns a media file into an HLS stream, copying what the client can play and
// transcoding the rest, on the GPU when one is available.
package playback

import (
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"toto/api/internal/scanner"
)

const (
	SegmentSeconds  = 4
	maxOutputHeight = 1080
	// copySeekNudge is added to a keyframe start when a stream is copied. With B-frames, ffmpeg seeks 3/23 s
	// before -ss; asked for the keyframe itself, it lands on the previous one, so the copied stream starts
	// seconds earlier than the session while decoded streams start on time.
	copySeekNudge = 0.2
)

// Accel is the hardware acceleration used for transcoding.
type Accel string

const (
	AccelQSV   Accel = "qsv"
	AccelVAAPI Accel = "vaapi"
	AccelNone  Accel = "none"
)

// ClientCaps lists what the client can decode, as reported by the browser (MediaSource) or the mobile app.
type ClientCaps struct {
	VideoCodecs         []string `json:"videoCodecs"`
	AudioCodecs         []string `json:"audioCodecs"`
	HDR                 bool     `json:"hdr"`
	MaxHeight           int      `json:"maxHeight"`
	PreferredAudioCodec string   `json:"preferredAudioCodec,omitempty"`
}

// Video codecs that fit in fMP4 HLS segments and can therefore be copied.
var copyableVideo = []string{"h264", "hevc", "av1"}

// Audio codecs that fit in fMP4; the client must also support them.
var copyableAudio = []string{"aac", "mp3", "opus", "flac", "ac3", "eac3", "alac"}

// GPUs decode these codecs; others (MPEG-4 Part 2, VC-1...) are decoded on the CPU and uploaded.
var hwDecodable = []string{"h264", "hevc", "av1", "vp9", "mpeg2video"}

type Source struct {
	Path     string
	Duration float64
	Video    *scanner.Stream
	Audio    *scanner.Stream
	// Subtitle is set only for image subtitles, which must be burned into the picture.
	Subtitle *scanner.Stream
	// TextSubtitle is converted to WebVTT next to the segments while the stream is produced.
	TextSubtitle *scanner.Stream
}

type Plan struct {
	CopyVideo     bool `json:"copyVideo"`
	CopyAudio     bool `json:"copyAudio"`
	Tonemap       bool `json:"tonemap"`
	BurnSubtitle  bool `json:"burnSubtitle"`
	Width         int  `json:"width,omitempty"`
	Height        int  `json:"height,omitempty"`
	AudioChannels int  `json:"audioChannels,omitempty"`
	// AudioCodec is only set when transcoding to something other than AAC.
	AudioCodec string `json:"audioCodec,omitempty"`
}

func IsHDR(v *scanner.Stream) bool {
	return v != nil && (v.ColorTransfer == "smpte2084" || v.ColorTransfer == "arib-std-b67")
}

func is10Bit(v *scanner.Stream) bool {
	return strings.Contains(v.PixFmt, "10") || strings.Contains(v.PixFmt, "12")
}

// Decide copies streams the client can play and transcodes the others.
func Decide(src Source, caps ClientCaps) Plan {
	var p Plan
	if v := src.Video; v != nil {
		p.CopyVideo = src.Subtitle == nil &&
			slices.Contains(copyableVideo, v.Codec) && slices.Contains(caps.VideoCodecs, v.Codec) &&
			!(v.Codec == "h264" && is10Bit(v)) &&
			(!IsHDR(v) || caps.HDR) &&
			(caps.MaxHeight == 0 || v.Height <= caps.MaxHeight)
		if !p.CopyVideo {
			p.Tonemap = IsHDR(v)
			p.BurnSubtitle = src.Subtitle != nil
			p.Width, p.Height = outputSize(v.Width, v.Height, caps.MaxHeight)
		}
	}
	if a := src.Audio; a != nil {
		preferSurround := a.Channels > 2 && slices.Contains([]string{"ac3", "eac3"}, caps.PreferredAudioCodec) && slices.Contains(caps.AudioCodecs, caps.PreferredAudioCodec)
		p.CopyAudio = slices.Contains(copyableAudio, a.Codec) && slices.Contains(caps.AudioCodecs, a.Codec) &&
			(!preferSurround || a.Codec == "ac3" || a.Codec == "eac3")
		if !p.CopyAudio {
			p.AudioChannels = min(max(a.Channels, 2), 6)
			if preferSurround {
				p.AudioCodec = caps.PreferredAudioCodec
			} else if !slices.Contains(caps.AudioCodecs, "aac") && slices.Contains(caps.AudioCodecs, "flac") {
				p.AudioCodec = "flac"
			}
		}
	}
	return p
}

// outputSize caps the height at 1080 (or the client limit) and keeps the aspect ratio with even dimensions.
func outputSize(w, h, clientMax int) (int, int) {
	limit := maxOutputHeight
	if clientMax > 0 && clientMax < limit {
		limit = clientMax
	}
	if w <= 0 || h <= 0 {
		return 1920 * limit / 1080 / 2 * 2, limit
	}
	if h <= limit {
		return w / 2 * 2, h / 2 * 2
	}
	return (w*limit/h + 1) / 2 * 2, limit
}

type Encoder struct {
	Accel         Accel
	Device        string
	HEVCVAAPI     bool
	VAAPITonemap  bool
	VulkanTonemap bool
}

func gpuHDR(p Plan, src Source, enc Encoder) bool {
	return p.Tonemap && !p.BurnSubtitle && src.Video != nil && src.Video.Codec == "hevc" &&
		enc.HEVCVAAPI && ((enc.VAAPITonemap && src.Video.HasMasteringDisplay) || enc.VulkanTonemap) &&
		(enc.Accel == AccelQSV || enc.Accel == AccelVAAPI)
}

func (enc Encoder) VideoAccel(p Plan, src Source) Accel {
	if gpuHDR(p, src, enc) {
		return AccelVAAPI
	}
	return enc.Accel
}

// Args builds the ffmpeg command line writing an fMP4 HLS stream into outDir, starting at start seconds.
func Args(p Plan, src Source, start float64, enc Encoder, outDir string) []string {
	args := []string{"-hide_banner", "-loglevel", "error", "-nostdin", "-y"}

	transcode := src.Video != nil && !p.CopyVideo
	hdrGPU := transcode && gpuHDR(p, src, enc)
	vaapiHDR := hdrGPU && enc.VAAPITonemap && src.Video.HasMasteringDisplay
	hwFrames := false
	if transcode {
		hwDecode := slices.Contains(hwDecodable, src.Video.Codec) && !p.BurnSubtitle
		switch {
		case hdrGPU:
			args = append(args, "-init_hw_device", "vaapi=va:"+enc.Device, "-filter_hw_device", "va")
			if !vaapiHDR {
				args = append(args, "-init_hw_device", "vulkan=vk@va")
			}
			args = append(args,
				"-hwaccel", "vaapi", "-hwaccel_device", "va", "-hwaccel_output_format", "vaapi")
			hwFrames = true
		case enc.Accel == AccelQSV:
			args = append(args, "-init_hw_device", "vaapi=va:"+enc.Device, "-init_hw_device", "qsv=qs@va", "-filter_hw_device", "qs")
			// Never use the QSV HEVC decoder: it can corrupt pixels and timestamps on Intel drivers.
			if hwDecode && src.Video.Codec != "hevc" && !p.Tonemap {
				args = append(args, "-hwaccel", "qsv", "-hwaccel_output_format", "qsv")
				hwFrames = true
			}
		case enc.Accel == AccelVAAPI:
			args = append(args, "-init_hw_device", "vaapi=va:"+enc.Device, "-filter_hw_device", "va")
			// VAAPI tone mapping needs mastering metadata; other HDR routes use the validated GPU path above.
			if hwDecode && !p.Tonemap {
				args = append(args, "-hwaccel", "vaapi", "-hwaccel_device", "va", "-hwaccel_output_format", "vaapi")
				hwFrames = true
			}
		}
	}

	nudge := 0.0
	if start > 0 {
		if p.CopyVideo || p.CopyAudio {
			nudge = copySeekNudge
		}
		args = append(args, "-ss", formatSeconds(start+nudge))
	}
	args = append(args, "-i", src.Path)

	if src.Video != nil {
		input := fmt.Sprintf("0:%d", src.Video.Index)
		switch {
		case p.CopyVideo:
			args = append(args, "-map", input, "-c:v", "copy")
			if src.Video.Codec == "hevc" {
				// Safari only plays HEVC in fMP4 when tagged hvc1.
				args = append(args, "-tag:v", "hvc1")
			}
		case p.BurnSubtitle:
			graph := fmt.Sprintf("[%s][0:%d]overlay=eof_action=pass,%s[v]", input, src.Subtitle.Index, videoFilter(p, src.Video, enc.Accel, false))
			args = append(args, "-filter_complex", graph, "-map", "[v]")
		case hdrGPU:
			args = append(args, "-map", input, "-vf", gpuHDRFilter(p, enc, vaapiHDR))
		default:
			filter := videoFilter(p, src.Video, enc.Accel, hwFrames)
			if hwFrames && enc.Accel == AccelQSV && src.Video.Codec == "hevc" {
				filter = "hwmap=derive_device=qsv,format=qsv," + filter
			}
			args = append(args, "-map", input, "-vf", filter)
		}
		if transcode {
			videoEncoder := enc.Accel
			if hdrGPU {
				videoEncoder = AccelVAAPI
			}
			args = append(args, encoderArgs(videoEncoder)...)
			if hdrGPU {
				// The output is SDR; remove source HDR SEI after encoding rather than inserting a CPU
				// metadata filter between Vulkan and VAAPI frames.
				args = append(args, "-bsf:v", "filter_units=remove_types=6")
			}
			args = append(args, "-force_key_frames", fmt.Sprintf("expr:gte(t,n_forced*%d)", SegmentSeconds))
			if p.Tonemap {
				args = append(args, "-color_primaries", "bt709", "-color_trc", "bt709")
				if vaapiHDR {
					args = append(args, "-color_range", "tv")
				} else {
					args = append(args, "-colorspace", "bt709")
					args = append(args, "-color_range", "tv")
				}
			}
		}
	}

	if src.Audio != nil {
		args = append(args, "-map", fmt.Sprintf("0:%d", src.Audio.Index))
		if p.CopyAudio {
			args = append(args, "-c:a", "copy")
		} else if p.AudioCodec == "ac3" || p.AudioCodec == "eac3" {
			args = append(args, "-c:a", p.AudioCodec, "-b:a", "640k", "-ac", strconv.Itoa(p.AudioChannels))
		} else if p.AudioCodec == "flac" {
			args = append(args, "-c:a", "flac", "-ac", strconv.Itoa(p.AudioChannels))
		} else {
			bitrate := "192k"
			if p.AudioChannels > 2 {
				bitrate = "384k"
			}
			args = append(args, "-c:a", "aac", "-b:a", bitrate, "-ac", strconv.Itoa(p.AudioChannels))
		}
	}

	args = append(args,
		"-sn", "-dn", "-map_chapters", "-1",
		"-max_muxing_queue_size", "4096",
		"-f", "hls",
		"-hls_time", strconv.Itoa(SegmentSeconds),
		"-hls_list_size", "0",
		"-hls_playlist_type", "event",
		"-hls_segment_type", "fmp4",
		"-hls_fmp4_init_filename", "init.mp4",
		"-hls_flags", "independent_segments+temp_file",
		// Without edit lists, each track's timestamps carry its real start. Otherwise ffmpeg rebases every
		// track to zero and relies on the edit list to restore the gap between them (the B-frame delay of the
		// video, plus any audio delay of the source). MSE and ExoPlayer ignore that edit list in fMP4, so the
		// sound played ahead of the picture.
		"-hls_segment_options", "use_editlist=0",
		"-hls_segment_filename", filepath.Join(outDir, "seg_%05d.m4s"),
		filepath.Join(outDir, "index.m3u8"),
	)
	// Timestamps restart at zero after -ss, so these cues already match the stream clock. The stream starts
	// on the keyframe, before the nudged -ss, so the cues are shifted by the nudge.
	if src.TextSubtitle != nil {
		args = append(args, "-map", fmt.Sprintf("0:%d", src.TextSubtitle.Index), "-c:s", "webvtt", "-flush_packets", "1")
		if nudge > 0 {
			args = append(args, "-output_ts_offset", formatSeconds(nudge))
		}
		args = append(args, "-f", "webvtt", filepath.Join(outDir, LiveSubtitleFile))
	}
	return args
}

const LiveSubtitleFile = "subs.vtt"

// Software tone mapping: linearize, map with Hable, convert back to BT.709.
const hdrMetadataCleanup = "sidedata=mode=delete:type=MASTERING_DISPLAY_METADATA,sidedata=mode=delete:type=CONTENT_LIGHT_LEVEL," +
	"sidedata=mode=delete:type=DYNAMIC_HDR_PLUS,sidedata=mode=delete:type=DYNAMIC_HDR_VIVID," +
	"sidedata=mode=delete:type=DOVI_RPU_BUFFER,sidedata=mode=delete:type=DOVI_METADATA"

const cpuTonemap = "zscale=t=linear:npl=100,format=gbrpf32le,zscale=p=bt709,tonemap=hable:desat=0,zscale=t=bt709:m=bt709:r=tv," + hdrMetadataCleanup

func gpuHDRFilter(p Plan, enc Encoder, vaapiHDR bool) string {
	if vaapiHDR {
		return fmt.Sprintf("tonemap_vaapi=format=nv12:matrix=bt709:primaries=bt709:transfer=bt709,"+
			"scale_vaapi=w=%d:h=%d:format=nv12:out_color_matrix=bt709:out_color_primaries=bt709:"+
			"out_color_transfer=bt709:out_range=tv", p.Width, p.Height)
	}
	filter := fmt.Sprintf("hwmap=derive_device=vulkan,libplacebo=w=%d:h=%d:format=nv12:"+
		"colorspace=bt709:color_primaries=bt709:color_trc=bt709:range=tv:tonemapping=hable,"+
		"hwmap=derive_device=vaapi,format=vaapi", p.Width, p.Height)
	return filter
}

// cpuTonemapAt downscales to the output size before tone mapping. The tone mapping works on float RGB, so
// running it on a 4K picture instead of 1080p costs four times as much and cannot keep up with playback.
func cpuTonemapAt(p Plan) string {
	return fmt.Sprintf("scale=%d:%d,%s", p.Width, p.Height, cpuTonemap)
}

// videoFilter scales, tone maps when needed and converts to 8-bit 4:2:0 on the device of the encoder.
// hwFrames tells whether decoded frames already live on the GPU.
func videoFilter(p Plan, v *scanner.Stream, accel Accel, hwFrames bool) string {
	size := fmt.Sprintf("w=%d:h=%d", p.Width, p.Height)
	var chain []string
	switch accel {
	case AccelQSV:
		if !hwFrames {
			upload := "nv12"
			if p.Tonemap {
				chain = append(chain, cpuTonemapAt(p))
			} else if is10Bit(v) {
				upload = "p010"
			}
			chain = append(chain, "format="+upload, "hwupload=extra_hw_frames=64")
		}
		chain = append(chain, "vpp_qsv="+size+":format=nv12")
	case AccelVAAPI:
		if !hwFrames {
			if p.Tonemap {
				chain = append(chain, cpuTonemapAt(p))
			}
			chain = append(chain, "format=nv12", "hwupload")
		}
		chain = append(chain, "scale_vaapi="+size+":format=nv12")
	default:
		if p.Tonemap {
			chain = append(chain, cpuTonemapAt(p))
		} else {
			chain = append(chain, fmt.Sprintf("scale=%d:%d", p.Width, p.Height))
		}
		chain = append(chain, "format=yuv420p")
	}
	return strings.Join(chain, ",")
}

func encoderArgs(accel Accel) []string {
	switch accel {
	case AccelQSV:
		return []string{"-c:v", "h264_qsv", "-preset", "veryfast", "-global_quality", "24", "-profile:v", "high", "-forced_idr", "1"}
	case AccelVAAPI:
		return []string{"-c:v", "h264_vaapi", "-qp", "24", "-profile:v", "high"}
	default:
		return []string{"-c:v", "libx264", "-preset", "veryfast", "-crf", "23", "-maxrate", "12M", "-bufsize", "24M", "-profile:v", "high"}
	}
}

func formatSeconds(s float64) string {
	return strconv.FormatFloat(s, 'f', 3, 64)
}
