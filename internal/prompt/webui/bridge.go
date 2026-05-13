package webui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

type uiMode string

const (
	modePrompt  uiMode = "prompt"
	modeCapture uiMode = "capture"
	modeManage  uiMode = "manage"
)

// initPayload is what the JS layer requests at boot.
type initPayload struct {
	Mode           uiMode              `json:"mode"`
	Title          string              `json:"title"`
	Subtitle       string              `json:"subtitle"`
	AvailableFiles []string            `json:"availableFiles"`
	SavedPrompts   []SavedPromptOption `json:"savedPrompts"`
}

// submitPayload is what the JS layer sends back when the user clicks a button.
type submitPayload struct {
	Action string   `json:"action"` // useOnce | useAndSave | use | updateAndSave | cancel | save | delete | close
	Files  []string `json:"files"`
	Text   string   `json:"text"`
	Name   string   `json:"name"`
	// LoadedName is the saved-prompt name that was loaded into the editor
	// when this submit happened (empty if none). Used to derive update target.
	LoadedName string `json:"loadedName"`
}

// submitResponse is returned by handleSubmit to the JS layer.
// Exactly one of Error or Confirm is non-empty on a meaningful response;
// both empty means the window closes without showing any modal.
// Prompts is populated in manage mode after save/delete to refresh the UI list.
type submitResponse struct {
	Error   string              `json:"error"`
	Confirm string              `json:"confirm"`
	Prompts []SavedPromptOption `json:"prompts"`
}

// bridge holds the state shared between the webview UI thread and the caller
// goroutine. The result is delivered exactly once; subsequent submit/cancel
// calls are ignored. done is closed when the result becomes available so
// watchers can react without consuming it. readyToClose is closed either
// immediately (for use/useOnce/cancel) or after the JS layer calls gtcClose
// (for save actions that show a confirmation modal first).
type bridge struct {
	mode  uiMode
	init  initPayload
	store PromptStore // nil for flow modes
	ctx   context.Context

	mu           sync.Mutex
	result       submitResult
	done         chan struct{}
	closed       bool
	readyToClose chan struct{}
	closeOnce    sync.Once
}

type submitResult struct {
	files  []string
	result Result
	err    error
}

func newBridge(mode uiMode, title, subtitle string, files []string, saved []SavedPromptOption) *bridge {
	if saved == nil {
		saved = []SavedPromptOption{}
	}
	if files == nil {
		files = []string{}
	}
	return &bridge{
		mode: mode,
		init: initPayload{
			Mode:           mode,
			Title:          title,
			Subtitle:       subtitle,
			AvailableFiles: files,
			SavedPrompts:   saved,
		},
		done:         make(chan struct{}),
		readyToClose: make(chan struct{}),
	}
}

func newBridgeWithStore(ctx context.Context, mode uiMode, title, subtitle string, files []string, saved []SavedPromptOption, store PromptStore) *bridge {
	b := newBridge(mode, title, subtitle, files, saved)
	b.ctx = ctx
	b.store = store
	return b
}

// Done returns a channel that closes when a result is available.
func (b *bridge) Done() <-chan struct{} { return b.done }

// ReadyToClose returns a channel that closes when the window may be terminated.
// For save actions this happens after the JS confirmation modal is dismissed;
// for all other actions it closes at the same time as Done().
func (b *bridge) ReadyToClose() <-chan struct{} { return b.readyToClose }

func (b *bridge) signalClose() {
	b.closeOnce.Do(func() { close(b.readyToClose) })
}

// Result returns the stored result. It is only meaningful after Done() fires.
func (b *bridge) Result() submitResult {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.result
}

// handleInit is bound to JS as gtcInit() — returns the page state.
func (b *bridge) handleInit() initPayload {
	return b.init
}

// handleSubmit is bound to JS as gtcSubmit(payload).
// Returns a submitResponse where:
//   - Error non-empty: validation failed; UI shows error modal and stays open.
//   - Confirm non-empty: save succeeded; UI shows confirmation modal, then
//     calls gtcClose() when the user dismisses it.
//   - Prompts non-empty: manage-mode list refresh; UI updates dropdown and stays open.
//   - Both empty: immediate close (use / useOnce / close in manage mode).
func (b *bridge) handleSubmit(p submitPayload) submitResponse {
	if errMsg := b.validate(p); errMsg != "" {
		return submitResponse{Error: errMsg}
	}

	if b.mode == modeManage {
		return b.handleManageAction(p)
	}

	res := b.toResult(p)
	b.deliver(res)

	switch p.Action {
	case "useAndSave":
		name := strings.TrimSpace(p.Name)
		return submitResponse{Confirm: fmt.Sprintf("Prompt %q saved to library.", name)}
	case "updateAndSave":
		name := strings.TrimSpace(p.LoadedName)
		return submitResponse{Confirm: fmt.Sprintf("Prompt %q updated.", name)}
	default:
		// useOnce, use, cancel — close immediately.
		b.signalClose()
		return submitResponse{}
	}
}

