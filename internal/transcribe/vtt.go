package transcribe

import (
	"fmt"
	"path/filepath"
	"strings"
)

// VTT cue sizing limits. Cues break at sentence boundaries; these caps split
// sentences that run too long for comfortable on-screen reading.
const (
	maxCueSeconds = 6.0 // max wall-clock span of a single cue
	maxCueChars   = 84  // max characters per cue (~two lines)
	maxLineChars  = 42  // wrap width per displayed line
)

// vttPath returns the .vtt sibling path for a transcript output path, e.g.
// "/dir/recording.txt" -> "/dir/recording.vtt".
func vttPath(txtPath string) string {
	return strings.TrimSuffix(txtPath, filepath.Ext(txtPath)) + ".vtt"
}

type cue struct {
	start float64
	end   float64
	text  string
}

// BuildVTT renders a WebVTT document from an STT response. It groups
// word-level timings into sentence-based cues. If the response carries no
// word timings, it falls back to a single cue spanning the whole clip so the
// output is always a valid .vtt.
func BuildVTT(resp STTResponse) string {
	cues := groupCues(resp.Words)
	if len(cues) == 0 {
		end := resp.Duration
		if end <= 0 {
			// No words and no duration: still emit one readable cue.
			end = 1
		}
		cues = []cue{{start: 0, end: end, text: strings.TrimSpace(resp.Text)}}
	}

	var b strings.Builder
	b.WriteString("WEBVTT\n\n")
	for _, c := range cues {
		fmt.Fprintf(&b, "%s --> %s\n", formatTimestamp(c.start), formatTimestamp(c.end))
		b.WriteString(wrapText(c.text))
		b.WriteString("\n\n")
	}
	return b.String()
}

// groupCues splits words into cues, starting a new cue after a word that ends
// a sentence, or when adding the next word would exceed the character or
// duration caps.
func groupCues(words []Word) []cue {
	var cues []cue
	var cur []Word

	flush := func() {
		if len(cur) == 0 {
			return
		}
		texts := make([]string, len(cur))
		for i, w := range cur {
			texts[i] = w.Text
		}
		cues = append(cues, cue{
			start: cur[0].Start,
			end:   cur[len(cur)-1].End,
			text:  strings.Join(texts, " "),
		})
		cur = nil
	}

	for _, w := range words {
		if text := strings.TrimSpace(w.Text); text == "" {
			continue
		}
		if len(cur) > 0 {
			// Would this word overflow the current cue's size limits?
			projectedChars := cueChars(cur) + 1 + len(strings.TrimSpace(w.Text))
			projectedSpan := w.End - cur[0].Start
			if projectedChars > maxCueChars || projectedSpan > maxCueSeconds {
				flush()
			}
		}
		cur = append(cur, w)
		if endsSentence(w.Text) {
			flush()
		}
	}
	flush()
	return cues
}

// cueChars returns the rendered character length of the words joined by spaces.
func cueChars(words []Word) int {
	n := 0
	for i, w := range words {
		if i > 0 {
			n++ // joining space
		}
		n += len(strings.TrimSpace(w.Text))
	}
	return n
}

// endsSentence reports whether a word ends a sentence, ignoring trailing
// closing quotes/brackets (e.g. `dog."` or `done!)`).
func endsSentence(word string) bool {
	s := strings.TrimRight(strings.TrimSpace(word), "\"'”’)]}")
	if s == "" {
		return false
	}
	switch s[len(s)-1] {
	case '.', '?', '!':
		return true
	default:
		return false
	}
}

// formatTimestamp renders seconds as a WebVTT timestamp "HH:MM:SS.mmm".
func formatTimestamp(seconds float64) string {
	if seconds < 0 {
		seconds = 0
	}
	totalMillis := int64(seconds*1000 + 0.5)
	ms := totalMillis % 1000
	totalSecs := totalMillis / 1000
	s := totalSecs % 60
	m := (totalSecs / 60) % 60
	h := totalSecs / 3600
	return fmt.Sprintf("%02d:%02d:%02d.%03d", h, m, s, ms)
}

// wrapText word-wraps a cue's text to at most two lines of maxLineChars. Any
// overflow beyond two lines stays on the second line (cue caps keep this rare).
func wrapText(s string) string {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return s
	}
	var lines []string
	var line string
	for _, w := range fields {
		switch {
		case line == "":
			line = w
		case len(line)+1+len(w) <= maxLineChars && len(lines) < 1:
			line += " " + w
		case len(lines) < 1:
			lines = append(lines, line)
			line = w
		default:
			// Already on the last allowed line; keep appending.
			line += " " + w
		}
	}
	lines = append(lines, line)
	return strings.Join(lines, "\n")
}
