package transcribe

import (
	"fmt"
	"path/filepath"
	"sort"
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

// BuildVTT renders a spec-compliant WebVTT document from an STT response. It
// groups word-level timings into sentence-based cues. If the response carries
// no word timings, it falls back to a single cue spanning the whole clip so the
// output is always a valid .vtt.
func BuildVTT(resp STTResponse) string {
	words := sanitizeWords(resp.Words)
	cues := groupCues(words)
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
	for i := range cues {
		ensureCueDuration(&cues[i])
		fmt.Fprintf(&b, "%s --> %s\n", formatTimestamp(cues[i].start), formatTimestamp(cues[i].end))
		b.WriteString(escapeVTTText(wrapText(cues[i].text)))
		b.WriteString("\n\n")
	}
	return b.String()
}

// sanitizeWords returns a cleaned copy of the input word slice:
//   - Empty/whitespace-only words are removed.
//   - Words are sorted by Start time ascending.
//   - If Start > End, they are swapped.
//   - Zero-duration words are padded so End >= Start+0.001.
func sanitizeWords(words []Word) []Word {
	filtered := make([]Word, 0, len(words))
	for _, w := range words {
		if strings.TrimSpace(w.Text) == "" {
			continue
		}
		if w.Start > w.End {
			w.Start, w.End = w.End, w.Start
		}
		if w.End < w.Start+0.001 {
			w.End = w.Start + 0.001
		}
		filtered = append(filtered, w)
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Start < filtered[j].Start
	})
	return filtered
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
	s := strings.TrimRight(strings.TrimSpace(word), "\"'\u201d\u2019)]}")
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

// escapeVTTText escapes cue text so it is safe to embed in a WebVTT file.
// It normalizes line endings, collapses blank lines (which would otherwise end
// the cue), and escapes characters that have special meaning in WebVTT:
//   - & → &amp;
//   - < → &lt;
//   - > → &gt;
//
// Escaping '>' also neutralizes literal "-->" sequences, preventing them from
// being misinterpreted as cue timing separators.
func escapeVTTText(s string) string {
	// Normalize all line endings to \n.
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	// Collapse consecutive blank lines — a blank line ends a cue in WebVTT.
	for strings.Contains(s, "\n\n") {
		s = strings.ReplaceAll(s, "\n\n", "\n")
	}
	// Escape special characters.
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// ensureCueDuration guarantees that a cue's end time is strictly greater than
// its start time. If they are equal (or end < start), end is bumped by 0.001s.
func ensureCueDuration(c *cue) {
	if c.end <= c.start {
		c.end = c.start + 0.001
	}
}
