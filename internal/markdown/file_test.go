package markdown

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDiscoverBatchTasksTopLevelTxtOnly(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "one.txt"), "hello")
	mustWrite(t, filepath.Join(dir, ".hidden.txt"), "secret")
	mustWrite(t, filepath.Join(dir, "note.md"), "skip")
	require.NoError(t, os.Mkdir(filepath.Join(dir, "nested"), 0o755))
	mustWrite(t, filepath.Join(dir, "nested", "two.txt"), "nested")

	tasks, err := discoverBatchTasks(dir, false)
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	require.Equal(t, filepath.Join(dir, "one.txt"), tasks[0].InputPath)
	require.Equal(t, filepath.Join(dir, "one.md"), tasks[0].OutputPath)
}

func TestDiscoverBatchTasksSkipsExistingNonEmptyMarkdown(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "done.txt"), "hello")
	mustWrite(t, filepath.Join(dir, "done.md"), "# Done")
	mustWrite(t, filepath.Join(dir, "empty.txt"), "hello")
	mustWrite(t, filepath.Join(dir, "empty.md"), "")

	tasks, err := discoverBatchTasks(dir, false)
	require.NoError(t, err)
	require.Len(t, tasks, 2)

	require.Equal(t, filepath.Join(dir, "done.txt"), tasks[0].InputPath)
	require.True(t, tasks[0].Skipped)
	require.Equal(t, filepath.Join(dir, "empty.txt"), tasks[1].InputPath)
	require.False(t, tasks[1].Skipped)
}

func TestDiscoverBatchTasksForceIncludesOverwriteCandidates(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "done.txt"), "hello")
	mustWrite(t, filepath.Join(dir, "done.md"), "# Done")

	tasks, err := discoverBatchTasks(dir, true)
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	require.False(t, tasks[0].Skipped)
}

func TestResolveSingleTaskValidation(t *testing.T) {
	dir := t.TempDir()
	_, err := resolveSingleTask(filepath.Join(dir, "missing.txt"), false)
	require.Error(t, err)

	mustWrite(t, filepath.Join(dir, "not-md.md"), "hello")
	_, err = resolveSingleTask(filepath.Join(dir, "not-md.md"), false)
	require.Error(t, err)
}

func mustWrite(t *testing.T, path string, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}
