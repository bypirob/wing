package app

import (
	"testing"

	"wing/internal/git"
)

func TestBuildRowsCollapsedDefault(t *testing.T) {
	files := []git.StatusEntry{
		{Path: "docs/a.txt"},
		{Path: "docs/sub/b.txt"},
		{Path: "root.txt"},
	}
	collapsed := map[string]bool{}
	rows := buildRows(files, collapsed)
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if !rows[0].IsDir || rows[0].Path != "docs" {
		t.Fatalf("expected docs folder row first, got %+v", rows[0])
	}
	if !rows[0].Collapsed {
		t.Fatalf("expected docs to be collapsed by default")
	}
	if rows[1].IsDir || rows[1].Path != "root.txt" {
		t.Fatalf("expected root.txt file row, got %+v", rows[1])
	}
}

func TestBuildRowsExpanded(t *testing.T) {
	files := []git.StatusEntry{
		{Path: "docs/a.txt"},
		{Path: "docs/sub/b.txt"},
		{Path: "root.txt"},
	}
	collapsed := map[string]bool{
		"docs":     false,
		"docs/sub": false,
	}
	rows := buildRows(files, collapsed)
	if len(rows) != 5 {
		t.Fatalf("expected 5 rows, got %d", len(rows))
	}
	want := []struct {
		path string
		isDir bool
	}{
		{path: "docs", isDir: true},
		{path: "docs/a.txt", isDir: false},
		{path: "docs/sub", isDir: true},
		{path: "docs/sub/b.txt", isDir: false},
		{path: "root.txt", isDir: false},
	}
	for i, w := range want {
		if rows[i].Path != w.path || rows[i].IsDir != w.isDir {
			t.Fatalf("row %d expected %v, got %+v", i, w, rows[i])
		}
	}
}

func TestBuildRowsIgnoredFolder(t *testing.T) {
	files := []git.StatusEntry{
		{Path: "docs/a.txt", Ignored: true},
		{Path: "docs/b.txt", Ignored: true},
		{Path: "src/main.go"},
	}
	rows := buildRows(files, map[string]bool{})
	var docsIgnored *bool
	var srcIgnored *bool
	for _, row := range rows {
		if row.IsDir && row.Path == "docs" {
			v := row.Ignored
			docsIgnored = &v
		}
		if row.IsDir && row.Path == "src" {
			v := row.Ignored
			srcIgnored = &v
		}
	}
	if docsIgnored == nil || !*docsIgnored {
		t.Fatalf("expected docs folder to be ignored")
	}
	if srcIgnored == nil || *srcIgnored {
		t.Fatalf("expected src folder to not be ignored")
	}
}
