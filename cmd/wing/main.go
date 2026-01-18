package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"wing/internal/app"
)

var version = "dev"

func main() {
	repoPath := flag.String("repo", ".", "path to the git repo")
	refresh := flag.Duration("refresh", 2*time.Second, "refresh interval")
	theme := flag.String("theme", "default", "color theme (stub)")
	showVersion := flag.Bool("version", false, "print version")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}

	model := app.New(app.Config{
		RepoPath:      *repoPath,
		RefreshPeriod: *refresh,
		Theme:         *theme,
	})

	program := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
