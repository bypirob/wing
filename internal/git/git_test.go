package git

import (
	"os"
	"os/exec"
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

func TestListFilesIncludeIgnored(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init")

	if err := os.WriteFile(filepath.Join(repo, ".gitignore"), []byte("*.log\n"), 0o644); err != nil {
		t.Fatalf("write gitignore: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "keep.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatalf("write keep: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "ignored.log"), []byte("ignore"), 0o644); err != nil {
		t.Fatalf("write ignored: %v", err)
	}
	runGit(t, repo, "add", "keep.txt", ".gitignore")

	entries, err := ListFiles(repo, false)
	if err != nil {
		t.Fatalf("ListFiles error: %v", err)
	}
	if hasPath(entries, "ignored.log") {
		t.Fatalf("expected ignored.log to be excluded without includeIgnored")
	}
	if !hasPath(entries, "keep.txt") {
		t.Fatalf("expected keep.txt to be listed")
	}

	entries, err = ListFiles(repo, true)
	if err != nil {
		t.Fatalf("ListFiles include ignored error: %v", err)
	}
	entry, ok := entryForPath(entries, "ignored.log")
	if !ok {
		t.Fatalf("expected ignored.log to be listed when includeIgnored is true")
	}
	if !entry.Ignored || entry.Status != "!!" {
		t.Fatalf("expected ignored entry, got %+v", entry)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v (%s)", args, err, strings.TrimSpace(string(out)))
	}
}

func hasPath(entries []StatusEntry, path string) bool {
	_, ok := entryForPath(entries, path)
	return ok
}

func entryForPath(entries []StatusEntry, path string) (StatusEntry, bool) {
	for _, entry := range entries {
		if entry.Path == path {
			return entry, true
		}
	}
	return StatusEntry{}, false
}
