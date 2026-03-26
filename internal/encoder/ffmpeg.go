package encoder

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"h265conv/internal/i18n"
)

type EncoderMode int

const (
	ModeNVENC EncoderMode = iota
	ModeCPU
)

type FFmpeg struct {
	ffmpegPath  string
	ffprobePath string
	Mode        EncoderMode
	GPUName     string
}

func NewFFmpeg(binDir string) (*FFmpeg, error) {
	ffmpegPath := filepath.Join(binDir, "ffmpeg.exe")
	ffprobePath := filepath.Join(binDir, "ffprobe.exe")

	if _, err := os.Stat(ffmpegPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("ffmpeg.exe が見つかりません: %s", ffmpegPath)
	}
	if _, err := os.Stat(ffprobePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("ffprobe.exe が見つかりません: %s", ffprobePath)
	}

	f := &FFmpeg{
		ffmpegPath:  ffmpegPath,
		ffprobePath: ffprobePath,
		Mode:        ModeCPU,
	}

	f.detectNVENC()
	return f, nil
}

// detectNVENC tries a 1-frame NVENC encode to check GPU availability.
func (f *FFmpeg) detectNVENC() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*1000*1000*1000) // 10s
	defer cancel()

	cmd := exec.CommandContext(ctx, f.ffmpegPath,
		"-f", "lavfi", "-i", "nullsrc=s=256x256:d=0.04",
		"-c:v", "hevc_nvenc", "-f", "null", "-",
	)
	cmd.Stdout = nil
	cmd.Stderr = nil
	hideWindow(cmd)

	if err := cmd.Run(); err == nil {
		f.Mode = ModeNVENC
		f.GPUName = f.detectGPUName()
	}
}

func (f *FFmpeg) detectGPUName() string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*1000*1000*1000)
	defer cancel()

	cmd := exec.CommandContext(ctx, f.ffmpegPath,
		"-f", "lavfi", "-i", "nullsrc=s=256x256:d=0.04",
		"-c:v", "hevc_nvenc", "-f", "null", "-",
	)
	hideWindow(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "NVIDIA GPU"
	}
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.Contains(line, "GPU") || strings.Contains(line, "gpu") {
			// Try to extract GPU name from NVENC init output
			if idx := strings.Index(line, "GPU #"); idx >= 0 {
				part := strings.TrimSpace(line[idx:])
				if dashIdx := strings.Index(part, " - "); dashIdx >= 0 {
					name := strings.TrimSpace(part[dashIdx+3:])
					if closeParen := strings.Index(name, ")"); closeParen >= 0 {
						name = name[:closeParen]
					}
					if name != "" {
						return name
					}
				}
			}
		}
	}
	return "NVIDIA GPU"
}

func (f *FFmpeg) StatusText() string {
	if f.Mode == ModeNVENC {
		return fmt.Sprintf(i18n.T("enc.nvenc"), f.GPUName)
	}
	return i18n.T("enc.cpu")
}

type ProbeResult struct {
	VideoBitrateKbps int64
	DurationSec      float64
	FileSize         int64
}

