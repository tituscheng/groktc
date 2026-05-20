package transcribe

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/briandowns/spinner"
)

type Pipeline interface {
	Run(ctx context.Context, task FileTask) (duration float64, err error)
}

type filePipeline struct {
	stt        STTClient
	converter  FFmpegConverter
	downloader URLDownloader
	stdout     io.Writer
}

func newFilePipeline(stt STTClient, converter FFmpegConverter, downloader URLDownloader) *filePipeline {
	return &filePipeline{stt: stt, converter: converter, downloader: downloader}
}

// SetStdout sets the writer used for animated progress spinners. If unset,
// os.Stdout is used.
func (p *filePipeline) SetStdout(w io.Writer) {
	p.stdout = w
}

// withSpinner runs fn while animating spinner charset 14. The message is left
// on the terminal as a completion line after the spinner stops.
func (p *filePipeline) withSpinner(message string, fn func() error) error {
	out := p.stdout
	if out == nil {
		out = os.Stdout
	}
	indent := transcribeMuted("     ")
	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond, spinner.WithWriter(out))
	s.Prefix = indent + " "
	s.Suffix = " " + message
	s.FinalMSG = fmt.Sprintf("%s   %s\n", indent, message)
	s.Start()
	err := fn()
	s.Stop()
	return err
}

func (p *filePipeline) Run(ctx context.Context, task FileTask) (float64, error) {
	switch task.Kind {
	case KindMP3:
		return p.processAudio(ctx, task.InputPath, task.OutputPath)

	case KindMP4:
		return p.processVideo(ctx, task, task.OutputPath)

	case KindURL:
		return p.processURL(ctx, task, task.OutputPath)

	default:
		return 0, fmt.Errorf("unsupported input kind %d", task.Kind)
	}
}

func (p *filePipeline) processVideo(ctx context.Context, task FileTask, outputPath string) (float64, error) {
	// NOTE: the 500 MB STT limit applies to the MP3 sent to STT, not the
	// source MP4. We don't pre-check MP4 size — a multi-GB video can produce
	// an MP3 well under the limit, especially with mono/low-bitrate output.
	dir := filepath.Dir(task.InputPath)
	base := strings.TrimSuffix(filepath.Base(task.InputPath), filepath.Ext(task.InputPath))
	tmpMp3, err := os.CreateTemp(dir, "."+base+".*.tmp.mp3")
	if err != nil {
		slog.Error("create temp mp3 failed", "dir", dir, "base", base, "error", err)
		return 0, fmt.Errorf("create temp mp3: %w", err)
	}
	tmpMp3Path := tmpMp3.Name()
	tmpMp3.Close()

	srcInfo, statErr := os.Stat(task.InputPath)
	var msg string
	if statErr == nil {
		msg = fmt.Sprintf("%s extracting audio (ffmpeg) from %.1f MB video...",
			task.InputPath, float64(srcInfo.Size())/(1024*1024))
	} else {
		msg = fmt.Sprintf("%s extracting audio (ffmpeg)...", task.InputPath)
	}

	if err := p.withSpinner(msg, func() error {
		return p.converter.Convert(ctx, task.InputPath, tmpMp3Path)
	}); err != nil {
		slog.Error("ffmpeg video conversion failed",
			"input_path", task.InputPath,
			"tmp_mp3_path", tmpMp3Path,
			"error", err,
		)
		return 0, fmt.Errorf("convert %q to MP3: %w", task.InputPath, err)
	}

	duration, err := p.processAudio(ctx, tmpMp3Path, outputPath)
	if err != nil {
		return duration, err
	}

	// Delete temp mp3 only on full success; on failure it stays so the user
	// can rerun directly against the .mp3.
	os.Remove(tmpMp3Path)
	return duration, nil
}

func (p *filePipeline) processURL(ctx context.Context, task FileTask, outputPath string) (float64, error) {
	dir := filepath.Dir(outputPath)
	if dir == "" || dir == "." {
		dir = "."
	}
	base := strings.TrimSuffix(filepath.Base(outputPath), filepath.Ext(outputPath))
	// Use a temp path but do NOT pre-create the file — some downloaders see
	// an empty existing file and skip the download, then post-processing fails.
	tmpMp3, err := os.CreateTemp(dir, "."+base+".*.tmp.mp3")
	if err != nil {
		slog.Error("create temp mp3 failed", "dir", dir, "base", base, "error", err)
		return 0, fmt.Errorf("create temp mp3: %w", err)
	}
	tmpMp3Path := tmpMp3.Name()
	tmpMp3.Close()
	os.Remove(tmpMp3Path)

	msg := fmt.Sprintf("%s downloading audio...", task.InputPath)
	if err := p.withSpinner(msg, func() error {
		return p.downloader.DownloadAudio(ctx, task.InputPath, tmpMp3Path)
	}); err != nil {
		slog.Error("download audio from URL failed",
			"input_path", task.InputPath,
			"tmp_mp3_path", tmpMp3Path,
			"error", err,
		)
		return 0, fmt.Errorf("download audio from %q: %w", task.InputPath, err)
	}

	duration, err := p.processAudio(ctx, tmpMp3Path, outputPath)
	if err != nil {
		return duration, err
	}

	// Delete temp mp3 only on full success; on failure it stays so the user
	// can rerun directly against the .mp3.
	os.Remove(tmpMp3Path)
	return duration, nil
}

func (p *filePipeline) processAudio(ctx context.Context, audioPath, outputPath string) (float64, error) {
	info, err := os.Stat(audioPath)
	if err != nil {
		slog.Error("stat audio file failed", "audio_path", audioPath, "error", err)
		return 0, fmt.Errorf("stat %q: %w", audioPath, err)
	}
	sizeMB := float64(info.Size()) / (1024 * 1024)
	if info.Size() > MaxAudioBytes {
		slog.Error("audio file too large",
			"audio_path", audioPath,
			"size_mb", sizeMB,
			"limit_mb", MaxAudioBytes/(1024*1024),
		)
		return 0, fmt.Errorf("%w: %q is %.1f MB (limit 500 MB)", ErrFileTooLarge, audioPath, sizeMB)
	}

	var resp STTResponse
	if err := p.withSpinner(
		fmt.Sprintf("%s transcribing %.1f MB audio (xAI STT)...", audioPath, sizeMB),
		func() error {
			r, err := p.stt.Transcribe(ctx, audioPath)
			if err != nil {
				return err
			}
			resp = r
			return nil
		},
	); err != nil {
		slog.Error("transcribe audio failed", "audio_path", audioPath, "error", err)
		return 0, fmt.Errorf("transcribe %q: %w", audioPath, err)
	}
	if strings.TrimSpace(resp.Text) == "" {
		slog.Error("STT returned empty transcript", "audio_path", audioPath)
		return 0, fmt.Errorf("STT returned empty transcript for %q", audioPath)
	}

	if err := os.WriteFile(outputPath, []byte(resp.Text), 0o644); err != nil {
		slog.Error("write transcript failed", "output_path", outputPath, "error", err)
		return resp.Duration, fmt.Errorf("write transcript to %q: %w", outputPath, err)
	}

	return resp.Duration, nil
}

var ErrFileTooLarge = fmt.Errorf("audio file exceeds 500 MB limit")
