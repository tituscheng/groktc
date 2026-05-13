// Package fileutil provides small file-system helpers reused across commands.
package fileutil

import "os"

// ShouldSkipOutput reports whether an existing non-empty file at path should be
// skipped. When force is true, it never skips.
func ShouldSkipOutput(path string, force bool) bool {
	if force {
		return false
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Size() > 0
}
