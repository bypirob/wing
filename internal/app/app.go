package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"wing/internal/git"
)

type Config struct {
	RepoPath      string
	RefreshPeriod time.Duration
	Theme         string
}

type Model struct {
	config     Config
	width      int
	height     int
	files      []git.StatusEntry
	diff       string
	diffLines  []string
	contentLines []string
	err          error
	selected   int
	fileOffset int
	diffOffset int
	focus      paneFocus
	modal      modalState
	modalErr   string
	commitText textinput.Model
	mode       viewMode
	gitInfo    string
}

func New(config Config) Model {
	input := textinput.New()
	input.Placeholder = "Commit message"
	input.CharLimit = 120
	input.Width = 44
	return Model{
		config:     config,
		focus:      focusFiles,
		commitText: input,
		mode:       modeExplorer,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.refreshCmd(), m.tickCmd())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateContentLines()
	case refreshMsg:
		m.files = msg.files
		m.diff = msg.diff
		m.diffLines = splitLines(msg.diff)
		m.updateContentLines()
		m.err = msg.err
		m.gitInfo = msg.gitInfo
		m.selected = m.indexForPath(msg.selectedPath)
		m.fileOffset = clampOffset(m.fileOffset, len(m.files), m.filesVisibleHeight())
		m.ensureSelectionVisible()
	case diffMsg:
		m.diff = msg.diff
		m.diffLines = splitLines(msg.diff)
		m.updateContentLines()
		m.err = msg.err
	case tea.KeyMsg:
		if m.modal != modalNone {
			return m.handleModalKey(msg)
		}
		switch msg.String() {
		case "tab", "shift+tab":
			m.toggleFocus()
		case "m":
			m.toggleMode()
			return m, m.refreshCmd()
		case "up", "k":
			if m.focus == focusFiles {
				m.moveSelection(-1)
				return m, m.diffCmd()
			}
			m.scrollDiff(-1)
		case "down", "j":
			if m.focus == focusFiles {
				m.moveSelection(1)
				return m, m.diffCmd()
			}
			m.scrollDiff(1)
		case "pgup":
			if m.focus == focusFiles {
				m.moveSelection(-m.filesVisibleHeight())
				return m, m.diffCmd()
			}
			m.scrollDiff(-m.diffVisibleHeight())
		case "pgdown":
			if m.focus == focusFiles {
				m.moveSelection(m.filesVisibleHeight())
				return m, m.diffCmd()
			}
			m.scrollDiff(m.diffVisibleHeight())
		case "enter":
			m.openCommitModal()
		case "h":
			m.openHelpModal()
	case "q", "esc", "ctrl+c":
		return m, tea.Quit
	}
	case tickMsg:
		return m, tea.Batch(m.refreshCmd(), m.tickCmd())
	case commitMsg:
		if msg.err != nil {
			m.modal = modalCommit
			m.modalErr = msg.err.Error()
		} else {
			m.modal = modalPush
			m.modalErr = ""
		}
	case pushMsg:
		if msg.err != nil {
			m.modal = modalPush
			m.modalErr = msg.err.Error()
		} else {
			m.closeModal()
			return m, m.refreshCmd()
		}
	}

	return m, nil
}

func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	if m.modal != modalNone {
		return m.renderModal()
	}

	paneHeight := m.paneHeight()
	leftWidth, rightWidth := m.paneWidths()

	left := m.renderFiles(leftWidth, paneHeight)
	right := m.renderDiff(rightWidth, paneHeight)
	main := lipgloss.JoinHorizontal(lipgloss.Top, left, right)

	status := m.renderStatusBar()
	return lipgloss.JoinVertical(lipgloss.Top, main, status)
}

