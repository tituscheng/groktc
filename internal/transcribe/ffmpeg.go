package transcribe

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"
)

var ErrFFmpegNotFound = errors.New(
	"ffmpeg not found — install it:\n" +
		"  macOS:   brew install ffmpeg\n" +
		"  Ubuntu:  sudo apt-get install ffmpeg\n" +
		"  Windows: https://ffmpeg.org/download.html",
)

var ErrFFprobeNotFound = errors.New(
	"ffprobe not found — install it:\n" +
		"  macOS:   brew install ffmpeg\n" +
		"  Ubuntu:  sudo apt-get install ffmpeg\n" +
		"  Windows: https://ffmpeg.org/download.html",
)

type FFmpegConverter interface {
	CheckInstalled() error
	Convert(ctx context.Context, inputPath, outputPath string) error
}

type MediaProbe struct {
	DurationSeconds float64
	AudioBitrate    int64
}

type MediaProber interface {
	CheckInstalled() error
	ProbeMedia(ctx context.Context, path string) (MediaProbe, error)
}

type ExecFFmpegConverter struct {
	LookPath       func(string) (string, error)
	CommandContext func(ctx context.Context, name string, arg ...string) *exec.Cmd
}

type ExecMediaProber struct {
	LookPath       func(string) (string, error)
	CommandContext func(ctx context.Context, name string, arg ...string) *exec.Cmd
}

func NewExecFFmpegConverter() *ExecFFmpegConverter {
	return &ExecFFmpegConverter{
		LookPath:       exec.LookPath,
		CommandContext: exec.CommandContext,
	}
}

func NewExecMediaProber() *ExecMediaProber {
	return &ExecMediaProber{
		LookPath:       exec.LookPath,
		CommandContext: exec.CommandContext,
	}
}

func (c *ExecFFmpegConverter) CheckInstalled() error {
	_, err := c.LookPath("ffmpeg")
	if err != nil {
		slog.Error("ffmpeg not found", "error", err)
		return ErrFFmpegNotFound
	}
	return nil
}

func (c *ExecFFmpegConverter) Convert(ctx context.Context, inputPath, outputPath string) error {
	if err := c.CheckInstalled(); err != nil {
		return err
	}
	cmd := c.CommandContext(ctx, "ffmpeg",
		"-y",
		"-i", inputPath,
		"-vn",
		"-c:a", "libmp3lame",
		"-q:a", "4",
		outputPath,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		slog.Error("ffmpeg conversion failed",
			"input_path", inputPath,
			"output_path", outputPath,
			"error", err,
			"output", string(out),
		)
		return fmt.Errorf("ffmpeg: %w\n%s", err, out)
	}
	return nil
}

func (p *ExecMediaProber) CheckInstalled() error {
	_, err := p.LookPath("ffprobe")
	if err != nil {
		slog.Error("ffprobe not found", "error", err)
		return ErrFFprobeNotFound
	}
	return nil
}

func (p *ExecMediaProber) ProbeMedia(ctx context.Context, path string) (MediaProbe, error) {
	if err := p.CheckInstalled(); err != nil {
		return MediaProbe{}, err
	}

	cmd := p.CommandContext(ctx, "ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-show_entries", "stream=bit_rate",
		"-select_streams", "a:0",
		"-of", "default=noprint_wrappers=1",
		path,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		slog.Error("ffprobe failed",
			"path", path,
			"error", err,
			"output", string(out),
		)
		return MediaProbe{}, fmt.Errorf("ffprobe %q: %w\n%s", path, err, out)
	}

	return parseFFprobeOutput(string(out))
}

func parseFFprobeOutput(output string) (MediaProbe, error) {
	var probe MediaProbe
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch key {
		case "duration":
			duration, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return MediaProbe{}, fmt.Errorf("parse ffprobe duration %q: %w", value, err)
			}
			probe.DurationSeconds = duration
		case "bit_rate":
			bitrate, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return MediaProbe{}, fmt.Errorf("parse ffprobe bit_rate %q: %w", value, err)
			}
			if bitrate > 0 {
				probe.AudioBitrate = bitrate
			}
		}
	}

	if probe.DurationSeconds <= 0 {
		return MediaProbe{}, fmt.Errorf("ffprobe returned no duration")
	}

	return probe, nil
}

