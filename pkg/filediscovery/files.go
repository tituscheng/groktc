package filediscovery

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"unicode/utf8"
)

const sniffBytes = 8192

func DiscoverTextFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		slog.Error("discover text files failed", "dir", dir, "error", err)
		return nil, fmt.Errorf("read directory %q: %w", dir, err)
	}

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		ok, err := IsTextFile(path)
		if err != nil {
			return nil, err
		}
		if ok {
			files = append(files, filepath.Clean(path))
		}
	}

	sort.Strings(files)
	return files, nil
}

func ValidateExplicitFiles(paths []string) ([]string, error) {
	seen := make(map[string]struct{}, len(paths))
	validated := make([]string, 0, len(paths))

	for _, path := range paths {
		cleaned := filepath.Clean(path)
		if _, ok := seen[cleaned]; ok {
			continue
		}

		info, err := os.Stat(cleaned)
		if err != nil {
			if os.IsNotExist(err) {
				slog.Error("file does not exist", "path", cleaned)
				return nil, fmt.Errorf("file %q does not exist", cleaned)
			}
			slog.Error("stat file failed", "path", cleaned, "error", err)
			return nil, fmt.Errorf("stat file %q: %w", cleaned, err)
		}
		if info.IsDir() {
			slog.Error("path is a directory", "path", cleaned)
			return nil, fmt.Errorf("path %q is a directory", cleaned)
		}
		if !info.Mode().IsRegular() {
			slog.Error("path is not a regular file", "path", cleaned)
			return nil, fmt.Errorf("path %q is not a regular file", cleaned)
		}

		ok, err := IsTextFile(cleaned)
		if err != nil {
			slog.Error("text file check failed", "path", cleaned, "error", err)
			return nil, err
		}
		if !ok {
			slog.Error("file is not a text file", "path", cleaned)
			return nil, fmt.Errorf("file %q does not appear to be a text file", cleaned)
		}

		seen[cleaned] = struct{}{}
		validated = append(validated, cleaned)
	}

	sort.Strings(validated)
	return validated, nil
}

func IsTextFile(path string) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, fmt.Errorf("open file %q: %w", path, err)
	}
	defer file.Close()

	buffer := make([]byte, sniffBytes)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return false, fmt.Errorf("read file %q: %w", path, err)
	}

	sample := buffer[:n]
	if len(sample) == 0 {
		return true, nil
	}
	if bytes.IndexByte(sample, 0) >= 0 {
		return false, nil
	}
	if !utf8.Valid(sample) {
		return false, nil
	}

	return true, nil
}