func (m Model) renderFiles(width, height int) string {
	borderColor := lipgloss.Color("240")
	titleStyle := lipgloss.NewStyle().Bold(true)
	if m.focus == focusFiles {
		borderColor = lipgloss.Color("62")
		titleStyle = titleStyle.Foreground(lipgloss.Color("62"))
	}
	style := lipgloss.NewStyle().
		Width(width).
		Height(height).
		Padding(1, 1).
		Border(lipgloss.NormalBorder()).
		BorderForeground(borderColor)

	title := titleStyle.Render("Files")
	items := make([]string, 0, len(m.files)+1)
	if len(m.files) == 0 {
		if m.mode == modeExplorer {
			items = append(items, "No files found.")
		} else {
			items = append(items, "No changes detected.")
		}
	} else {
		for i, entry := range m.files {
			statusText := fmt.Sprintf("%-2s", entry.Status)
			statusColor := statusColor(entry.Status)
			if i == m.selected {
				bg := lipgloss.Color("62")
				if m.focus != focusFiles {
					bg = lipgloss.Color("238")
				}
				selectedStyle := lipgloss.NewStyle().Background(bg)
				if statusColor != "" {
					statusText = lipgloss.NewStyle().Foreground(statusColor).Render(statusText)
				}
				pathPart := selectedStyle.Render(entry.Path)
				items = append(items, fmt.Sprintf("%s %s", statusText, pathPart))
				continue
			}

			if statusColor != "" {
				statusText = lipgloss.NewStyle().Foreground(statusColor).Render(statusText)
			}
			line := fmt.Sprintf("%s %s", statusText, entry.Path)
			items = append(items, line)
		}
	}

	body := strings.Join(m.sliceLines(items, m.fileOffset, m.filesVisibleHeight()), "\n")
	return style.Render(fmt.Sprintf("%s\n\n%s", title, body))
}

func (m Model) renderDiff(width, height int) string {
	borderColor := lipgloss.Color("240")
	titleStyle := lipgloss.NewStyle().Bold(true)
	if m.focus == focusDiff {
		borderColor = lipgloss.Color("62")
		titleStyle = titleStyle.Foreground(lipgloss.Color("62"))
	}
	style := lipgloss.NewStyle().
		Width(width).
		Height(height).
		Padding(1, 1).
		Border(lipgloss.NormalBorder()).
		BorderForeground(borderColor)

	titleLabel := "Diff"
	if m.mode == modeExplorer {
		titleLabel = "File"
	}
	title := titleStyle.Render(titleLabel)
	body := ""
	if m.err != nil {
		body = fmt.Sprintf("Error: %s", m.err)
	} else if len(m.diffLines) == 0 {
		if _, ok := m.selectedEntry(); ok {
			if m.mode == modeExplorer {
				body = "No file content to display."
			} else {
				body = "No diff for selected file."
			}
		} else {
			if m.mode == modeExplorer {
				body = "Select a file to view its contents."
			} else {
				body = "Select a file to view its diff."
			}
		}
	} else {
		lines := m.sliceLines(m.contentLines, m.diffOffset, m.diffVisibleHeight())
		if m.mode == modeDiff {
			lines = colorizeDiffLines(lines)
		}
		body = strings.Join(lines, "\n")
	}

	return style.Render(fmt.Sprintf("%s\n\n%s", title, body))
}

type refreshMsg struct {
	files        []git.StatusEntry
	diff         string
	err          error
	selectedPath string
	gitInfo      string
}

type diffMsg struct {
	diff string
	err  error
}

type tickMsg struct{}

type commitMsg struct {
	err error
}

type pushMsg struct {
	err error
}

func (m Model) refreshCmd() tea.Cmd {
	keepPath := m.selectedPath()
	mode := m.mode
	return func() tea.Msg {
		var (
			files    []git.StatusEntry
			statuses []git.StatusEntry
			err      error
		)
		if mode == modeExplorer {
			files, err = git.ListFiles(m.config.RepoPath)
			if err == nil {
				var statusErr error
				statuses, statusErr = git.Status(m.config.RepoPath)
				if statusErr == nil {
					files = applyStatuses(files, statuses)
				}
			}
		} else {
			files, err = git.Status(m.config.RepoPath)
			statuses = files
		}
		if err != nil {
			return refreshMsg{files: nil, diff: "", err: err}
		}

		gitInfo := buildGitInfo(m.config.RepoPath, statuses)
		selectedPath := keepPath
		selectedStatus := ""
		found := false
		if selectedPath == "" && len(files) > 0 {
			selectedPath = files[0].Path
			selectedStatus = files[0].Status
			found = true
		} else {
			for _, entry := range files {
				if entry.Path == selectedPath {
					selectedStatus = entry.Status
					found = true
					break
				}
			}
			if !found && len(files) > 0 {
				selectedPath = files[0].Path
				selectedStatus = files[0].Status
			}
		}

		var diff string
		var diffErr error
		if mode == modeExplorer {
			diff, diffErr = git.FileContents(m.config.RepoPath, selectedPath)
		} else {
			diff, diffErr = git.Diff(m.config.RepoPath, selectedPath, selectedStatus)
		}
		if diffErr != nil {
			err = diffErr
		}

		return refreshMsg{files: files, diff: diff, err: err, selectedPath: selectedPath, gitInfo: gitInfo}
	}
}

