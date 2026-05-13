//go:build !darwin

package dockicon

// Hide is a no-op on non-macOS platforms.
func Hide() {}
