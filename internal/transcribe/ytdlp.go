package transcribe

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
)

var ErrYTDLPNotFound = errors.New(
	"yt-dlp not found — install it:\n" +
		"  macOS:   brew install yt-dlp\n" +
		"  Ubuntu:  sudo apt-get install yt-dlp\n" +
		"  Windows: https://github.com/yt-dlp/yt-dlp#release-files",
)

type URLDownloader interface {
	CheckInstalled() error
	GetTitle(ctx context.Context, url string) (string, error)
	GetTitleAndID(ctx context.Context, url string) (title, id string, err error)
	DownloadAudio(ctx context.Context, url, outputPath string) error
}

type ExecYTDLPDownloader struct {
	LookPath       func(string) (string, error)
	CommandContext func(ctx context.Context, name string, arg ...string) *exec.Cmd
}

func NewExecYTDLPDownloader() *ExecYTDLPDownloader {
	return &ExecYTDLPDownloader{
		LookPath:       exec.LookPath,
		CommandContext: exec.CommandContext,
	}
}

func (d *ExecYTDLPDownloader) CheckInstalled() error {
	_, err := d.LookPath("yt-dlp")
	if err != nil {
		slog.Error("yt-dlp not found", "error", err)
		return ErrYTDLPNotFound
	}
	return nil
}

func (d *ExecYTDLPDownloader) GetTitle(ctx context.Context, url string) (string, error) {
	if err := d.CheckInstalled(); err != nil {
		return "", err
	}
	cmd := d.CommandContext(ctx, "yt-dlp",
		"--print", "title",
		"--no-download",
		url,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		slog.Error("yt-dlp get title failed",
			"url", url,
			"error", err,
			"output", string(out),
		)
		return "", fmt.Errorf("yt-dlp get title: %w\n%s", err, out)
	}
	title := strings.TrimSpace(string(out))
	if title == "" {
		slog.Error("yt-dlp returned empty title", "url", url)
		return "", fmt.Errorf("yt-dlp returned empty title for %q", url)
	}
	return title, nil
}

func (d *ExecYTDLPDownloader) GetTitleAndID(ctx context.Context, url string) (string, string, error) {
	if err := d.CheckInstalled(); err != nil {
		return "", "", err
	}
	cmd := d.CommandContext(ctx, "yt-dlp",
		"--print", "title",
		"--print", "id",
		"--no-download",
		url,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		slog.Error("yt-dlp get title+id failed",
			"url", url,
			"error", err,
			"output", string(out),
		)
		return "", "", fmt.Errorf("yt-dlp get title+id: %w\n%s", err, out)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		slog.Error("yt-dlp returned unexpected output for title+id",
			"url", url,
			"output", string(out),
		)
		return "", "", fmt.Errorf("yt-dlp returned unexpected output for title+id for %q", url)
	}
	title := strings.TrimSpace(lines[0])
	id := strings.TrimSpace(lines[1])
	if title == "" {
		slog.Error("yt-dlp returned empty title", "url", url)
		return "", "", fmt.Errorf("yt-dlp returned empty title for %q", url)
	}
	return title, id, nil
}

func (d *ExecYTDLPDownloader) DownloadAudio(ctx context.Context, url, outputPath string) error {
	if err := d.CheckInstalled(); err != nil {
		return err
	}
	cmd := d.CommandContext(ctx, "yt-dlp",
		"-x",
		"--audio-format", "mp3",
		"--audio-quality", "4",
		"-o", outputPath,
		url,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		slog.Error("yt-dlp download audio failed",
			"url", url,
			"output_path", outputPath,
			"error", err,
			"output", string(out),
		)
		return fmt.Errorf("yt-dlp: %w\n%s", err, out)
	}
	return nil
}
