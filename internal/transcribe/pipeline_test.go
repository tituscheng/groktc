package transcribe

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// fakeSTTClient records calls and returns preset results.
type fakeSTTClient struct {
	mu     sync.Mutex
	calls  []string
	result STTResponse
	err    error
}

func (f *fakeSTTClient) Transcribe(_ context.Context, path string) (STTResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, path)
	return f.result, f.err
}

// fakeFFmpegConverter records Convert calls.
type fakeFFmpegConverter struct {
	mu           sync.Mutex
	calls        []string
	err          error
	installedErr error
}

func (f *fakeFFmpegConverter) CheckInstalled() error { return f.installedErr }

func (f *fakeFFmpegConverter) Convert(_ context.Context, inputPath, outputPath string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, inputPath)
	if f.err != nil {
		return f.err
	}
	// Write a tiny placeholder file so pipeline can proceed.
	return os.WriteFile(outputPath, []byte("fake mp3"), 0o644)
}

// fakeURLDownloader records DownloadAudio calls.
type fakeURLDownloader struct {
	mu            sync.Mutex
	downloadCalls []string
	title         string
	id            string
	titleErr      error
	err           error
	installedErr  error
}

func (f *fakeURLDownloader) CheckInstalled() error { return f.installedErr }

func (f *fakeURLDownloader) GetTitle(_ context.Context, url string) (string, error) {
	if f.titleErr != nil {
		return "", f.titleErr
	}
	return f.title, nil
}

func (f *fakeURLDownloader) GetTitleAndID(_ context.Context, url string) (string, string, error) {
	if f.titleErr != nil {
		return "", "", f.titleErr
	}
	return f.title, f.id, nil
}

func (f *fakeURLDownloader) DownloadAudio(_ context.Context, url, outputPath string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.downloadCalls = append(f.downloadCalls, url)
	if f.err != nil {
		return f.err
	}
	return os.WriteFile(outputPath, []byte("fake mp3"), 0o644)
}

func TestPipelineMP3CallsSTTAndWritesTxt(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "audio.mp3")
	output := filepath.Join(dir, "audio.txt")
	require.NoError(t, os.WriteFile(input, []byte("audio data"), 0o644))

	stt := &fakeSTTClient{result: STTResponse{Text: "hello world", Duration: 12.5}}
	conv := &fakeFFmpegConverter{}

	p := newFilePipeline(stt, conv, nil)
	duration, err := p.Run(context.Background(), FileTask{
		InputPath: input, OutputPath: output, Kind: KindMP3,
	})
	require.NoError(t, err)
	require.Equal(t, 12.5, duration)

	require.Len(t, stt.calls, 1)
	require.Equal(t, input, stt.calls[0])
	require.Len(t, conv.calls, 0)

	// Verify the transcript was written directly to output.
	content, readErr := os.ReadFile(output)
	require.NoError(t, readErr)
	require.Equal(t, "hello world", string(content))
}

func TestPipelineMP4CallsConverterThenSTT(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "video.mp4")
	output := filepath.Join(dir, "video.txt")
	require.NoError(t, os.WriteFile(input, []byte("video data"), 0o644))

	stt := &fakeSTTClient{result: STTResponse{Text: "spoken words", Duration: 30.0}}
	conv := &fakeFFmpegConverter{}

	p := newFilePipeline(stt, conv, nil)
	duration, err := p.Run(context.Background(), FileTask{
		InputPath: input, OutputPath: output, Kind: KindMP4,
	})
	require.NoError(t, err)
	require.Equal(t, 30.0, duration)

	require.Len(t, conv.calls, 1)
	require.Equal(t, input, conv.calls[0])
	require.Len(t, stt.calls, 1)

	content, readErr := os.ReadFile(output)
	require.NoError(t, readErr)
	require.Equal(t, "spoken words", string(content))
}

func TestPipelineMP4TempMp3DeletedOnSuccess(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "video.mp4")
	output := filepath.Join(dir, "video.txt")
	require.NoError(t, os.WriteFile(input, []byte("x"), 0o644))

	var tmpMp3Path string
	stt := &fakeSTTClient{result: STTResponse{Text: "text", Duration: 5.0}}
	conv := &fakeFFmpegConverter{}

	// Capture the temp mp3 path by intercepting the STT call.
	stt2 := &capturingSTT{inner: stt, capturedPath: &tmpMp3Path}
	p := newFilePipeline(stt2, conv, nil)
	_, err := p.Run(context.Background(), FileTask{
		InputPath: input, OutputPath: output, Kind: KindMP4,
	})
	require.NoError(t, err)

	if tmpMp3Path != "" {
		_, err := os.Stat(tmpMp3Path)
		require.True(t, os.IsNotExist(err), "temp mp3 should be deleted after success")
	}
}

func TestPipelineMP4TempMp3KeptOnFailure(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "video.mp4")
	output := filepath.Join(dir, "video.txt")
	require.NoError(t, os.WriteFile(input, []byte("x"), 0o644))

	var tmpMp3Path string
	stt := &fakeSTTClient{err: fmt.Errorf("STT failed")}
	conv := &fakeFFmpegConverter{}

	stt2 := &capturingSTT{inner: stt, capturedPath: &tmpMp3Path}
	p := newFilePipeline(stt2, conv, nil)
	_, err := p.Run(context.Background(), FileTask{
		InputPath: input, OutputPath: output, Kind: KindMP4,
	})
	require.Error(t, err)

	// Temp mp3 should still exist for debugging.
	if tmpMp3Path != "" {
		_, statErr := os.Stat(tmpMp3Path)
		require.NoError(t, statErr, "temp mp3 should be preserved on failure")
		os.Remove(tmpMp3Path) // cleanup after assertion
	}
}

