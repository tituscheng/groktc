package transcribe

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tituscheng/groktc/pkg/fileutil"
)

func ResolveTasks(args []string, force bool, outputPath string) ([]FileTask, error) {
	if len(args) == 0 {
		return discoverMediaTasks(".", force)
	}
	if outputPath != "" && len(args) > 1 {
		return nil, fmt.Errorf("--output can only be used with a single input")
	}
	return resolveExplicitTasks(args, force, outputPath)
}

func resolveExplicitTasks(args []string, force bool, outputPath string) ([]FileTask, error) {
	seen := make(map[string]struct{}, len(args))
	tasks := make([]FileTask, 0, len(args))

	for _, arg := range args {
		// Check URL before filepath.Clean, which corrupts schemes like https://.
		if isURL(arg) {
			if _, ok := seen[arg]; ok {
				continue
			}
			seen[arg] = struct{}{}
			task := taskForURL(arg, outputPath)
			if fileutil.ShouldSkipOutput(task.OutputPath, force) {
				task.Skipped = true
				task.SkipReason = "matching non-empty transcript file already exists"
			}
			tasks = append(tasks, task)
			continue
		}

		cleaned := filepath.Clean(arg)
		if _, ok := seen[cleaned]; ok {
			continue
		}
		seen[cleaned] = struct{}{}

		task, err := resolveSingleTask(cleaned, force, outputPath)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

func resolveSingleTask(path string, force bool, outputPath string) (FileTask, error) {
	kind, ok := kindForExt(filepath.Ext(path))
	if !ok {
		return FileTask{}, fmt.Errorf("unsupported input %q — supported: .mp4, .mp3, or a URL", filepath.Ext(path))
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return FileTask{}, fmt.Errorf("file %q does not exist", path)
		}
		return FileTask{}, fmt.Errorf("stat %q: %w", path, err)
	}
	if info.IsDir() {
		return FileTask{}, fmt.Errorf("path %q is a directory", path)
	}
	if !info.Mode().IsRegular() {
		return FileTask{}, fmt.Errorf("path %q is not a regular file", path)
	}

	task := taskForMediaFile(path, kind, outputPath)
	if fileutil.ShouldSkipOutput(task.OutputPath, force) {
		task.Skipped = true
		task.SkipReason = "matching non-empty transcript file already exists"
	}
	return task, nil
}

func discoverMediaTasks(dir string, force bool) ([]FileTask, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read directory %q: %w", dir, err)
	}

	tasks := make([]FileTask, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") || !entry.Type().IsRegular() {
			continue
		}
		kind, ok := kindForExt(filepath.Ext(name))
		if !ok {
			continue
		}
		task := taskForMediaFile(filepath.Join(dir, name), kind, "")
		if fileutil.ShouldSkipOutput(task.OutputPath, force) {
			task.Skipped = true
			task.SkipReason = "matching non-empty transcript file already exists"
		}
		tasks = append(tasks, task)
	}

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].InputPath < tasks[j].InputPath
	})
	return tasks, nil
}

func DiscoverAvailableFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read directory %q: %w", dir, err)
	}

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") || !entry.Type().IsRegular() {
			continue
		}
		if _, ok := kindForExt(filepath.Ext(name)); !ok {
			continue
		}
		files = append(files, filepath.Join(dir, name))
	}
	sort.Strings(files)
	return files, nil
}

func BuildTasksFromPaths(paths []string, force bool) ([]FileTask, error) {
	tasks := make([]FileTask, 0, len(paths))
	for _, p := range paths {
		task, err := resolveSingleTask(p, force, "")
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

func taskForMediaFile(path string, kind InputKind, outputPath string) FileTask {
	if outputPath != "" {
		return FileTask{
			InputPath:  filepath.Clean(path),
			OutputPath: filepath.Clean(outputPath),
			Kind:       kind,
		}
	}
	ext := filepath.Ext(path)
	return FileTask{
		InputPath:  filepath.Clean(path),
		OutputPath: filepath.Clean(strings.TrimSuffix(path, ext) + ".txt"),
		Kind:       kind,
	}
}

func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

func taskForURL(url string, outputPath string) FileTask {
	if outputPath != "" {
		return FileTask{
			InputPath:  url,
			OutputPath: filepath.Clean(outputPath),
			Kind:       KindURL,
		}
	}
	return FileTask{
		InputPath:  url,
		OutputPath: filepath.Clean(urlSlug(url) + ".txt"),
		Kind:       KindURL,
	}
}

func urlSlug(url string) string {
	// Drop scheme and take the last meaningful path segment.
	s := strings.TrimPrefix(url, "https://")
	s = strings.TrimPrefix(s, "http://")
	s = strings.Trim(s, "/")
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, ":", "_")
	s = strings.ReplaceAll(s, "?", "_")
	s = strings.ReplaceAll(s, "=", "_")
	s = strings.ReplaceAll(s, "&", "_")
	// Limit length so we don't hit filename limits.
	if len(s) > 80 {
		s = s[:80]
	}
	return s
}

func sanitizeFilename(name string) string {
	// Replace characters that are invalid or problematic on common filesystems.
	invalid := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	for _, ch := range invalid {
		name = strings.ReplaceAll(name, ch, "_")
	}
	name = strings.TrimSpace(name)
	// Collapse multiple spaces/underscores.
	for strings.Contains(name, "  ") {
		name = strings.ReplaceAll(name, "  ", " ")
	}
	for strings.Contains(name, "__") {
		name = strings.ReplaceAll(name, "__", "_")
	}
	return name
}

func kindForExt(ext string) (InputKind, bool) {
	switch strings.ToLower(ext) {
	case ".mp4":
		return KindMP4, true
	case ".mp3":
		return KindMP3, true
	default:
		return 0, false
	}
}
