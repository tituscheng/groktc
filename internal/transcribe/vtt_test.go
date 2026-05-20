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
