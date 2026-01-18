package git

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestSplitNullPaths(t *testing.T) {
	input := "a\x00b\x00c\x00"
	got := splitNullPaths(input)
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("expected %d entries, got %d", len(want), len(got))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected %q at %d, got %q", want[i], i, got[i])
		}
	}
}

func TestExpandUntrackedDir(t *testing.T) {
	repo := t.TempDir()
	root := filepath.Join(repo, "newdir")
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	files := []string{
		filepath.Join(root, "a.txt"),
		filepath.Join(root, "sub", "b.txt"),
	}
	for _, path := range files {
		if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	entries := expandUntrackedDir(repo, "newdir")
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.Status != "??" {
			t.Fatalf("expected status ??, got %q", entry.Status)
		}
		paths = append(paths, entry.Path)
	}
	sort.Strings(paths)
	want := []string{"newdir/a.txt", "newdir/sub/b.txt"}
	for i := range want {
		if paths[i] != want[i] {
			t.Fatalf("expected %q at %d, got %q", want[i], i, paths[i])
		}
	}
}

func TestFileContentsTrim(t *testing.T) {
	repo := t.TempDir()
	path := filepath.Join(repo, "file.txt")
	content := "hello\nworld\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	got, err := FileContents(repo, "file.txt")
	if err != nil {
		t.Fatalf("FileContents error: %v", err)
	}
	if strings.HasSuffix(got, "\n") {
		t.Fatalf("expected trimmed trailing newline")
	}
	if got != "hello\nworld" {
		t.Fatalf("expected content to match, got %q", got)
	}
}
