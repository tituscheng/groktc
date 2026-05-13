package webui

import (
	"errors"
	"io/fs"
	"testing"
)

func TestBridgeValidate(t *testing.T) {
	cases := []struct {
		name    string
		mode    uiMode
		payload submitPayload
		wantErr string
	}{
		{"useOnce ok", modePrompt, submitPayload{Action: "useOnce", Text: "hello"}, ""},
		{"useOnce empty", modePrompt, submitPayload{Action: "useOnce", Text: "  "}, "Instructions are required."},
		{"useAndSave missing name", modePrompt, submitPayload{Action: "useAndSave", Text: "x"}, "Prompt name is required."},
		{"useAndSave ok", modePrompt, submitPayload{Action: "useAndSave", Text: "x", Name: "n"}, ""},
		{"updateAndSave no loaded", modePrompt, submitPayload{Action: "updateAndSave", Text: "x"}, "No saved prompt is loaded to update."},
		{"updateAndSave ok", modePrompt, submitPayload{Action: "updateAndSave", Text: "x", LoadedName: "saved"}, ""},
		{"unknown action", modePrompt, submitPayload{Action: "weird", Text: "x"}, `Unknown action: "weird".`},
		{"capture needs files", modeCapture, submitPayload{Action: "useOnce", Text: "x"}, "Select at least one file to transcribe."},
		{"capture with files", modeCapture, submitPayload{Action: "useOnce", Text: "x", Files: []string{"a.mp3"}}, ""},
		{"cancel always ok", modeCapture, submitPayload{Action: "cancel"}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			br := newBridge(tc.mode, "", "", nil, nil)
			got := br.validate(tc.payload)
			if got != tc.wantErr {
				t.Fatalf("validate() = %q, want %q", got, tc.wantErr)
			}
		})
	}
}

func TestBridgeToResult(t *testing.T) {
	br := newBridge(modePrompt, "", "", nil, nil)

	r := br.toResult(submitPayload{Action: "useAndSave", Text: " hi ", Name: " foo "})
	if r.result.Text != "hi" || r.result.CacheName != "foo" || r.result.CacheMode != CacheModeCreate || !r.result.Cache {
		t.Fatalf("unexpected useAndSave result: %+v", r.result)
	}

	r = br.toResult(submitPayload{Action: "updateAndSave", Text: "x", LoadedName: "bar"})
	if r.result.CacheName != "bar" || r.result.CacheMode != CacheModeUpdate {
		t.Fatalf("unexpected updateAndSave result: %+v", r.result)
	}

	r = br.toResult(submitPayload{Action: "cancel"})
	if !errors.Is(r.err, ErrCanceled) {
		t.Fatalf("cancel should yield ErrCanceled, got %v", r.err)
	}
}

func TestBridgeHandleSubmitResponse(t *testing.T) {
	// useOnce: empty response, readyToClose fires immediately
	br := newBridge(modePrompt, "", "", nil, nil)
	resp := br.handleSubmit(submitPayload{Action: "useOnce", Text: "hi"})
	if resp.Error != "" || resp.Confirm != "" {
		t.Fatalf("useOnce should return empty response, got %+v", resp)
	}
	select {
	case <-br.ReadyToClose():
	default:
		t.Fatal("ReadyToClose should be closed after useOnce")
	}

	// useAndSave: confirm message returned, readyToClose NOT yet closed
	br2 := newBridge(modePrompt, "", "", nil, nil)
	resp2 := br2.handleSubmit(submitPayload{Action: "useAndSave", Text: "x", Name: "myprompt"})
	if resp2.Confirm == "" {
		t.Fatal("useAndSave should return a confirm message")
	}
	select {
	case <-br2.ReadyToClose():
		t.Fatal("ReadyToClose should not be closed before gtcClose")
	default:
	}
	// simulate JS calling gtcClose after modal dismissed
	br2.handleClose()
	select {
	case <-br2.ReadyToClose():
	default:
		t.Fatal("ReadyToClose should be closed after handleClose")
	}

	// validation error: window stays open
	br3 := newBridge(modePrompt, "", "", nil, nil)
	resp3 := br3.handleSubmit(submitPayload{Action: "useOnce", Text: ""})
	if resp3.Error == "" {
		t.Fatal("empty text should produce an error response")
	}
}

func TestBridgeDeliverOnce(t *testing.T) {
	br := newBridge(modePrompt, "", "", nil, nil)
	br.deliver(submitResult{result: Result{Text: "first"}})
	br.deliver(submitResult{result: Result{Text: "second"}})
	got := br.Result()
	if got.result.Text != "first" {
		t.Fatalf("deliver should be idempotent; got %q", got.result.Text)
	}
	select {
	case <-br.Done():
	default:
		t.Fatal("Done() should be closed after first deliver")
	}
}

func TestAssetsContainEntryFiles(t *testing.T) {
	required := []string{"index.html", "app.js", "styles.css", "vendor/tailwind.js"}
	root := assetsFS()
	for _, name := range required {
		if _, err := fs.Stat(root, name); err != nil {
			t.Errorf("missing embedded asset %q: %v", name, err)
		}
	}
}