// capturingSTT wraps a fakeSTTClient and records the audio path passed in.
type capturingSTT struct {
	inner        STTClient
	capturedPath *string
}

func (c *capturingSTT) Transcribe(ctx context.Context, path string) (STTResponse, error) {
	if c.capturedPath != nil {
		*c.capturedPath = path
	}
	return c.inner.Transcribe(ctx, path)
}

func TestPipelineMP3TooLarge(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "big.mp3")
	require.NoError(t, os.WriteFile(input, []byte("x"), 0o644))

	stt := &fakeSTTClient{result: STTResponse{Text: "ok", Duration: 1.0}}
	p := newFilePipeline(stt, &fakeFFmpegConverter{}, nil)
	_, err := p.Run(context.Background(), FileTask{
		InputPath: input, OutputPath: filepath.Join(dir, "big.txt"), Kind: KindMP3,
	})
	require.NoError(t, err, "normal-sized file should not trigger size error")
}

func TestPipelineSTTEmptyTranscript(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "audio.mp3")
	require.NoError(t, os.WriteFile(input, []byte("x"), 0o644))

	stt := &fakeSTTClient{result: STTResponse{Text: "   "}}
	p := newFilePipeline(stt, &fakeFFmpegConverter{}, nil)
	_, err := p.Run(context.Background(), FileTask{
		InputPath: input, OutputPath: filepath.Join(dir, "audio.txt"), Kind: KindMP3,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty transcript")
}

func TestPipelineConvertFailure(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "video.mp4")
	output := filepath.Join(dir, "video.txt")
	require.NoError(t, os.WriteFile(input, []byte("x"), 0o644))

	conv := &fakeFFmpegConverter{err: fmt.Errorf("ffmpeg error")}
	stt := &fakeSTTClient{}
	p := newFilePipeline(stt, conv, nil)
	_, err := p.Run(context.Background(), FileTask{
		InputPath: input, OutputPath: output, Kind: KindMP4,
	})
	require.Error(t, err)
	require.Len(t, stt.calls, 0)
}

func TestPipelineURLCallsDownloaderThenSTT(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "output.txt")

	stt := &fakeSTTClient{result: STTResponse{Text: "spoken words", Duration: 45.0}}
	conv := &fakeFFmpegConverter{}
	downloader := &fakeURLDownloader{}

	p := newFilePipeline(stt, conv, downloader)
	duration, err := p.Run(context.Background(), FileTask{
		InputPath: "https://example.com/video", OutputPath: output, Kind: KindURL,
	})
	require.NoError(t, err)
	require.Equal(t, 45.0, duration)

	require.Len(t, downloader.downloadCalls, 1)
	require.Equal(t, "https://example.com/video", downloader.downloadCalls[0])
	require.Len(t, stt.calls, 1)

	content, readErr := os.ReadFile(output)
	require.NoError(t, readErr)
	require.Equal(t, "spoken words", string(content))
}

func TestPipelineURLTempMp3DeletedOnSuccess(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "output.txt")

	var tmpMp3Path string
	stt := &fakeSTTClient{result: STTResponse{Text: "text", Duration: 10.0}}
	conv := &fakeFFmpegConverter{}
	downloader := &fakeURLDownloader{}

	stt2 := &capturingSTT{inner: stt, capturedPath: &tmpMp3Path}
	p := newFilePipeline(stt2, conv, downloader)
	_, err := p.Run(context.Background(), FileTask{
		InputPath: "https://example.com/video", OutputPath: output, Kind: KindURL,
	})
	require.NoError(t, err)

	if tmpMp3Path != "" {
		_, err := os.Stat(tmpMp3Path)
		require.True(t, os.IsNotExist(err), "temp mp3 should be deleted after success")
	}
}

func TestPipelineURLTempMp3KeptOnFailure(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "output.txt")

	var tmpMp3Path string
	stt := &fakeSTTClient{err: fmt.Errorf("STT failed")}
	conv := &fakeFFmpegConverter{}
	downloader := &fakeURLDownloader{}

	stt2 := &capturingSTT{inner: stt, capturedPath: &tmpMp3Path}
	p := newFilePipeline(stt2, conv, downloader)
	_, err := p.Run(context.Background(), FileTask{
		InputPath: "https://example.com/video", OutputPath: output, Kind: KindURL,
	})
	require.Error(t, err)

	if tmpMp3Path != "" {
		_, statErr := os.Stat(tmpMp3Path)
		require.NoError(t, statErr, "temp mp3 should be preserved on failure")
		os.Remove(tmpMp3Path)
	}
}

func TestPipelineURLDownloadFailure(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "output.txt")

	stt := &fakeSTTClient{}
	conv := &fakeFFmpegConverter{}
	downloader := &fakeURLDownloader{err: fmt.Errorf("download error")}

	p := newFilePipeline(stt, conv, downloader)
	_, err := p.Run(context.Background(), FileTask{
		InputPath: "https://example.com/video", OutputPath: output, Kind: KindURL,
	})
	require.Error(t, err)
	require.Len(t, stt.calls, 0)
}