func (b *bridge) handleManageAction(p submitPayload) submitResponse {
	switch p.Action {
	case "close":
		b.deliver(submitResult{})
		b.signalClose()
		return submitResponse{}

	case "save":
		name := strings.TrimSpace(p.Name)
		loaded := strings.TrimSpace(p.LoadedName)
		text := strings.TrimSpace(p.Text)
		var err error
		if loaded == "" {
			err = b.store.SavePrompt(b.ctx, name, text)
		} else {
			err = b.store.UpsertPrompt(b.ctx, loaded, text)
		}
		if err != nil {
			return submitResponse{Error: err.Error()}
		}
		prompts, _ := b.store.ListPrompts(b.ctx)
		return submitResponse{Prompts: prompts}

	case "delete":
		loaded := strings.TrimSpace(p.LoadedName)
		if err := b.store.DeletePrompt(b.ctx, loaded); err != nil {
			return submitResponse{Error: err.Error()}
		}
		prompts, _ := b.store.ListPrompts(b.ctx)
		return submitResponse{Prompts: prompts}

	default:
		return submitResponse{Error: fmt.Sprintf("Unknown manage action: %q.", p.Action)}
	}
}

// handleClose is bound to JS as gtcClose() — called after the JS confirmation
// modal is dismissed for save actions, allowing the window to terminate.
func (b *bridge) handleClose() {
	b.signalClose()
}

// handleCancel is bound to JS as gtcCancel() — fired when the user closes the
// window or hits Escape.
func (b *bridge) handleCancel() {
	b.deliver(submitResult{err: ErrCanceled})
	b.signalClose()
}

func (b *bridge) deliver(res submitResult) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.result = res
	b.closed = true
	close(b.done)
}

func (b *bridge) cancelWith(err error) {
	if err == nil {
		err = ErrCanceled
	}
	b.deliver(submitResult{err: err})
	b.signalClose()
}

func (b *bridge) validate(p submitPayload) string {
	text := strings.TrimSpace(p.Text)

	if b.mode == modeManage {
		switch p.Action {
		case "close":
			return ""
		case "save":
			if text == "" {
				return "Prompt content is required."
			}
			if strings.TrimSpace(p.LoadedName) == "" && strings.TrimSpace(p.Name) == "" {
				return "Prompt name is required."
			}
			return ""
		case "delete":
			if strings.TrimSpace(p.LoadedName) == "" {
				return "No saved prompt is selected to delete."
			}
			return ""
		default:
			return fmt.Sprintf("Unknown action: %q.", p.Action)
		}
	}

	switch p.Action {
	case "cancel":
		return ""
	case "useOnce", "use":
		if text == "" {
			return "Instructions are required."
		}
	case "useAndSave":
		if text == "" {
			return "Instructions are required."
		}
		if strings.TrimSpace(p.Name) == "" {
			return "Prompt name is required."
		}
	case "updateAndSave":
		if text == "" {
			return "Instructions are required."
		}
		if strings.TrimSpace(p.LoadedName) == "" {
			return "No saved prompt is loaded to update."
		}
	default:
		return fmt.Sprintf("Unknown action: %q.", p.Action)
	}

	if b.mode == modeCapture && p.Action != "cancel" {
		if len(filterNonEmpty(p.Files)) == 0 {
			return "Select at least one file to transcribe."
		}
	}
	return ""
}

func (b *bridge) toResult(p submitPayload) submitResult {
	text := strings.TrimSpace(p.Text)
	files := filterNonEmpty(p.Files)

	switch p.Action {
	case "cancel":
		return submitResult{err: ErrCanceled}
	case "useOnce", "use":
		return submitResult{
			files: files,
			result: Result{
				Text:      text,
				CacheMode: CacheModeNone,
			},
		}
	case "useAndSave":
		return submitResult{
			files: files,
			result: Result{
				Text:      text,
				Cache:     true,
				CacheName: strings.TrimSpace(p.Name),
				CacheMode: CacheModeCreate,
			},
		}
	case "updateAndSave":
		return submitResult{
			files: files,
			result: Result{
				Text:      text,
				Cache:     true,
				CacheName: strings.TrimSpace(p.LoadedName),
				CacheMode: CacheModeUpdate,
			},
		}
	}
	return submitResult{err: errors.New("webui: unreachable action")}
}

func filterNonEmpty(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return out
}
