//go:build darwin

// Package dockicon manages NSApplication activation policy for Go CLI tools
// that open transient webview windows on macOS.
//
// Without explicit cleanup, the webview library activates the app via
// [NSApp activateIgnoringOtherApps:YES] but never deactivates it after the
// window closes. While the rest of the program continues with non-UI work
// (e.g. HTTP calls, file I/O), macOS sees a frontmost app that isn't
// pumping UI events and shows a "not responding" beach-ball cursor.
//
// Hide() sets NSApplicationActivationPolicyAccessory before the window opens.
// Deactivate() removes the app from the foreground after the window closes.
package dockicon

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa

#import <Cocoa/Cocoa.h>

static void groktcHideDockIcon(void) {
    [[NSApplication sharedApplication] setActivationPolicy:NSApplicationActivationPolicyAccessory];
}

static void groktcDeactivateApp(void) {
    [[NSApplication sharedApplication] deactivate];
}
*/
import "C"

// Hide removes the application's dock icon. Safe to call multiple times.
// Must be called before (or just after) the Fyne app is created — calling
// it after the dock icon has been displayed for a while may briefly flash
// the icon, but the steady-state result is the same.
func Hide() {
	C.groktcHideDockIcon()
}

// Deactivate removes the app from the foreground and returns focus to the
// previously active application. Call this after closing a webview window
// to prevent macOS from showing a beach-ball cursor while the program
// continues with non-UI work.
func Deactivate() {
	C.groktcDeactivateApp()
}
