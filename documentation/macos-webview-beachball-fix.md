# Fixing macOS Beach-Ball Cursor After Closing a webview_go Window

> **Use case:** You have a Go CLI tool that occasionally opens a webview window (e.g., a settings dialog, a prompt form, a file picker). After the user closes the window and your program continues with non-UI work (HTTP requests, file I/O, etc.), macOS shows a spinning beach-ball cursor. This document explains the root cause and provides a complete, copy-pasteable fix.

---

## Problem Statement

When using `github.com/webview/webview_go` in a Go program on macOS:

1. The program starts from a terminal (not as a bundled `.app`).
2. It opens a webview window for user input.
3. The user interacts with the window and closes it.
4. Control returns to Go code, which continues with background work (e.g., API calls).
5. **macOS shows a spinning beach-ball cursor** over the terminal, as if the program is not responding.

The program is not actually frozen — it is working normally. macOS is incorrectly monitoring the process for UI responsiveness because the process is still registered as the "frontmost app."

---

## Root Cause

### The chain of events

1. **Your code (or you) sets an accessory activation policy** before opening the webview:
   ```objc
   [[NSApplication sharedApplication] setActivationPolicy:NSApplicationActivationPolicyAccessory];
   ```
   This prevents the app from appearing in the Dock. (If you don't do this, the app will appear in the Dock when the window opens.)

2. **The webview library overrides this policy** for non-bundled apps. In the C++ backend (`webview.h`), the `on_application_did_finish_launching` callback does:
   ```objc
   if (!is_app_bundled()) {
       [app setActivationPolicy:NSApplicationActivationPolicyRegular];
       [app activateIgnoringOtherApps:YES];
   }
   ```
   This forcibly activates the app so the window receives focus. This is necessary — without it, the webview window would open unfocused and the user could not interact with it.

3. **The window closes, but the app is never deactivated.** When you call `w.Terminate()` and `w.Run()` returns, the Cocoa run loop stops and the window is destroyed. However, `NSApplication` is still the active (frontmost) app. It has no key window and is no longer processing events.

4. **macOS sees a frontmost app doing background work.** While your Go code makes HTTP calls or writes files, macOS's WindowServer expects the frontmost app to pump UI events. Since it doesn't, macOS displays the spinning cursor to indicate "not responding."

---

## Solution

After `w.Run()` returns, explicitly deactivate the application so macOS returns focus to the previously active app (e.g., Terminal) before your program continues with non-UI work.

### Step 1: Create a platform-specific helper package

Create `internal/dockicon/dock_darwin.go` (macOS only):

```go
//go:build darwin

// Package dockicon manages NSApplication activation policy for Go CLI tools
// that open transient webview windows on macOS.
package dockicon

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa

#import <Cocoa/Cocoa.h>

static void hideDockIcon(void) {
    [[NSApplication sharedApplication] setActivationPolicy:NSApplicationActivationPolicyAccessory];
}

static void deactivateApp(void) {
    [[NSApplication sharedApplication] deactivate];
}
*/
import "C"

// Hide removes the application's dock icon. Call this before opening a
// webview window to prevent the app from appearing in the Dock.
func Hide() {
    C.hideDockIcon()
}

// Deactivate removes the app from the foreground and returns focus to the
// previously active application. Call this immediately after w.Run() returns
// to prevent macOS from showing a beach-ball cursor while the program
// continues with non-UI work.
func Deactivate() {
    C.deactivateApp()
}
```

Create `internal/dockicon/dock_other.go` (Linux/Windows — no-op stubs):

```go
//go:build !darwin

package dockicon

// Hide is a no-op on non-macOS platforms.
func Hide() {}

// Deactivate is a no-op on non-macOS platforms.
func Deactivate() {}
```

### Step 2: Wire the lifecycle into your webview code

The correct lifecycle for a transient webview dialog in a CLI tool:

```go
package yourpackage

import (
    "runtime"

    "yourmodule/internal/dockicon"
    webview "github.com/webview/webview_go"
)

func ShowDialog() {
    // 1. Lock the goroutine to the OS thread. Webview requires this on macOS.
    runtime.LockOSThread()
    defer runtime.UnlockOSThread()

    // 2. Hide the dock icon before the webview overrides the policy.
    dockicon.Hide()

    // 3. Create and configure the webview as usual.
    w := webview.New(false)
    if w == nil {
        panic("failed to create webview")
    }
    defer w.Destroy()

    w.SetTitle("My Dialog")
    w.SetSize(800, 600, webview.HintNone)
    w.Navigate("http://localhost:8080")

    // 4. Run the webview event loop. This blocks until the window closes.
    w.Run()

    // 5. CRITICAL: Deactivate the app after the window closes.
    //    This prevents the beach-ball cursor during subsequent non-UI work.
    dockicon.Deactivate()

    // 6. Continue with background work (HTTP calls, file I/O, etc.).
    //    macOS no longer considers this process the frontmost app.
    doBackgroundWork()
}
```

### Step 3: Ensure the window actually terminates

You need a mechanism to call `w.Terminate()` when the user is done. The standard pattern is a goroutine watcher:

```go
func ShowDialogWithCancel(done chan struct{}) {
    runtime.LockOSThread()
    defer runtime.UnlockOSThread()

    dockicon.Hide()

    w := webview.New(false)
    defer w.Destroy()

    w.SetTitle("My Dialog")
    w.SetSize(800, 600, webview.HintNone)
    w.Navigate("http://localhost:8080")

    // Watch for external cancellation or a "done" signal.
    go func() {
        <-done
        w.Dispatch(w.Terminate)
    }()

    w.Run()
    close(done) // or synchronize appropriately

    dockicon.Deactivate()
}
```

If your JS code calls a bound Go function to signal completion, that function should close the `done` channel, which triggers the goroutine to call `w.Dispatch(w.Terminate)`.

---

## Why `Hide()` alone is not enough

`dockicon.Hide()` (which calls `setActivationPolicy:Accessory`) runs *before* the webview is created. However, the webview library's `on_application_did_finish_launching` callback forcibly overrides this to `Regular` and activates the app. This is hardcoded behavior for non-bundled apps in the C++ backend.

Therefore:
- `Hide()` alone → Dock icon is hidden briefly, then reappears when the webview activates.
- `Deactivate()` alone → App is removed from foreground, but may still show in Dock.
- **Both together** → Dock icon stays hidden, and focus returns to Terminal after close.

---

## Full working example

```go
package main

import (
    "fmt"
    "runtime"

    "yourmodule/internal/dockicon"
    webview "github.com/webview/webview_go"
)

func main() {
    result := runDialog()
    fmt.Println("Result:", result)

    // Simulate background work that would trigger the beach-ball without the fix.
    fmt.Println("Processing...")
    // time.Sleep(5 * time.Second) // or HTTP calls, file I/O, etc.
    fmt.Println("Done.")
}

type dialogResult struct {
    value string
}

func runDialog() dialogResult {
    runtime.LockOSThread()
    defer runtime.UnlockOSThread()

    dockicon.Hide()

    w := webview.New(false)
    if w == nil {
        panic("failed to create webview")
    }
    defer w.Destroy()

    done := make(chan dialogResult, 1)

    w.Bind("submit", func(value string) {
        done <- dialogResult{value: value}
        w.Dispatch(w.Terminate)
    })

    w.Bind("cancel", func() {
        w.Dispatch(w.Terminate)
    })

    w.SetTitle("Example Dialog")
    w.SetSize(600, 400, webview.HintNone)
    w.SetHtml(`<!DOCTYPE html>
<html>
<body>
    <input id="input" type="text" placeholder="Type something...">
    <button onclick="window.submit(document.getElementById('input').value)">Submit</button>
    <button onclick="window.cancel()">Cancel</button>
</body>
</html>`)

    w.Run()

    // CRITICAL: Deactivate before returning to non-UI code.
    dockicon.Deactivate()

    select {
    case res := <-done:
        return res
    default:
        return dialogResult{value: ""} // cancelled
    }
}
```

---

## Platform Notes

| Platform | Behavior | Action needed |
|----------|----------|---------------|
| **macOS** | Beach-ball after close | Use both `Hide()` and `Deactivate()` |
| **Linux (GTK)** | No beach-ball issue | `Hide()` and `Deactivate()` are no-ops |
| **Windows** | No beach-ball issue | `Hide()` and `Deactivate()` are no-ops |

The build tags (`//go:build darwin` vs `//go:build !darwin`) ensure the helper package compiles everywhere without conditional logic at the call site.

---

## Why this is not fixed upstream

The `webview` core library is designed for **traditional desktop applications**, not CLI tools with transient dialogs. Its macOS backend assumes:

1. The app is either a bundled `.app` (launched from Finder/Dock), OR
2. The app is a standalone desktop program that should remain active until quit.

The `applicationShouldTerminateAfterLastWindowClosed:` delegate returns `NO`, and there is no API to configure activation policy or post-close deactivation. There is an open issue about `activateIgnoringOtherApps:` being deprecated ([#1276](https://github.com/webview/webview/issues/1276)), but no issue tracking post-close cleanup because it is outside the library's design scope.

---

## Alternatives considered

| Approach | Pros | Cons |
|----------|------|------|
| **Manual `deactivate` (this fix)** | Minimal, targeted, no dependencies | Requires CGo + Cocoa knowledge |
| **Bundle as `.app`** | Avoids forced activation entirely | Heavyweight for a CLI tool; breaks terminal workflow |
| **Use Lorca (Chrome/CDP)** | No native app lifecycle at all | Requires Chrome installed; different API |
| **Use Wails/Tauri** | Full lifecycle management | Massive dependency increase; designed for full desktop apps |
| **Patch webview C++ core** | Cleanest fix | Requires maintaining a fork |

For Go CLI tools that need occasional webview pop-ups, the manual `deactivate` approach is the most practical.

---

## Quick checklist for applying this fix

- [ ] Create `dockicon` package with `dock_darwin.go` and `dock_other.go`
- [ ] Call `dockicon.Hide()` before creating the webview
- [ ] Call `runtime.LockOSThread()` before webview setup
- [ ] Ensure `w.Terminate()` is called when the dialog should close
- [ ] Call `dockicon.Deactivate()` immediately after `w.Run()` returns
- [ ] Test on macOS by opening the dialog, closing it, and observing the cursor during subsequent work

---

## References

- `github.com/webview/webview_go` — Go bindings for the webview library
- `webview` C++ header: `cocoa_wkwebview_engine::on_application_did_finish_launching()` — sets `NSApplicationActivationPolicyRegular` for non-bundled apps
- Apple Documentation: `-[NSApplication deactivate]`, `NSApplicationActivationPolicy`
