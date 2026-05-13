package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPromptPreviewShortText(t *testing.T) {
	require.Equal(t, "hello world", promptPreview("hello world"))
	require.Equal(t, "", promptPreview(""))
	require.Equal(t, "", promptPreview("   \n  "))
}

func TestPromptPreviewLongTextTruncated(t *testing.T) {
	in := strings.Repeat("a", 100)
	out := promptPreview(in)
	require.Len(t, out, 60)
	require.True(t, strings.HasSuffix(out, "..."))
}

func TestPromptPreviewCollapsesWhitespace(t *testing.T) {
	require.Equal(t, "hello world", promptPreview("hello   world\n\n  "))
	require.Equal(t, "a b c d", promptPreview("\ta\nb\nc d  "))
}

func TestEditInEditorRoundTrip(t *testing.T) {
	// Use a fake editor that just appends text to the temp file.
	// We exec /bin/sh -c 'echo edited >> "$1"' -- $tmp
	scriptDir := t.TempDir()
	scriptPath := filepath.Join(scriptDir, "fakeeditor.sh")
	const script = `#!/bin/sh
echo "edited content" > "$1"
`
	require.NoError(t, os.WriteFile(scriptPath, []byte(script), 0o755))

	t.Setenv("EDITOR", scriptPath)
	out, err := editInEditor("initial")
	require.NoError(t, err)
	require.Equal(t, "edited content\n", out)
}

func TestEditInEditorPreservesInitialContent(t *testing.T) {
	// Editor that doesn't modify the file (just exits).
	scriptDir := t.TempDir()
	scriptPath := filepath.Join(scriptDir, "noopeditor.sh")
	const script = `#!/bin/sh
exit 0
`
	require.NoError(t, os.WriteFile(scriptPath, []byte(script), 0o755))

	t.Setenv("EDITOR", scriptPath)
	out, err := editInEditor("seeded prompt")
	require.NoError(t, err)
	require.Equal(t, "seeded prompt", out)
}

func TestEditInEditorMissingEditorReturnsError(t *testing.T) {
	t.Setenv("EDITOR", "/nonexistent/binary/that/does/not/exist")
	_, err := editInEditor("")
	require.Error(t, err)
}
