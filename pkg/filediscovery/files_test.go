package filediscovery

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDiscoverTextFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "a.txt"), []byte("hello"))
	mustWriteFile(t, filepath.Join(dir, "b.bin"), []byte{0, 1, 2, 3})
	if err := os.Mkdir(filepath.Join(dir, "nested"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	mustWriteFile(t, filepath.Join(dir, "nested", "c.txt"), []byte("nested"))

	got, err := DiscoverTextFiles(dir)
	if err != nil {
		t.Fatalf("DiscoverTextFiles returned error: %v", err)
	}

	want := []string{filepath.Join(dir, "a.txt")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestValidateExplicitFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	first := filepath.Join(dir, "a.txt")
	second := filepath.Join(dir, "b.txt")
	mustWriteFile(t, first, []byte("hello"))
	mustWriteFile(t, second, []byte("world"))

	got, err := ValidateExplicitFiles([]string{second, first, first})
	if err != nil {
		t.Fatalf("ValidateExplicitFiles returned error: %v", err)
	}

	want := []string{first, second}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestValidateExplicitFilesRejectsBinary(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "a.bin")
	mustWriteFile(t, path, []byte{0, 1, 2, 3})

	_, err := ValidateExplicitFiles([]string{path})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateExplicitFilesRejectsDirectory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	_, err := ValidateExplicitFiles([]string{dir})
	if err == nil {
		t.Fatal("expected error")
	}
}

func mustWriteFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write file %q: %v", path, err)
	}
}
