package estimate

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tituscheng/groktc/internal/transcribe"
)

type MediaFile struct {
	Path string
	Kind transcribe.InputKind
}

func ResolveFiles(args []string) ([]MediaFile, error) {
	if len(args) == 0 {
		return discoverMediaFiles(".")
	}
	return resolveExplicitFiles(args)
}

func discoverMediaFiles(dir string) ([]MediaFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read directory %q: %w", dir, err)
	}

	files := make([]MediaFile, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") || !entry.Type().IsRegular() {
			continue
		}
		kind, ok := kindForExt(filepath.Ext(name))
		if !ok {
			continue
		}
		files = append(files, MediaFile{
			Path: filepath.Join(dir, name),
			Kind: kind,
		})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})
	return files, nil
}

func resolveExplicitFiles(args []string) ([]MediaFile, error) {
	seen := make(map[string]struct{}, len(args))
	files := make([]MediaFile, 0, len(args))

	for _, arg := range args {
		cleaned := filepath.Clean(arg)
		if _, ok := seen[cleaned]; ok {
			continue
		}
		seen[cleaned] = struct{}{}

		kind, ok := kindForExt(filepath.Ext(cleaned))
		if !ok {
			return nil, fmt.Errorf("unsupported input %q — supported: .mp3, .mp4", filepath.Ext(cleaned))
		}

		info, err := os.Stat(cleaned)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("file %q does not exist", cleaned)
			}
			return nil, fmt.Errorf("stat %q: %w", cleaned, err)
		}
		if info.IsDir() {
			return nil, fmt.Errorf("path %q is a directory", cleaned)
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("path %q is not a regular file", cleaned)
		}

		files = append(files, MediaFile{Path: cleaned, Kind: kind})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})
	return files, nil
}

func kindForExt(ext string) (transcribe.InputKind, bool) {
	switch strings.ToLower(ext) {
	case ".mp3":
		return transcribe.KindMP3, true
	case ".mp4":
		return transcribe.KindMP4, true
	default:
		return 0, false
	}
}
