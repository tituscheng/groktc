package transcribe

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"time"

	"ytgo/pkg/ytgo/api"
)

// URLDownloader abstracts the ability to check for a downloader, extract
// video metadata, and download audio from a URL.
type URLDownloader interface {
	CheckInstalled() error
	GetTitle(ctx context.Context, url string) (string, error)
	GetTitleAndID(ctx context.Context, url string) (title, id string, err error)
	DownloadAudio(ctx context.Context, url, outputPath string) error
}

// ytgoDownloader uses the ytgo library to extract metadata and download
// audio from YouTube URLs. It implements the URLDownloader interface.
type ytgoDownloader struct {
	ffmpegPath string
}

func NewYtgoDownloader() *ytgoDownloader {
	return &ytgoDownloader{}
}

// CheckInstalled verifies that ffmpeg is available on PATH. ytgo itself is
// a pure Go library, but audio extraction (ExtractAudio → MP3) requires
// ffmpeg, so we check for it here.
func (d *ytgoDownloader) CheckInstalled() error {
	if d.ffmpegPath != "" {
		return nil
	}
	p, err := exec.LookPath("ffmpeg")
	if err != nil {
		slog.Error("ffmpeg not found", "error", err)
		return ErrFFmpegNotFound
	}
	d.ffmpegPath = p
	return nil
}

func (d *ytgoDownloader) GetTitle(ctx context.Context, url string) (string, error) {
	info, err := api.ExtractOnly(ctx, url, 30*time.Second)
	if err != nil {
		slog.Error("ytgo get title failed", "url", url, "error", err)
		return "", fmt.Errorf("ytgo get title: %w", err)
	}
	if info.Title == "" {
		slog.Error("ytgo returned empty title", "url", url)
		return "", fmt.Errorf("ytgo returned empty title for %q", url)
	}
	return info.Title, nil
}

func (d *ytgoDownloader) GetTitleAndID(ctx context.Context, url string) (string, string, error) {
	info, err := api.ExtractOnly(ctx, url, 30*time.Second)
	if err != nil {
		slog.Error("ytgo get title+id failed", "url", url, "error", err)
		return "", "", fmt.Errorf("ytgo get title+id: %w", err)
	}
	if info.Title == "" {
		slog.Error("ytgo returned empty title", "url", url)
		return "", "", fmt.Errorf("ytgo returned empty title for %q", url)
	}
	return info.Title, info.ID, nil
}

func (d *ytgoDownloader) DownloadAudio(ctx context.Context, url, outputPath string) error {
	if err := d.CheckInstalled(); err != nil {
		return err
	}

	opts := api.DefaultOptions()
	opts.ExtractAudio = true
	opts.AudioFormat = "mp3"
	opts.AudioQuality = "4"
	opts.OutputTemplate = outputPath
	opts.FFmpegLocation = d.ffmpegPath
	opts.ContinuePartial = false

	if err := api.Download(ctx, url, opts); err != nil {
		slog.Error("ytgo download audio failed",
			"url", url,
			"output_path", outputPath,
			"error", err,
		)
		return fmt.Errorf("ytgo download audio: %w", err)
	}
	return nil
}
