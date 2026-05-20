package transcribe

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/tituscheng/groktc/pkg/runner"
	"github.com/tituscheng/groktc/internal/style"

	"github.com/stretchr/testify/require"
)

// fakePipeline records Run calls and optionally fails for specific input paths.
type fakePipeline struct {
	mu        sync.Mutex
	failFor   map[string]error
	calls     []string
	active    int
	maxActive int
	sleepFor  time.Duration
	durations map[string]float64
}

func (f *fakePipeline) Run(_ context.Context, task FileTask) (float64, error) {
	f.mu.Lock()
	f.active++
	if f.active > f.maxActive {
		f.maxActive = f.active
	}
	f.mu.Unlock()

	if f.sleepFor > 0 {
		time.Sleep(f.sleepFor)
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	f.active--
	f.calls = append(f.calls, task.InputPath)
	return f.durations[task.InputPath], f.failFor[task.InputPath]
}

// fakeInstalledConverter always reports ffmpeg as installed.
type fakeInstalledConverter struct{}

func (f *fakeInstalledConverter) CheckInstalled() error { return nil }
func (f *fakeInstalledConverter) Convert(_ context.Context, _, _ string) error {
	return nil
}

// fakeMissingConverter reports ffmpeg as not installed.
type fakeMissingConverter struct{}

func (f *fakeMissingConverter) CheckInstalled() error { return ErrFFmpegNotFound }
func (f *fakeMissingConverter) Convert(_ context.Context, _, _ string) error {
	return ErrFFmpegNotFound
}

func newTestRunner() *Runner {
	r := NewRunner()
	r.Stdout = io.Discard
	r.Stderr = io.Discard
	r.ConverterFactory = func() FFmpegConverter { return &fakeInstalledConverter{} }
	r.URLDownloaderFactory = func() URLDownloader { return &fakeInstalledDownloader{title: "Test Title", id: "abc123"} }
	return r
}

func TestRunnerMissingAPIKey(t *testing.T) {
	r := newTestRunner()
	r.LookupEnv = func(string) string { return "" }
	_, err := r.Run(context.Background(), Options{
		Args: []string{"file.mp3"},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), XAIAPIKeyEnv)
}

func TestRunnerProcessTasksContinuesAfterFailure(t *testing.T) {
	pipeline := &fakePipeline{
		failFor: map[string]error{"bad.mp3": fmt.Errorf("transcribe failed")},
	}
	var stderr bytes.Buffer
	r := newTestRunner()
	r.Stdout = io.Discard
	r.Stderr = &stderr

	summary := runner.ProcessTasks(context.Background(), []FileTask{
		{InputPath: "ok.mp3", OutputPath: "ok.txt", Kind: KindMP3},
		{InputPath: "bad.mp3", OutputPath: "bad.txt", Kind: KindMP3},
	}, func(t FileTask) runner.Task { return runner.Task{InputPath: t.InputPath, OutputPath: t.OutputPath, Skipped: t.Skipped, SkipReason: t.SkipReason} }, func(ctx context.Context, task FileTask) error {
		_, err := pipeline.Run(ctx, task)
		return err
	}, runner.Options{
		Concurrency: DefaultConcurrency,
		Stdout:      r.Stdout,
		Stderr:      r.Stderr,
		Style:       runner.Style{Header: style.Header, Label: style.Label, OK: style.OK, Skip: style.Skip, Fail: style.Fail, Muted: style.Muted, Value: style.Value, Total: style.Total},
		PanicLabel:  "transcribe",
		TaskLabel:   "transcribe",
	})

	require.Len(t, summary.Processed, 1)
	require.Len(t, summary.Failed, 1)
	require.Contains(t, stderr.String(), "Failed bad.mp3")
}

func TestRunnerProcessTasksLimitsConcurrency(t *testing.T) {
	pipeline := &fakePipeline{sleepFor: 25 * time.Millisecond}
	r := newTestRunner()

	tasks := make([]FileTask, 12)
	for i := range tasks {
		tasks[i] = FileTask{
			InputPath:  fmt.Sprintf("%d.mp3", i),
			OutputPath: fmt.Sprintf("%d.txt", i),
			Kind:       KindMP3,
		}
	}

	summary := runner.ProcessTasks(context.Background(), tasks, func(t FileTask) runner.Task { return runner.Task{InputPath: t.InputPath, OutputPath: t.OutputPath, Skipped: t.Skipped, SkipReason: t.SkipReason} }, func(ctx context.Context, task FileTask) error {
		_, err := pipeline.Run(ctx, task)
		return err
	}, runner.Options{
		Concurrency: DefaultConcurrency,
		Stdout:      r.Stdout,
		Stderr:      r.Stderr,
		Style:       runner.Style{Header: style.Header, Label: style.Label, OK: style.OK, Skip: style.Skip, Fail: style.Fail, Muted: style.Muted, Value: style.Value, Total: style.Total},
		PanicLabel:  "transcribe",
		TaskLabel:   "transcribe",
	})
	require.Len(t, summary.Processed, 12)
	require.LessOrEqual(t, pipeline.maxActive, DefaultConcurrency)
}

func TestRunnerProcessTasksSkipsSkippedTasks(t *testing.T) {
	pipeline := &fakePipeline{}
	r := newTestRunner()

	summary := runner.ProcessTasks(context.Background(), []FileTask{
		{InputPath: "skip.mp3", OutputPath: "skip.txt", Skipped: true, Kind: KindMP3},
		{InputPath: "run.mp3", OutputPath: "run.txt", Kind: KindMP3},
	}, func(t FileTask) runner.Task { return runner.Task{InputPath: t.InputPath, OutputPath: t.OutputPath, Skipped: t.Skipped, SkipReason: t.SkipReason} }, func(ctx context.Context, task FileTask) error {
		_, err := pipeline.Run(ctx, task)
		return err
	}, runner.Options{
		Concurrency: DefaultConcurrency,
		Stdout:      r.Stdout,
		Stderr:      r.Stderr,
		Style:       runner.Style{Header: style.Header, Label: style.Label, OK: style.OK, Skip: style.Skip, Fail: style.Fail, Muted: style.Muted, Value: style.Value, Total: style.Total},
		PanicLabel:  "transcribe",
		TaskLabel:   "transcribe",
	})

	require.Len(t, summary.Processed, 1)
	require.Len(t, summary.Skipped, 1)
	require.Len(t, pipeline.calls, 1)
	require.Equal(t, "run.mp3", pipeline.calls[0])
}

func TestRunnerMP4TaskChecksFFmpeg(t *testing.T) {
	dir := t.TempDir()
	mp4 := filepath.Join(dir, "video.mp4")
	require.NoError(t, os.WriteFile(mp4, []byte("x"), 0o644))

	r := newTestRunner()
	r.ConverterFactory = func() FFmpegConverter { return &fakeMissingConverter{} }
	r.LookupEnv = func(string) string { return "key" }

	_, err := r.Run(context.Background(), Options{
		Args: []string{mp4},
	})
	require.ErrorIs(t, err, ErrFFmpegNotFound)
}

func TestRunnerNoFilesToProcess(t *testing.T) {
	dir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	defer os.Chdir(origDir)

	r := newTestRunner()
	r.LookupEnv = func(string) string { return "key" }

	_, err = r.Run(context.Background(), Options{})
	require.Error(t, err)
}

// fakeInstalledDownloader always reports the downloader as installed.
type fakeInstalledDownloader struct {
	title    string
	id       string
	titleErr error
	err      error
}

func (f *fakeInstalledDownloader) CheckInstalled() error { return nil }
func (f *fakeInstalledDownloader) GetTitle(_ context.Context, _ string) (string, error) {
	if f.titleErr != nil {
		return "", f.titleErr
	}
	return f.title, nil
}
func (f *fakeInstalledDownloader) GetTitleAndID(_ context.Context, _ string) (string, string, error) {
	if f.titleErr != nil {
		return "", "", f.titleErr
	}
	return f.title, f.id, nil
}
func (f *fakeInstalledDownloader) DownloadAudio(_ context.Context, _, outputPath string) error {
	if f.err != nil {
		return f.err
	}
	return os.WriteFile(outputPath, []byte("fake mp3"), 0o644)
}

// fakeMissingDownloader reports the downloader as not installed.
type fakeMissingDownloader struct{}

func (f *fakeMissingDownloader) CheckInstalled() error { return ErrFFmpegNotFound }
func (f *fakeMissingDownloader) GetTitle(_ context.Context, _ string) (string, error) {
	return "", ErrFFmpegNotFound
}
func (f *fakeMissingDownloader) GetTitleAndID(_ context.Context, _ string) (string, string, error) {
	return "", "", ErrFFmpegNotFound
}
func (f *fakeMissingDownloader) DownloadAudio(_ context.Context, _, _ string) error {
	return ErrFFmpegNotFound
}

func newTestRunnerWithDownloader() *Runner {
	r := newTestRunner()
	r.URLDownloaderFactory = func() URLDownloader { return &fakeInstalledDownloader{title: "Test Title", id: "abc123"} }
	return r
}

func TestRunnerURLTaskResolvesTitleAndID(t *testing.T) {
	r := newTestRunnerWithDownloader()
	tasks := []FileTask{
		{InputPath: "https://example.com/video", OutputPath: "placeholder.txt", Kind: KindURL},
	}
	resolved, err := r.resolveURLTasks(context.Background(), tasks, false)
	require.NoError(t, err)
	require.Len(t, resolved, 1)
	require.Equal(t, "Test Title.abc123.txt", resolved[0].OutputPath)
}

func TestRunnerURLTaskFallsBackToSlugOnTitleError(t *testing.T) {
	r := newTestRunner()
	r.URLDownloaderFactory = func() URLDownloader {
		return &fakeInstalledDownloader{titleErr: fmt.Errorf("title fail")}
	}
	tasks := []FileTask{
		{InputPath: "https://example.com/watch?v=123", OutputPath: "placeholder.txt", Kind: KindURL},
	}
	resolved, err := r.resolveURLTasks(context.Background(), tasks, false)
	require.NoError(t, err)
	require.Len(t, resolved, 1)
	require.Equal(t, "example.com_watch_v_123.txt", resolved[0].OutputPath)
}

func TestRunnerURLTaskSkipsExistingTranscript(t *testing.T) {
	dir := t.TempDir()
	existingTxt := filepath.Join(dir, "Test Title.abc123.txt")
	require.NoError(t, os.WriteFile(existingTxt, []byte("existing"), 0o644))
	// Both outputs must exist for the task to be skipped.
	existingVTT := filepath.Join(dir, "Test Title.abc123.vtt")
	require.NoError(t, os.WriteFile(existingVTT, []byte("WEBVTT"), 0o644))

	r := newTestRunnerWithDownloader()
	tasks := []FileTask{
		{InputPath: "https://example.com/video", OutputPath: "placeholder.txt", Kind: KindURL},
	}
	// resolveURLTasks uses the task's OutputPath dir; we need to run from the dir
	// so that the resolved title-based path lands in the same dir.
	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	defer os.Chdir(origDir)

	resolved, err := r.resolveURLTasks(context.Background(), tasks, false)
	require.NoError(t, err)
	require.Len(t, resolved, 1)
	require.True(t, resolved[0].Skipped)
	require.Contains(t, resolved[0].SkipReason, "already exist")
}

func TestRunnerURLTaskForceOverridesSkip(t *testing.T) {
	dir := t.TempDir()
	existingTxt := filepath.Join(dir, "Test Title.abc123.txt")
	require.NoError(t, os.WriteFile(existingTxt, []byte("existing"), 0o644))

	r := newTestRunnerWithDownloader()
	tasks := []FileTask{
		{InputPath: "https://example.com/video", OutputPath: "placeholder.txt", Kind: KindURL},
	}
	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	defer os.Chdir(origDir)

	resolved, err := r.resolveURLTasks(context.Background(), tasks, true)
	require.NoError(t, err)
	require.Len(t, resolved, 1)
	require.False(t, resolved[0].Skipped)
}

func TestRunnerURLTaskChecksDownloader(t *testing.T) {
	r := newTestRunner()
	r.URLDownloaderFactory = func() URLDownloader { return &fakeMissingDownloader{} }
	r.LookupEnv = func(string) string { return "key" }

	_, err := r.Run(context.Background(), Options{
		Args: []string{"https://example.com/video"},
	})
	require.ErrorIs(t, err, ErrFFmpegNotFound)
}

func TestRunnerURLTaskEndToEnd(t *testing.T) {
	dir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	defer os.Chdir(origDir)

	r := newTestRunnerWithDownloader()
	r.LookupEnv = func(string) string { return "key" }
	r.STTFactory = func(string) STTClient {
		return &fakeSTTClient{result: STTResponse{Text: "transcript", Duration: 60.0}}
	}

	_, err = r.Run(context.Background(), Options{
		Args: []string{"https://example.com/video"},
	})
	require.NoError(t, err)
	// Verify the transcript file was written with the resolved title.
	_, err = os.Stat(filepath.Join(dir, "Test Title.abc123.txt"))
	require.NoError(t, err)
}

func TestRunnerJSONModeReturnsTasks(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "audio.mp3")
	require.NoError(t, os.WriteFile(input, []byte("x"), 0o644))

	r := newTestRunner()
	r.LookupEnv = func(string) string { return "key" }
	r.STTFactory = func(string) STTClient {
		return &fakeSTTClient{result: STTResponse{Text: "hello", Duration: 30.0}}
	}

	result, err := r.Run(context.Background(), Options{
		Args: []string{input},
	})
	require.NoError(t, err)
	require.Len(t, result.Tasks, 1)
	require.Equal(t, input, result.Tasks[0].InputPath)
	require.Equal(t, filepath.Join(dir, "audio.txt"), result.Tasks[0].OutputPath)
	require.Equal(t, 30.0, result.Tasks[0].Duration)
	require.True(t, result.Tasks[0].Success)
	require.Greater(t, result.Tasks[0].Elapsed, 0.0)
}
