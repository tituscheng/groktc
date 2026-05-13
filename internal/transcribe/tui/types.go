package tui

import "errors"

// ErrCanceled is returned when the user cancels a prompt or selection.
var ErrCanceled = errors.New("canceled")

type CacheMode string

const (
	CacheModeNone   CacheMode = "none"
	CacheModeCreate CacheMode = "create"
	CacheModeUpdate CacheMode = "update"
)

type SavedPromptOption struct {
	Name    string
	Content string
}

// PromptResult is the user's prompt-related decision.
type PromptResult struct {
	Text      string
	Cache     bool
	CacheName string
	CacheMode CacheMode
}

// CaptureResult is returned by Capture for the no-args path.
type CaptureResult struct {
	Files  []string
	Prompt PromptResult
}
