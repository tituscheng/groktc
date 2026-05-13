package tui

import (
	"io"
)

// Capture runs the file picker followed by the prompt selection. Used when
// the user runs `groktc transcribe` with no args.
func Capture(input io.Reader, output io.Writer, available []string, saved []SavedPromptOption) (CaptureResult, error) {
	files, err := PickFiles(input, output, available)
	if err != nil {
		return CaptureResult{}, err
	}

	prompt, err := PromptInteractive(input, output, saved)
	if err != nil {
		return CaptureResult{}, err
	}

	return CaptureResult{Files: files, Prompt: prompt}, nil
}
