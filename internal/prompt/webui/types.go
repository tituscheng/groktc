package webui

import "context"

import "errors"

type CacheMode string

const (
	CacheModeNone   CacheMode = "none"
	CacheModeCreate CacheMode = "create"
	CacheModeUpdate CacheMode = "update"
)

type SavedPromptOption struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type Result struct {
	Text      string
	Cache     bool
	CacheName string
	CacheMode CacheMode
}

type CaptureResult struct {
	Files []string
	Result
}

var ErrCanceled = errors.New("prompt capture canceled")

// PromptStore is the minimal interface the management UI needs to perform
// CRUD operations on the prompt library.
type PromptStore interface {
	SavePrompt(ctx context.Context, name string, content string) error
	UpsertPrompt(ctx context.Context, name string, content string) error
	DeletePrompt(ctx context.Context, name string) error
	ListPrompts(ctx context.Context) ([]SavedPromptOption, error)
	Close() error
}
