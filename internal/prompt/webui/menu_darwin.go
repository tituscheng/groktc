//go:build darwin

package webui

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa

#import <Cocoa/Cocoa.h>

// installEditMenu adds a minimal application menu containing the standard
// Edit submenu. Without this, macOS has nothing bound to the cut:/copy:/paste:
// selectors, so Cmd-X/C/V never reach the WKWebView's first responder.
//
// Key equivalents on NSMenuItems are honored even when the app is running
// under NSApplicationActivationPolicyAccessory (dock icon hidden), so this
// pairs cleanly with internal/dockicon.
static void groktcInstallEditMenu(void) {
    NSApplication *app = [NSApplication sharedApplication];

    NSMenu *mainMenu = [app mainMenu];
    if (mainMenu == nil) {
        mainMenu = [[NSMenu alloc] init];
        [app setMainMenu:mainMenu];
    }

    // Application sub-menu is required for the rest of the menu bar to be
    // wired up correctly. We only need Quit so Cmd-Q still works.
    NSMenuItem *appItem = [[NSMenuItem alloc] init];
    [mainMenu addItem:appItem];
    NSMenu *appMenu = [[NSMenu alloc] init];
    [appItem setSubmenu:appMenu];
    [appMenu addItemWithTitle:@"Quit"
                       action:@selector(terminate:)
                keyEquivalent:@"q"];

    // Edit menu — the actual reason we're here.
    NSMenuItem *editItem = [[NSMenuItem alloc] init];
    [mainMenu addItem:editItem];
    NSMenu *editMenu = [[NSMenu alloc] initWithTitle:@"Edit"];
    [editItem setSubmenu:editMenu];

    [editMenu addItemWithTitle:@"Undo"
                        action:@selector(undo:)
                 keyEquivalent:@"z"];
    NSMenuItem *redo = [editMenu addItemWithTitle:@"Redo"
                                           action:@selector(redo:)
                                    keyEquivalent:@"z"];
    [redo setKeyEquivalentModifierMask:(NSEventModifierFlagCommand | NSEventModifierFlagShift)];
    [editMenu addItem:[NSMenuItem separatorItem]];
    [editMenu addItemWithTitle:@"Cut"
                        action:@selector(cut:)
                 keyEquivalent:@"x"];
    [editMenu addItemWithTitle:@"Copy"
                        action:@selector(copy:)
                 keyEquivalent:@"c"];
    [editMenu addItemWithTitle:@"Paste"
                        action:@selector(paste:)
                 keyEquivalent:@"v"];
    [editMenu addItemWithTitle:@"Select All"
                        action:@selector(selectAll:)
                 keyEquivalent:@"a"];
}
*/
import "C"

import "sync"

var installEditMenuOnce sync.Once

func installPlatformMenus() {
	installEditMenuOnce.Do(func() {
		C.groktcInstallEditMenu()
	})
}
