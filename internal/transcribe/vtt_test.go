package transcribe

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFormatTimestamp(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{0, "00:00:00.000"},
		{8.45, "00:00:08.450"},
		{61.5, "00:01:01.500"},
		{3661.5, "01:01:01.500"},
		{-1, "00:00:00.000"},
	}
	for _, tc := range cases {
		require.Equal(t, tc.want, formatTimestamp(tc.in), "in=%v", tc.in)
	}
}

func TestVTTPath(t *testing.T) {
	require.Equal(t, "/dir/recording.vtt", vttPath("/dir/recording.txt"))
	require.Equal(t, "name.vtt", vttPath("name.txt"))
}

// probeWords mirrors the real STT response captured during API probing.
var probeWords = []Word{
	{Text: "Hello,", Start: 0.12, End: 0.36},
	{Text: "this", Start: 0.8, End: 0.96},
	{Text: "is", Start: 1.02, End: 1.1},
	{Text: "a", Start: 1.16, End: 1.18},
	{Text: "test", Start: 1.28, End: 1.64},
	{Text: "of", Start: 1.68, End: 1.74},
	{Text: "the", Start: 1.76, End: 1.82},
	{Text: "Grok", Start: 1.86, End: 2.1},
	{Text: "API.", Start: 3.1, End: 3.54},
	{Text: "One,", Start: 4.16, End: 4.32},
	{Text: "two,", Start: 4.4, End: 4.62},
	{Text: "three.", Start: 4.94, End: 5.2},
	{Text: "The", Start: 5.79, End: 5.85},
	{Text: "fox.", Start: 6.61, End: 6.85},
}

func TestBuildVTTSentenceCues(t *testing.T) {
	out := BuildVTT(STTResponse{Words: probeWords, Duration: 6.85})

	require.True(t, strings.HasPrefix(out, "WEBVTT\n\n"), "must start with WEBVTT header")

	// Three sentence-ending words ("API.", "three.", "fox.") => three cues.
	require.Equal(t, 3, strings.Count(out, " --> "), "expected one arrow per cue")

	// First cue spans the first word to "API." and contains its text.
	require.Contains(t, out, "00:00:00.120 --> 00:00:03.540")
	require.Contains(t, out, "Hello, this is a test of the Grok API.")
	require.Contains(t, out, "00:00:04.160 --> 00:00:05.200")
	require.Contains(t, out, "One, two, three.")
}

func TestBuildVTTFallbackWithoutWords(t *testing.T) {
	out := BuildVTT(STTResponse{Text: "hello there", Duration: 3})
	require.Equal(t, "WEBVTT\n\n00:00:00.000 --> 00:00:03.000\nhello there\n\n", out)
}

func TestBuildVTTFallbackNoDuration(t *testing.T) {
	out := BuildVTT(STTResponse{Text: "hi"})
	require.Contains(t, out, "00:00:00.000 --> 00:00:01.000")
	require.Contains(t, out, "hi")
}

func TestGroupCuesSplitsLongSentenceByDuration(t *testing.T) {
	// A single run-on "sentence" (no terminal punctuation) that exceeds the
	// 6s span cap must split into more than one cue.
	words := []Word{
		{Text: "aa", Start: 0, End: 1},
		{Text: "bb", Start: 1, End: 2},
		{Text: "cc", Start: 2, End: 3},
		{Text: "dd", Start: 3, End: 4},
		{Text: "ee", Start: 4, End: 5},
		{Text: "ff", Start: 5, End: 6},
		{Text: "gg", Start: 6, End: 7},
		{Text: "hh", Start: 7, End: 8},
	}
	cues := groupCues(words)
	require.Greater(t, len(cues), 1, "long run-on should split on the duration cap")
}

func TestGroupCuesSplitsLongSentenceByChars(t *testing.T) {
	// One quick "sentence" that exceeds maxCueChars but fits in the time cap.
	var words []Word
	for i := 0; i < 30; i++ {
		words = append(words, Word{Text: "word", Start: float64(i) * 0.1, End: float64(i)*0.1 + 0.05})
	}
	cues := groupCues(words)
	require.Greater(t, len(cues), 1, "long run-on should split on the character cap")
	for _, c := range cues {
		require.LessOrEqual(t, len(strings.ReplaceAll(c.text, "\n", " ")), maxCueChars)
	}
}

func TestGroupCuesEmpty(t *testing.T) {
	require.Empty(t, groupCues(nil))
}

func TestWrapTextTwoLines(t *testing.T) {
	out := wrapText("the quick brown fox jumps over the lazy dog and then some more words")
	lines := strings.Split(out, "\n")
	require.LessOrEqual(t, len(lines), 2)
	require.LessOrEqual(t, len(lines[0]), maxLineChars)
}

