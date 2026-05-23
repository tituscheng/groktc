package transcribe

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseFFprobeOutput(t *testing.T) {
	t.Parallel()

	probe, err := parseFFprobeOutput("duration=123.456\nbit_rate=128000\n")
	require.NoError(t, err)
	require.InDelta(t, 123.456, probe.DurationSeconds, 0.001)
	require.Equal(t, int64(128000), probe.AudioBitrate)
}

func TestParseFFprobeOutputMissingBitrate(t *testing.T) {
	t.Parallel()

	probe, err := parseFFprobeOutput("duration=60.0\n")
	require.NoError(t, err)
	require.InDelta(t, 60.0, probe.DurationSeconds, 0.001)
	require.Equal(t, int64(0), probe.AudioBitrate)
}

func TestParseFFprobeOutputNoDuration(t *testing.T) {
	t.Parallel()

	_, err := parseFFprobeOutput("bit_rate=128000\n")
	require.Error(t, err)
}

func TestExecMediaProberMissingBinary(t *testing.T) {
	t.Parallel()

	prober := &ExecMediaProber{
		LookPath: func(string) (string, error) {
			return "", ErrFFprobeNotFound
		},
	}
	require.ErrorIs(t, prober.CheckInstalled(), ErrFFprobeNotFound)
}
