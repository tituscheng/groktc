package transcribe

import (
	"context"
	"errors"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckInstalledNotFound(t *testing.T) {
	c := &ExecFFmpegConverter{
		LookPath: func(string) (string, error) {
			return "", exec.ErrNotFound
		},
	}
	err := c.CheckInstalled()
	require.ErrorIs(t, err, ErrFFmpegNotFound)
}

func TestCheckInstalledFound(t *testing.T) {
	c := &ExecFFmpegConverter{
		LookPath: func(string) (string, error) {
			return "/usr/bin/ffmpeg", nil
		},
	}
	require.NoError(t, c.CheckInstalled())
}

func TestConvertNotInstalled(t *testing.T) {
	c := &ExecFFmpegConverter{
		LookPath: func(string) (string, error) {
			return "", exec.ErrNotFound
		},
	}
	err := c.Convert(context.Background(), "input.mp4", "output.mp3")
	require.ErrorIs(t, err, ErrFFmpegNotFound)
}

func TestConvertContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c := &ExecFFmpegConverter{
		LookPath: func(string) (string, error) {
			return "/usr/bin/ffmpeg", nil
		},
		CommandContext: exec.CommandContext,
	}

	// With a canceled context, CommandContext itself should fail.
	err := c.Convert(ctx, "input.mp4", "output.mp3")
	require.Error(t, err)
}

func TestConvertBuildsCorrectArgs(t *testing.T) {
	var capturedArgs []string
	c := &ExecFFmpegConverter{
		LookPath: func(string) (string, error) {
			return "/usr/bin/ffmpeg", nil
		},
		CommandContext: func(ctx context.Context, name string, args ...string) *exec.Cmd {
			capturedArgs = append([]string{name}, args...)
			// Return a command that fails immediately so we don't actually run ffmpeg.
			return exec.CommandContext(ctx, "false")
		},
	}

	_ = c.Convert(context.Background(), "video.mp4", "audio.mp3")

	require.Equal(t, []string{"ffmpeg", "-y", "-i", "video.mp4", "-vn", "-c:a", "libmp3lame", "-q:a", "4", "audio.mp3"}, capturedArgs)
}

func TestErrFFmpegNotFoundMessage(t *testing.T) {
	require.True(t, errors.Is(ErrFFmpegNotFound, ErrFFmpegNotFound))
	require.Contains(t, ErrFFmpegNotFound.Error(), "brew install ffmpeg")
	require.Contains(t, ErrFFmpegNotFound.Error(), "apt-get install ffmpeg")
}
