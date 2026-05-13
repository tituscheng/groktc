package transcribe

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
)

var ErrFFmpegNotFound = errors.New(
	"ffmpeg not found — install it:\n" +
		"  macOS:   brew install ffmpeg\n" +
		"  Ubuntu:  sudo apt-get install ffmpeg\n" +
		"  Windows: https://ffmpeg.org/download.html",
)

type FFmpegConverter interface {
	CheckInstalled() error
	Convert(ctx context.Context, inputPath, outputPath string) error
}

type ExecFFmpegConverter struct {
	LookPath      func(string) (string, error)
	CommandContext func(ctx context.Context, name string, arg ...string) *exec.Cmd
}

func NewExecFFmpegConverter() *ExecFFmpegConverter {
	return &ExecFFmpegConverter{
		LookPath:      exec.LookPath,
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