func (m Model) diffCmd() tea.Cmd {
	entry, ok := m.selectedEntry()
	if !ok {
		return nil
	}
	m.diffOffset = 0
	mode := m.mode
	return func() tea.Msg {
		var (
			diff string
			err  error
		)
		if mode == modeExplorer {
			diff, err = git.FileContents(m.config.RepoPath, entry.Path)
		} else {
			diff, err = git.Diff(m.config.RepoPath, entry.Path, entry.Status)
		}
		return diffMsg{diff: diff, err: err}
	}
}

func (m Model) commitCmd(message string) tea.Cmd {
	return func() tea.Msg {
		err := git.Commit(m.config.RepoPath, message)
		return commitMsg{err: err}
	}
}

func (m Model) pushCmd() tea.Cmd {
	return func() tea.Msg {
		err := git.Push(m.config.RepoPath)
		return pushMsg{err: err}
	}
}

func (m Model) tickCmd() tea.Cmd {
	if m.config.RefreshPeriod <= 0 {
		return nil
	}
	return tea.Tick(m.config.RefreshPeriod, func(time.Time) tea.Msg {
		return tickMsg{}
	})
}

func (m *Model) moveSelection(delta int) {
	if len(m.files) == 0 {
		m.selected = 0
		m.fileOffset = 0
		return
	}
	next := m.selected + delta
	if next < 0 {
		next = 0
	}
	if next >= len(m.files) {
		next = len(m.files) - 1
	}
	m.selected = next
	m.ensureSelectionVisible()
}

func (m Model) selectedEntry() (git.StatusEntry, bool) {
	if len(m.files) == 0 || m.selected < 0 || m.selected >= len(m.files) {
		return git.StatusEntry{}, false
	}
	return m.files[m.selected], true
}

func (m Model) selectedPath() string {
	entry, ok := m.selectedEntry()
	if !ok {
		return ""
	}
	return entry.Path
}

func (m Model) indexForPath(path string) int {
	if path == "" || len(m.files) == 0 {
		return 0
	}
	for i, entry := range m.files {
		if entry.Path == path {
			return i
		}
	}
	return 0
}

type paneFocus int

const (
	focusFiles paneFocus = iota
	focusDiff
)

type viewMode int

const (
	modeExplorer viewMode = iota
	modeDiff
)

type modalState int

const (
	modalNone modalState = iota
	modalCommit
	modalPush
	modalHelp
)

func (m Model) filesVisibleHeight() int {
	return contentHeight(m.paneHeight())
}

func (m Model) diffVisibleHeight() int {
	return contentHeight(m.paneHeight())
}

func contentHeight(total int) int {
	height := total - 6
	if height < 0 {
		return 0
	}
	return height
}

func (m *Model) ensureSelectionVisible() {
	visible := m.filesVisibleHeight()
	if visible <= 0 {
		m.fileOffset = 0
		return
	}
	if m.selected < m.fileOffset {
		m.fileOffset = m.selected
		return
	}
	if m.selected >= m.fileOffset+visible {
		m.fileOffset = m.selected - visible + 1
	}
	m.fileOffset = clampOffset(m.fileOffset, len(m.files), visible)
}

func (m *Model) scrollDiff(delta int) {
	visible := m.diffVisibleHeight()
	if visible <= 0 {
		m.diffOffset = 0
		return
	}
	m.diffOffset = clampOffset(m.diffOffset+delta, len(m.contentLines), visible)
}

func clampOffset(offset, total, visible int) int {
	if total <= visible {
		return 0
	}
	max := total - visible
	if offset < 0 {
		return 0
	}
	if offset > max {
		return max
	}
	return offset
}

func splitLines(text string) []string {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	return strings.Split(strings.TrimRight(text, "\n"), "\n")
}

func (m Model) sliceLines(lines []string, offset, visible int) []string {
	if visible <= 0 || len(lines) == 0 {
		return []string{}
	}
	start := offset
	if start < 0 {
		start = 0
	}
	if start >= len(lines) {
		start = len(lines) - 1
	}
	end := start + visible
	if end > len(lines) {
		end = len(lines)
	}
	return lines[start:end]
}

