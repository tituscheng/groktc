package transcribe

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckSTTUploadSizeMP3UnderLimit(t *testing.T) {
	t.Parallel()

	result := CheckSTTUploadSize(KindMP3, 100*1024*1024, 3600, 0)
	require.False(t, result.OverLimit)
	require.Equal(t, int64(100*1024*1024), result.UploadBytes)
	require.False(t, result.Estimated)
}

func TestCheckSTTUploadSizeMP3OverLimit(t *testing.T) {
	t.Parallel()

	result := CheckSTTUploadSize(KindMP3, 501*1024*1024, 3600, 0)
	require.True(t, result.OverLimit)
	require.Equal(t, int64(501*1024*1024), result.UploadBytes)
}

func TestCheckSTTUploadSizeMP4UnderLimit(t *testing.T) {
	t.Parallel()

	// 1 hour at 64 kbps ≈ 28.8 MB
	result := CheckSTTUploadSize(KindMP4, 2*1024*1024*1024, 3600, 64000)
	require.False(t, result.OverLimit)
	require.False(t, result.Estimated)
}

func TestCheckSTTUploadSizeMP4OverLimitWithFallbackBitrate(t *testing.T) {
	t.Parallel()

	// 10 hours at fallback 165 kbps ≈ 742 MB
	result := CheckSTTUploadSize(KindMP4, 5*1024*1024*1024, 36000, 0)
	require.True(t, result.OverLimit)
	require.True(t, result.Estimated)
}

func TestCheckSTTUploadSizeMP4OverLimitWithHighBitrate(t *testing.T) {
	t.Parallel()

	// 2 hours at 640 kbps ≈ 576 MB
	result := CheckSTTUploadSize(KindMP4, 3*1024*1024*1024, 7200, 640000)
	require.True(t, result.OverLimit)
	require.False(t, result.Estimated)
}
