package transcribe

import "math"

// EstimatedMP3Bitrate is the fallback audio bitrate (bits/sec) used when ffprobe
// does not report one. It approximates the upper bound for ffmpeg -q:a 4 VBR.
const EstimatedMP3Bitrate = 165_000

type SizeCheckResult struct {
	UploadBytes int64
	OverLimit   bool
	Estimated   bool
}

// CheckSTTUploadSize reports whether the payload sent to xAI STT would exceed
// MaxAudioBytes. MP3 uses the file size directly; MP4 estimates extracted MP3 size.
func CheckSTTUploadSize(kind InputKind, fileSize int64, durationSec float64, audioBitrate int64) SizeCheckResult {
	var uploadBytes int64
	estimated := false

	switch kind {
	case KindMP3:
		uploadBytes = fileSize
	case KindMP4:
		bitrate := audioBitrate
		if bitrate <= 0 {
			bitrate = EstimatedMP3Bitrate
			estimated = true
		}
		uploadBytes = int64(math.Ceil(durationSec * float64(bitrate) / 8.0))
	default:
		uploadBytes = fileSize
	}

	return SizeCheckResult{
		UploadBytes: uploadBytes,
		OverLimit:   uploadBytes > MaxAudioBytes,
		Estimated:   estimated,
	}
}
