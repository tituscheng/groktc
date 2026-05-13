//go:build !darwin

package webui

// installPlatformMenus is a no-op on platforms other than macOS — Linux
// (WebKitGTK) and Windows (WebView2) wire up clipboard shortcuts for free.
func installPlatformMenus() {}