// Probe retrieves video info using ffprobe.
func (f *FFmpeg) Probe(inputPath string) (*ProbeResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*1000*1000*1000)
	defer cancel()

	cmd := exec.CommandContext(ctx, f.ffprobePath,
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		inputPath,
	)
	hideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}

	var data struct {
		Format struct {
			Duration string `json:"duration"`
			Size     string `json:"size"`
			BitRate  string `json:"bit_rate"`
		} `json:"format"`
		Streams []struct {
			CodecType string `json:"codec_type"`
			BitRate   string `json:"bit_rate"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(out, &data); err != nil {
		return nil, fmt.Errorf("ffprobe JSON parse error: %w", err)
	}

	result := &ProbeResult{}

	if dur, err := strconv.ParseFloat(data.Format.Duration, 64); err == nil {
		result.DurationSec = dur
	}
	if sz, err := strconv.ParseInt(data.Format.Size, 10, 64); err == nil {
		result.FileSize = sz
	}

	// Try to get video stream bitrate
	for _, s := range data.Streams {
		if s.CodecType == "video" && s.BitRate != "" {
			if br, err := strconv.ParseInt(s.BitRate, 10, 64); err == nil {
				result.VideoBitrateKbps = br / 1000
				return result, nil
			}
		}
	}
	// Fallback: use container bitrate
	if data.Format.BitRate != "" {
		if br, err := strconv.ParseInt(data.Format.BitRate, 10, 64); err == nil {
			result.VideoBitrateKbps = br / 1000
		}
	}
	return result, nil
}

type EncodeOptions struct {
	InputPath   string
	OutputPath  string
	TargetKbps  int64
	LowPriority bool
}

// Encode runs ffmpeg and reports progress via the callback.
// The callback receives progress percentage (0-100).
// Returns error if encoding fails.
func (f *FFmpeg) Encode(ctx context.Context, opts EncodeOptions, onProgress func(float64), duration float64) error {
	// Use a temp file for progress output to avoid pipe issues with NVENC
	progressFile, err := os.CreateTemp("", "h265conv-progress-*.txt")
	if err != nil {
		return fmt.Errorf("create progress file: %w", err)
	}
	progressPath := progressFile.Name()
	progressFile.Close()
	defer os.Remove(progressPath)

	args := f.buildArgs(opts, progressPath)

	cmd := exec.CommandContext(ctx, f.ffmpegPath, args...)

	if opts.LowPriority {
		setLowPriority(cmd)
	} else {
		hideWindow(cmd)
	}

	// No pipes — stderr captured to buffer, stdout discarded
	var stderrBuf strings.Builder
	cmd.Stderr = &stderrBuf
	cmd.Stdout = nil

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("ffmpeg start: %w", err)
	}

	// Poll progress file in background
	stopProgress := make(chan struct{})
	go func() {
		defer func() { recover() }()
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stopProgress:
				return
			case <-ticker.C:
				pct := readProgressFile(progressPath, duration)
				if pct > 0 {
					onProgress(pct)
				}
			}
		}
	}()

	waitErr := cmd.Wait()
	close(stopProgress)

	if waitErr != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		errOut := stderrBuf.String()
		lines := strings.Split(strings.TrimSpace(errOut), "\n")
		if len(lines) > 5 {
			lines = lines[len(lines)-5:]
		}
		return fmt.Errorf("ffmpeg error: %w\n%s", waitErr, strings.Join(lines, "\n"))
	}
	return nil
}

func (f *FFmpeg) buildArgs(opts EncodeOptions, progressPath string) []string {
	target := opts.TargetKbps
	args := []string{
		"-i", opts.InputPath,
		"-progress", progressPath, "-nostats",
	}

	if f.Mode == ModeNVENC {
		maxrate := target * 3 / 2
		bufsize := target * 2
		args = append(args,
			"-c:v", "hevc_nvenc",
			"-preset", "p4",
			"-rc", "vbr",
			"-b:v", fmt.Sprintf("%dk", target),
			"-maxrate", fmt.Sprintf("%dk", maxrate),
			"-bufsize", fmt.Sprintf("%dk", bufsize),
			"-spatial_aq", "1",
		)
	} else {
		args = append(args,
			"-c:v", "libx265",
			"-preset", "medium",
			"-b:v", fmt.Sprintf("%dk", target),
			"-x265-params", "aq-mode=3",
		)
	}

	args = append(args,
		"-tag:v", "hvc1",
		"-c:a", "copy",
		"-movflags", "+faststart",
		"-y", opts.OutputPath,
	)
	return args
}

// readProgressFile reads the ffmpeg progress file and returns the current percentage.
func readProgressFile(path string, totalDuration float64) float64 {
	if totalDuration <= 0 {
		return 0
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	// Find the last out_time_ms= line
	var lastUS int64
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "out_time_ms=") {
			val := strings.TrimPrefix(line, "out_time_ms=")
			if us, err := strconv.ParseInt(val, 10, 64); err == nil {
				lastUS = us
			}
		}
	}
	if lastUS <= 0 {
		return 0
	}
	sec := float64(lastUS) / 1_000_000.0
	pct := sec / totalDuration * 100.0
	if pct > 100 {
		pct = 100
	}
	if pct < 0 {
		pct = 0
	}
	return pct
}

// CalculateTargetBitrate computes the target bitrate based on original and decay rate.
func CalculateTargetBitrate(originalKbps int64, decayRate int) int64 {
	target := originalKbps * int64(decayRate) / 100
	if target < 200 {
		target = 200
	}
	return target
}