func (m Model) paneHeight() int {
	if m.height <= 1 {
		return 0
	}
	return m.height - 1
}

func (m *Model) updateContentLines() {
	if len(m.diffLines) == 0 {
		m.contentLines = nil
		m.diffOffset = 0
		return
	}
	if m.mode == modeExplorer {
		width := m.diffContentWidth()
		m.contentLines = wrapLines(m.diffLines, width)
	} else {
		m.contentLines = append([]string(nil), m.diffLines...)
	}
	m.diffOffset = clampOffset(m.diffOffset, len(m.contentLines), m.diffVisibleHeight())
}

func (m Model) diffContentWidth() int {
	_, right := m.paneWidths()
	width := right - 4
	if width < 1 {
		return 1
	}
	return width
}

func (m Model) paneWidths() (int, int) {
	leftWidth := m.width / 3
	if leftWidth < 24 {
		leftWidth = 24
	}
	if leftWidth > m.width-20 {
		leftWidth = m.width / 2
	}
	rightWidth := m.width - leftWidth - 1
	if rightWidth < 20 {
		rightWidth = 20
	}
	return leftWidth, rightWidth
}

func wrapLines(lines []string, width int) []string {
	if width < 1 || len(lines) == 0 {
		return lines
	}
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			out = append(out, "")
			continue
		}
		runes := []rune(line)
		for len(runes) > width {
			out = append(out, string(runes[:width]))
			runes = runes[width:]
		}
		out = append(out, string(runes))
	}
	return out
}

func (m *Model) toggleMode() {
	if m.mode == modeExplorer {
		m.mode = modeDiff
	} else {
		m.mode = modeExplorer
	}
}

func (m *Model) toggleFocus() {
	if m.focus == focusFiles {
		m.focus = focusDiff
	} else {
		m.focus = focusFiles
	}
}

func (m Model) renderStatusBar() string {
	modeLabel := "Explorer"
	if m.mode == modeDiff {
		modeLabel = "Diff"
	}
	gitInfo := m.gitInfo
	if gitInfo == "" {
		gitInfo = "git: -"
	}
	status := fmt.Sprintf("Mode: %s  |  %s  |  h for help", modeLabel, gitInfo)
	style := lipgloss.NewStyle().
		Width(m.width).
		Height(1).
		Padding(0, 1).
		Background(lipgloss.Color("236")).
		Foreground(lipgloss.Color("250"))
	return style.Render(status)
}

func statusColor(status string) lipgloss.Color {
	status = strings.TrimSpace(status)
	if status == "" {
		return ""
	}
	switch {
	case strings.HasPrefix(status, "??"):
		return lipgloss.Color("178")
	case strings.Contains(status, "D"):
		return lipgloss.Color("160")
	case strings.Contains(status, "A"):
		return lipgloss.Color("71")
	case strings.Contains(status, "M"):
		return lipgloss.Color("214")
	case strings.Contains(status, "R"):
		return lipgloss.Color("69")
	default:
		return lipgloss.Color("111")
	}
}

func buildGitInfo(repoPath string, statuses []git.StatusEntry) string {
	branch, err := git.Branch(repoPath)
	if err != nil || branch == "" {
		branch = "-"
	}

	counts := map[string]int{
		"M": 0,
		"A": 0,
		"D": 0,
		"?": 0,
	}
	for _, entry := range statuses {
		status := strings.TrimSpace(entry.Status)
		if status == "" {
			continue
		}
		if strings.HasPrefix(status, "??") {
			counts["?"]++
			continue
		}
		if strings.Contains(status, "M") {
			counts["M"]++
		}
		if strings.Contains(status, "A") {
			counts["A"]++
		}
		if strings.Contains(status, "D") {
			counts["D"]++
		}
	}

	parts := []string{fmt.Sprintf("git: %s", branch)}
	if counts["M"] == 0 && counts["A"] == 0 && counts["D"] == 0 && counts["?"] == 0 {
		parts = append(parts, "clean")
	} else {
		if counts["M"] > 0 {
			parts = append(parts, fmt.Sprintf("M%d", counts["M"]))
		}
		if counts["A"] > 0 {
			parts = append(parts, fmt.Sprintf("A%d", counts["A"]))
		}
		if counts["D"] > 0 {
			parts = append(parts, fmt.Sprintf("D%d", counts["D"]))
		}
		if counts["?"] > 0 {
			parts = append(parts, fmt.Sprintf("?%d", counts["?"]))
		}
	}

	return strings.Join(parts, " ")
}

