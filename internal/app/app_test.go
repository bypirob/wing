package app

import (
	"strings"
	"testing"

	"wing/internal/git"
)

func TestToggleModeDoesNotChangeFocus(t *testing.T) {
	m := New(Config{})
	m.focus = focusFiles
	m.mode = modeExplorer

	m.toggleMode()
	if m.mode != modeDiff {
		t.Fatalf("expected mode diff, got %v", m.mode)
	}
	if m.focus != focusFiles {
		t.Fatalf("expected focus to remain files, got %v", m.focus)
	}
}

func TestToggleFocus(t *testing.T) {
	m := New(Config{})
	m.focus = focusFiles
	m.toggleFocus()
	if m.focus != focusDiff {
		t.Fatalf("expected focus diff, got %v", m.focus)
	}
	m.toggleFocus()
	if m.focus != focusFiles {
		t.Fatalf("expected focus files, got %v", m.focus)
	}
}

func TestEnsureSelectionVisible(t *testing.T) {
	m := New(Config{})
	m.height = 20
	m.files = make([]git.StatusEntry, 30)
	m.selected = 25
	m.fileOffset = 0
	m.ensureSelectionVisible()
	if m.fileOffset == 0 {
		t.Fatalf("expected offset to move for selection")
	}
	m.selected = 0
	m.ensureSelectionVisible()
	if m.fileOffset != 0 {
		t.Fatalf("expected offset reset to 0, got %d", m.fileOffset)
	}
}

func TestScrollDiffClamps(t *testing.T) {
	m := New(Config{})
	m.height = 20
	for i := 0; i < 50; i++ {
		m.diffLines = append(m.diffLines, "line")
	}
	m.updateContentLines()
	m.scrollDiff(5)
	if m.diffOffset != 5 {
		t.Fatalf("expected offset 5, got %d", m.diffOffset)
	}
	m.scrollDiff(100)
	if m.diffOffset <= 5 {
		t.Fatalf("expected offset to advance")
	}
	m.scrollDiff(-100)
	if m.diffOffset != 0 {
		t.Fatalf("expected offset clamp to 0, got %d", m.diffOffset)
	}
}

func TestSliceLines(t *testing.T) {
	m := New(Config{})
	lines := []string{"a", "b", "c", "d"}
	out := m.sliceLines(lines, 1, 2)
	if strings.Join(out, ",") != "b,c" {
		t.Fatalf("unexpected slice: %v", out)
	}
}

func TestRenderDiffTitleChangesWithMode(t *testing.T) {
	m := New(Config{})
	m.height = 10
	m.width = 40
	m.mode = modeExplorer
	fileView := m.renderDiff(30, 9)
	if !strings.Contains(fileView, "File") {
		t.Fatalf("expected File title in explorer mode")
	}
	m.mode = modeDiff
	diffView := m.renderDiff(30, 9)
	if !strings.Contains(diffView, "Diff") {
		t.Fatalf("expected Diff title in diff mode")
	}
}
