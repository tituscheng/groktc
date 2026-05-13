package markdown

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewCommandFlags(t *testing.T) {
	cmd := NewCommand()

	require.NoError(t, cmd.Flags().Set("prompt", "clean it"))
	require.NoError(t, cmd.Flags().Set("prompt-file", "prompt.txt"))
	require.NoError(t, cmd.Flags().Set("no-gui", "true"))
	require.NoError(t, cmd.Flags().Set("force", "true"))
	require.NoError(t, cmd.Flags().Set("model", "grok-4.3"))
	require.NoError(t, cmd.Flags().Set("effort", "high"))
	require.NoError(t, cmd.Flags().Set("json", "true"))
	require.NotNil(t, cmd)
}