func applyStatuses(files []git.StatusEntry, statuses []git.StatusEntry) []git.StatusEntry {
	if len(files) == 0 || len(statuses) == 0 {
		return files
	}
	statusMap := make(map[string]string, len(statuses))
	for _, entry := range statuses {
		if entry.Path == "" {
			continue
		}
		statusMap[entry.Path] = entry.Status
	}
	if len(statusMap) == 0 {
		return files
	}
	merged := make([]git.StatusEntry, 0, len(files))
	for _, entry := range files {
		if status, ok := statusMap[entry.Path]; ok {
			entry.Status = status
		}
		merged = append(merged, entry)
	}
	return merged
}

func colorizeDiffLines(lines []string) []string {
	if len(lines) == 0 {
		return lines
	}
	plusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("71"))
	minusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("160"))
	hunkStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("69"))
	out := make([]string, len(lines))
	for i, line := range lines {
		if strings.HasPrefix(line, "+++ ") || strings.HasPrefix(line, "--- ") {
			out[i] = hunkStyle.Render(line)
			continue
		}
		switch {
		case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "+"):
			out[i] = plusStyle.Render(line)
		case strings.HasPrefix(line, "---") || strings.HasPrefix(line, "-"):
			out[i] = minusStyle.Render(line)
		case strings.HasPrefix(line, "@@"):
			out[i] = hunkStyle.Render(line)
		default:
			out[i] = line
		}
	}
	return out
}

func (m *Model) openCommitModal() {
	m.modal = modalCommit
	m.modalErr = ""
	m.commitText.SetValue("")
	m.commitText.Focus()
}

func (m *Model) openHelpModal() {
	m.modal = modalHelp
	m.modalErr = ""
	m.commitText.Blur()
}

func (m *Model) closeModal() {
	m.modal = modalNone
	m.modalErr = ""
	m.commitText.Blur()
}

func (m Model) handleModalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.modal {
	case modalCommit:
		switch msg.String() {
		case "esc":
			m.closeModal()
			return m, nil
		case "enter":
			message := strings.TrimSpace(m.commitText.Value())
			if message == "" {
				m.modalErr = "Commit message is required."
				return m, nil
			}
			m.modalErr = ""
			return m, m.commitCmd(message)
		default:
			var cmd tea.Cmd
			m.commitText, cmd = m.commitText.Update(msg)
			return m, cmd
		}
	case modalPush:
		switch msg.String() {
		case "esc":
			m.closeModal()
			return m, nil
		case "enter":
			m.modalErr = ""
			return m, m.pushCmd()
		}
	case modalHelp:
		switch msg.String() {
		case "esc", "enter":
			m.closeModal()
			return m, nil
		}
	}
	return m, nil
}

func (m Model) renderModal() string {
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1, 2)
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("62"))
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("160"))

	var title string
	var body []string
	switch m.modal {
	case modalCommit:
		title = titleStyle.Render("Commit")
		body = append(body, "Enter a commit message:")
		body = append(body, m.commitText.View())
		body = append(body, "")
		body = append(body, "Enter to commit, Esc to cancel.")
	case modalPush:
		title = titleStyle.Render("Push")
		body = append(body, "Commit created. Push now?")
		body = append(body, "")
		body = append(body, "Enter to push, Esc to cancel.")
	case modalHelp:
		title = titleStyle.Render("Help")
		body = append(body, "Navigation:")
		body = append(body, "  j/k or arrows to move/scroll")
		body = append(body, "  PgUp/PgDn for faster scroll")
		body = append(body, "  Tab to change focus")
		body = append(body, "")
		body = append(body, "Modes:")
		body = append(body, "  m to toggle explorer/diff")
		body = append(body, "")
		body = append(body, "Actions:")
		body = append(body, "  Enter to commit")
		body = append(body, "  h for help, q/Esc to quit")
	}
	if m.modalErr != "" {
		body = append(body, "")
		body = append(body, errStyle.Render(m.modalErr))
	}

	content := fmt.Sprintf("%s\n\n%s", title, strings.Join(body, "\n"))
	box := boxStyle.Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
