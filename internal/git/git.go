package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type StatusEntry struct {
	Path   string
	Status string
}

func Status(repoPath string) ([]StatusEntry, error) {
	out, err := run(repoPath, "status", "--porcelain")
	if err != nil {
		return nil, err
	}

	basePath := repoPath
	if !filepath.IsAbs(basePath) {
		absBase, err := filepath.Abs(basePath)
		if err == nil {
			basePath = absBase
		}
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	entries := make([]StatusEntry, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if len(line) < 3 {
			continue
		}
		status := strings.TrimSpace(line[:2])
		path := strings.TrimSpace(line[2:])
		if idx := strings.LastIndex(path, " -> "); idx != -1 {
			path = strings.TrimSpace(path[idx+4:])
		}
		if status == "??" {
			fullPath := path
			if !filepath.IsAbs(fullPath) {
				fullPath = filepath.Join(basePath, fullPath)
			}
			info, err := os.Stat(fullPath)
			if err == nil && info.IsDir() {
				entries = append(entries, expandUntrackedDir(basePath, path)...)
				continue
			}
		}
		entries = append(entries, StatusEntry{Path: path, Status: status})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Path < entries[j].Path
	})

	return entries, nil
}

func ListFiles(repoPath string) ([]StatusEntry, error) {
	tracked, err := run(repoPath, "ls-files", "-z")
	if err != nil {
		return nil, err
	}
	untracked, err := run(repoPath, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, err
	}

	entries := make([]StatusEntry, 0)
	seen := make(map[string]struct{})

	for _, path := range splitNullPaths(tracked) {
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		entries = append(entries, StatusEntry{Path: path, Status: ""})
	}

	for _, path := range splitNullPaths(untracked) {
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		entries = append(entries, StatusEntry{Path: path, Status: "??"})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Path < entries[j].Path
	})

	return entries, nil
}

func Diff(repoPath, path, status string) (string, error) {
	if path == "" {
		return "", nil
	}

	args := []string{"diff", "--no-color", "--", path}
	if status == "??" {
		targetPath := path
		if !filepath.IsAbs(targetPath) {
			basePath := repoPath
			if !filepath.IsAbs(basePath) {
				absBase, err := filepath.Abs(basePath)
				if err == nil {
					basePath = absBase
				}
			}
			targetPath = filepath.Join(basePath, targetPath)
		}
		targetPath = filepath.Clean(targetPath)
		if _, err := os.Stat(targetPath); err != nil {
			return "", fmt.Errorf("untracked file not found: %s", targetPath)
		}
		args = []string{"diff", "--no-color", "--no-index", "--", "/dev/null", targetPath}
	}

	out, err := run(repoPath, args...)
	if err != nil {
		return "", err
	}

	return strings.TrimRight(out, "\n"), nil
}

func FileContents(repoPath, path string) (string, error) {
	if path == "" {
		return "", nil
	}
	targetPath := path
	if !filepath.IsAbs(targetPath) {
		basePath := repoPath
		if !filepath.IsAbs(basePath) {
			absBase, err := filepath.Abs(basePath)
			if err == nil {
				basePath = absBase
			}
		}
		targetPath = filepath.Join(basePath, targetPath)
	}
	targetPath = filepath.Clean(targetPath)
	data, err := os.ReadFile(targetPath)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(data), "\n"), nil
}

func Commit(repoPath, message string) error {
	if strings.TrimSpace(message) == "" {
		return fmt.Errorf("commit message is required")
	}
	if _, err := run(repoPath, "add", "-A"); err != nil {
		return err
	}
	_, err := run(repoPath, "commit", "-m", message)
	return err
}

func Push(repoPath string) error {
	_, err := run(repoPath, "push")
	return err
}

func Branch(repoPath string) (string, error) {
	out, err := run(repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func run(repoPath string, args ...string) (string, error) {
	base := append([]string{"-C", repoPath}, args...)
	cmd := exec.Command("git", base...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			if stdout.Len() > 0 {
				return stdout.String(), nil
			}
		}
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

func splitNullPaths(out string) []string {
	return strings.Split(strings.TrimRight(out, "\x00"), "\x00")
}

func expandUntrackedDir(repoPath, relPath string) []StatusEntry {
	fullPath := relPath
	if !filepath.IsAbs(fullPath) {
		fullPath = filepath.Join(repoPath, relPath)
	}
	entries := []StatusEntry{}
	_ = filepath.WalkDir(fullPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(repoPath, path)
		if err != nil {
			return nil
		}
		entries = append(entries, StatusEntry{Path: filepath.ToSlash(rel), Status: "??"})
		return nil
	})
	return entries
}
