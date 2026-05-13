package markdown

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tituscheng/groktc/pkg/fileutil"
)

func ResolveTasks(args []string, force bool) ([]FileTask, error) {
	if len(args) > 1 {
		return nil, fmt.Errorf("markdown accepts at most one filename")
	}
	if len(args) == 1 {
		return resolveSingleTask(args[0], force)
	}
	return discoverBatchTasks(".", force)
}

func resolveSingleTask(path string, force bool) ([]FileTask, error) {
	cleaned := filepath.Clean(path)
	if !strings.EqualFold(filepath.Ext(cleaned), ".txt") {
		return nil, fmt.Errorf("input file %q must be a .txt file", cleaned)
	}

	info, err := os.Stat(cleaned)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("input file %q does not exist", cleaned)
		}
		return nil, fmt.Errorf("stat input file %q: %w", cleaned, err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("input path %q is a directory", cleaned)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("input path %q is not a regular file", cleaned)
	}

	task := taskForTextFile(cleaned)
	if fileutil.ShouldSkipOutput(task.OutputPath, force) {
		task.Skipped = true
		task.SkipReason = "matching non-empty Markdown file already exists"
	}
	return []FileTask{task}, nil
}

func discoverBatchTasks(dir string, force bool) ([]FileTask, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read directory %q: %w", dir, err)
	}

	tasks := make([]FileTask, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") || !entry.Type().IsRegular() || !strings.EqualFold(filepath.Ext(name), ".txt") {
			continue
		}

		task := taskForTextFile(filepath.Join(dir, name))
		if fileutil.ShouldSkipOutput(task.OutputPath, force) {
			task.Skipped = true
			task.SkipReason = "matching non-empty Markdown file already exists"
		}
		tasks = append(tasks, task)
	}

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].InputPath < tasks[j].InputPath
	})
	return tasks, nil
}

func taskForTextFile(path string) FileTask {
	ext := filepath.Ext(path)
	return FileTask{
		InputPath:  filepath.Clean(path),
		OutputPath: filepath.Clean(strings.TrimSuffix(path, ext) + ".md"),
	}
}