func TestEndsSentence(t *testing.T) {
	require.True(t, endsSentence("dog."))
	require.True(t, endsSentence("really?"))
	require.True(t, endsSentence("stop!"))
	require.True(t, endsSentence(`dog."`))
	require.True(t, endsSentence("done!)"))
	require.False(t, endsSentence("hello"))
	require.False(t, endsSentence("mid,"))
}

// --- Spec-compliance tests --------------------------------------------------

func TestEscapeVTTText(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"AT&T", "AT&amp;T"},
		{"5 < 10", "5 &lt; 10"},
		{"A > B", "A &gt; B"},
		{"look --> there", "look --&gt; there"},
		{"multi\r\nline", "multi\nline"},
		{"multi\rline", "multi\nline"},
		{"blank\n\nline", "blank\nline"},
		{"deep\n\n\nblank", "deep\nblank"},
		{"&<>混合", "&amp;&lt;&gt;混合"},
		{"normal text", "normal text"},
	}
	for _, tc := range cases {
		require.Equal(t, tc.want, escapeVTTText(tc.in), "input=%q", tc.in)
	}
}

func TestSanitizeWords(t *testing.T) {
	// Empty words removed.
	out := sanitizeWords([]Word{
		{Text: "", Start: 0, End: 1},
		{Text: "   ", Start: 0, End: 1},
		{Text: "hello", Start: 0, End: 1},
	})
	require.Len(t, out, 1)
	require.Equal(t, "hello", out[0].Text)

	// Out-of-order words sorted.
	out = sanitizeWords([]Word{
		{Text: "world", Start: 2, End: 3},
		{Text: "hello", Start: 0, End: 1},
	})
	require.Len(t, out, 2)
	require.Equal(t, "hello", out[0].Text)
	require.Equal(t, "world", out[1].Text)

	// Reversed start/end swapped.
	out = sanitizeWords([]Word{
		{Text: "oops", Start: 5, End: 2},
	})
	require.Len(t, out, 1)
	require.Equal(t, 2.0, out[0].Start)
	require.Equal(t, 5.0, out[0].End)

	// Zero-duration padded.
	out = sanitizeWords([]Word{
		{Text: "flash", Start: 1.0, End: 1.0},
	})
	require.Len(t, out, 1)
	require.Equal(t, 1.0, out[0].Start)
	require.Equal(t, 1.001, out[0].End)
}

func TestBuildVTTWithSpecialChars(t *testing.T) {
	words := []Word{
		{Text: "AT&T", Start: 0, End: 1},
		{Text: "costs", Start: 1.1, End: 1.5},
		{Text: "<", Start: 1.6, End: 1.7},
		{Text: "$10.", Start: 1.8, End: 2.2},
	}
	out := BuildVTT(STTResponse{Words: words, Duration: 2.2})

	require.Contains(t, out, "AT&amp;T")
	require.Contains(t, out, "&lt;")
	require.NotContains(t, out, "AT&T")
	require.NotContains(t, out, " < ")
}

func TestBuildVTTWithBlankLines(t *testing.T) {
	// Fallback path with text containing blank lines and carriage returns.
	// escapeVTTText normalizes line endings and collapses blank lines;
	// wrapText then collapses all whitespace into space-separated words.
	out := BuildVTT(STTResponse{Text: "line one\r\n\r\nline two", Duration: 5})
	require.Contains(t, out, "line one line two")
	// The file should have exactly two \n\n sequences: one after WEBVTT, one after the cue.
	require.Equal(t, 2, strings.Count(out, "\n\n"), "no extra blank lines inside cue text")
}

func TestBuildVTTOutOfOrderWords(t *testing.T) {
	words := []Word{
		{Text: "world.", Start: 2.0, End: 2.5},
		{Text: "Hello", Start: 0.0, End: 0.5},
	}
	out := BuildVTT(STTResponse{Words: words, Duration: 2.5})

	// After sanitization words are sorted; both fit in one cue under 6s.
	require.Contains(t, out, "Hello world.")
	// The cue spans from the first word to the last.
	require.Contains(t, out, "00:00:00.000 --> 00:00:02.500")
}

func TestBuildVTTZeroDurationWords(t *testing.T) {
	words := []Word{
		{Text: "snap", Start: 1.0, End: 1.0},
	}
	out := BuildVTT(STTResponse{Words: words, Duration: 1.0})

	// Cue must have end > start.
	require.Contains(t, out, "00:00:01.000 --> 00:00:01.001")
	require.Contains(t, out, "snap")
}

func TestBuildVTTWithArrowInText(t *testing.T) {
	// The literal "-->" must not be mistaken for a cue timing separator.
	out := BuildVTT(STTResponse{Text: "look --> there", Duration: 5})
	require.Contains(t, out, "look --&gt; there")
	// There should still be exactly one timing line.
	require.Equal(t, 1, strings.Count(out, " --> "))
}
