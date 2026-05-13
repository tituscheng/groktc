package webui

import (
	"context"
	"fmt"
	"runtime"

	"github.com/tituscheng/groktc/internal/dockicon"

	webview "github.com/webview/webview_go"
)

// Prompt opens the webview window in prompt-only mode (file list is hidden).
func Prompt(ctx context.Context, title, subtitle string, saved []SavedPromptOption) (Result, error) {
	out, err := run(ctx, modePrompt, title, subtitle, nil, saved)
	return out.result, err
}

// Capture opens the webview window in combined mode (file picker + prompt).
func Capture(ctx context.Context, title, subtitle string, available []string, saved []SavedPromptOption) (CaptureResult, error) {
	out, err := run(ctx, modeCapture, title, subtitle, available, saved)
	if err != nil {
		return CaptureResult{}, err
	}
	return CaptureResult{
		Files:  out.files,
		Result: out.result,
	}, nil
}

// Manage opens the webview window in management mode for creating, editing,
// and deleting saved prompts. The session stays open until the user clicks Close.
func Manage(ctx context.Context, title, subtitle string, store PromptStore, saved []SavedPromptOption) error {
	out, err := runWithStore(ctx, modeManage, title, subtitle, nil, saved, store)
	if err != nil {
		return err
	}
	return out.err
}

func run(ctx context.Context, mode uiMode, title, subtitle string, files []string, saved []SavedPromptOption) (submitResult, error) {
	return runWithStore(ctx, mode, title, subtitle, files, saved, nil)
}

func runWithStore(ctx context.Context, mode uiMode, title, subtitle string, files []string, saved []SavedPromptOption, store PromptStore) (submitResult, error) {
	if err := ctx.Err(); err != nil {
		return submitResult{}, err
	}

	// Webview requires the Cocoa main thread on macOS. Cobra dispatches RunE
	// from the main goroutine, so locking the OS thread here keeps us pinned
	// to it for the lifetime of the window.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Hide the macOS dock icon before bringing up the webview so the app
	// doesn't show "not responding" once the window closes and we return to
	// the terminal flow.
	dockicon.Hide()

	// Install the standard Edit menu on macOS so Cmd-X/C/V/Z reach the
	// WKWebView's first responder. No-op on Linux/Windows.
	installPlatformMenus()

	srv, err := startAssetServer()
	if err != nil {
		return submitResult{}, err
	}
	defer srv.Shutdown()

	var br *bridge
	if store != nil {
		br = newBridgeWithStore(ctx, mode, title, subtitle, files, saved, store)
	} else {
		br = newBridge(mode, title, subtitle, files, saved)
	}

	w := webview.New(false)
	if w == nil {
		return submitResult{}, fmt.Errorf("webui: failed to create webview")
	}
	defer w.Destroy()

	w.SetTitle(title)
	w.SetSize(860, 720, webview.HintNone)

	if err := w.Bind("gtcInit", br.handleInit); err != nil {
		return submitResult{}, fmt.Errorf("webui: bind gtcInit: %w", err)
	}
	if err := w.Bind("gtcSubmit", br.handleSubmit); err != nil {
		return submitResult{}, fmt.Errorf("webui: bind gtcSubmit: %w", err)
	}
	if err := w.Bind("gtcCancel", br.handleCancel); err != nil {
		return submitResult{}, fmt.Errorf("webui: bind gtcCancel: %w", err)
	}
	if err := w.Bind("gtcClose", br.handleClose); err != nil {
		return submitResult{}, fmt.Errorf("webui: bind gtcClose: %w", err)
	}

	// Terminate the webview once the window is ready to close. For save
	// actions this fires after the JS confirmation modal is dismissed;
	// for all other actions it fires immediately after submit/cancel.
	stopWatcher := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			br.cancelWith(ctx.Err())
			w.Dispatch(w.Terminate)
		case <-br.ReadyToClose():
			w.Dispatch(w.Terminate)
		case <-stopWatcher:
			return
		}
	}()

	w.Navigate(srv.URL())
	w.Run()
	close(stopWatcher)

	// The webview library activates the app on macOS. Deactivate it before
	// continuing to avoid the beach-ball cursor while the rest of the
	// program does non-UI work.
	dockicon.Deactivate()

	// If the user closed the window without submitting, treat it as a
	// cancellation. deliver is idempotent, so a real result still wins.
	br.cancelWith(ErrCanceled)
	res := br.Result()
	return res, res.err
}
