package transcribe

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func TestKindForExt(t *testing.T) {
	cases := []struct {
		ext  string
		kind InputKind
		ok   bool
	}{
		{".mp4", KindMP4, true},
		{".MP4", KindMP4, true},
		{".mp3", KindMP3, true},
		{".MP3", KindMP3, true},
		{".txt", 0, false},
		{".TXT", 0, false},
		{".wav", 0, false},
		{".md", 0, false},
		{"", 0, false},
	}
	for _, tc := range cases {
		k, ok := kindForExt(tc.ext)
		require.Equal(t, tc.ok, ok, "ext %s", tc.ext)
		if ok {
			require.Equal(t, tc.kind, k, "ext %s", tc.ext)
		}
	}
}

func TestDiscoverMediaTasksFindsAllTypes(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "video.mp4"), "x")
	mustWriteFile(t, filepath.Join(dir, "audio.mp3"), "x")
	mustWriteFile(t, filepath.Join(dir, "notes.txt"), "x")
	mustWriteFile(t, filepath.Join(dir, "image.png"), "x")  // ignored
	mustWriteFile(t, filepath.Join(dir, "doc.md"), "x")     // ignored
	mustWriteFile(t, filepath.Join(dir, ".hidden.mp3"), "x") // ignored

	tasks, err := discoverMediaTasks(dir, false)
	require.NoError(t, err)
	require.Len(t, tasks, 2)

	names := make([]string, len(tasks))
	for i, task := range tasks {
		names[i] = filepath.Base(task.InputPath)
	}
	require.Contains(t, names, "video.mp4")
	require.Contains(t, names, "audio.mp3")
}

func TestDiscoverMediaTasksSkipsExistingTranscript(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "done.mp3"), "audio")
	mustWriteFile(t, filepath.Join(dir, "done.txt"), "transcript")
	mustWriteFile(t, filepath.Join(dir, "todo.mp3"), "audio")

	tasks, err := discoverMediaTasks(dir, false)
	require.NoError(t, err)
	require.Len(t, tasks, 2)

	var skipped, pending int
	for _, task := range tasks {
		if task.Skipped {
			skipped++
		} else {
			pending++
		}
	}
	require.Equal(t, 1, skipped)
	require.Equal(t, 1, pending)
}

func TestDiscoverMediaTasksForceOverridesSkip(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "done.mp3"), "audio")
	mustWriteFile(t, filepath.Join(dir, "done.txt"), "transcript")

	tasks, err := discoverMediaTasks(dir, true)
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	require.False(t, tasks[0].Skipped)
}

func TestDiscoverMediaTasksExcludesSubdirectories(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "sub"), 0o755))
	mustWriteFile(t, filepath.Join(dir, "sub", "nested.mp3"), "x")
	mustWriteFile(t, filepath.Join(dir, "top.mp3"), "x")

	tasks, err := discoverMediaTasks(dir, false)
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	require.Equal(t, filepath.Join(dir, "top.mp3"), tasks[0].InputPath)
}

func TestResolveSingleTaskUnsupportedExtension(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.wav")
	mustWriteFile(t, path, "x")
	_, err := resolveSingleTask(path, false, "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported input")
}

func TestResolveSingleTaskMissingFile(t *testing.T) {
	_, err := resolveSingleTask("/tmp/does-not-exist.mp3", false, "")
	require.Error(t, err)
}

func TestResolveExplicitTasksDeduplicates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.mp3")
	mustWriteFile(t, path, "x")

	tasks, err := resolveExplicitTasks([]string{path, path}, false, "")
	require.NoError(t, err)
	require.Len(t, tasks, 1)
}

func TestResolveExplicitTasksRejectsOutputWithMultipleInputs(t *testing.T) {
	dir := t.TempDir()
	p1 := filepath.Join(dir, "a.mp3")
	p2 := filepath.Join(dir, "b.mp3")
	mustWriteFile(t, p1, "x")
	mustWriteFile(t, p2, "x")

	_, err := ResolveTasks([]string{p1, p2}, false, "out.txt")
	require.Error(t, err)
	require.Contains(t, err.Error(), "--output can only be used with a single input")
}

func TestTaskForMediaFileOutputPath(t *testing.T) {
	task := taskForMediaFile("/dir/recording.mp3", KindMP3, "")
	require.Equal(t, "/dir/recording.mp3", task.InputPath)
	require.Equal(t, "/dir/recording.txt", task.OutputPath)
	require.Equal(t, KindMP3, task.Kind)
}

func TestTaskForMediaFileWithCustomOutput(t *testing.T) {
	task := taskForMediaFile("/dir/recording.mp3", KindMP3, "/custom/out.txt")
	require.Equal(t, "/dir/recording.mp3", task.InputPath)
	require.Equal(t, "/custom/out.txt", task.OutputPath)
}

func TestDiscoverAvailableFiles(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "a.mp4"), "x")
	mustWriteFile(t, filepath.Join(dir, "b.mp3"), "x")
	mustWriteFile(t, filepath.Join(dir, "c.txt"), "x")
	mustWriteFile(t, filepath.Join(dir, ".hidden.mp3"), "x")
	mustWriteFile(t, filepath.Join(dir, "skip.png"), "x")

	files, err := DiscoverAvailableFiles(dir)
	require.NoError(t, err)
	require.Len(t, files, 2)
}

func TestResolveExplicitTasksDetectsURL(t *testing.T) {
	tasks, err := resolveExplicitTasks([]string{"https://example.com/watch?v=123"}, false, "")
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	require.Equal(t, KindURL, tasks[0].Kind)
	require.Equal(t, "https://example.com/watch?v=123", tasks[0].InputPath)
	require.True(t, strings.HasSuffix(tasks[0].OutputPath, ".txt"))
}

func TestResolveExplicitTasksURLWithCustomOutput(t *testing.T) {
	tasks, err := resolveExplicitTasks([]string{"https://example.com/watch?v=123"}, false, "custom.txt")
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	require.Equal(t, "custom.txt", tasks[0].OutputPath)
}

func TestResolveExplicitTasksMixesFilesAndURLs(t *testing.T) {
	dir := t.TempDir()
	mp3 := filepath.Join(dir, "audio.mp3")
	mustWriteFile(t, mp3, "x")

	tasks, err := resolveExplicitTasks([]string{mp3, "https://example.com/video"}, false, "")
	require.NoError(t, err)
	require.Len(t, tasks, 2)

	var kinds []InputKind
	for _, task := range tasks {
		kinds = append(kinds, task.Kind)
	}
	require.Contains(t, kinds, KindMP3)
	require.Contains(t, kinds, KindURL)
}

func TestResolveExplicitTasksDeduplicatesURLs(t *testing.T) {
	tasks, err := resolveExplicitTasks([]string{
		"https://example.com/video",
		"https://example.com/video",
	}, false, "")
	require.NoError(t, err)
	require.Len(t, tasks, 1)
}

func TestURLSlugSanitizes(t *testing.T) {
	require.Equal(t, "example.com_watch_v_123", urlSlug("https://example.com/watch?v=123"))
	require.Equal(t, "example.com_path", urlSlug("http://example.com/path/"))
}

func TestSanitizeFilename(t *testing.T) {
	require.Equal(t, "My Video_ A Story", sanitizeFilename("My Video: A Story"))
	require.Equal(t, "file_name", sanitizeFilename("file/name"))
	require.Equal(t, "a_b", sanitizeFilename("a<b"))
}
