package ui

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/joshuazd/nit/internal/cli"
	"github.com/joshuazd/nit/internal/comments"
	diffpkg "github.com/joshuazd/nit/internal/diff"
	"github.com/joshuazd/nit/internal/git"
	"github.com/joshuazd/nit/internal/models"
)

// Diff modes
var (
	DiffModes     = []string{"branch", "unstaged", "staged", "all"}
	DiffModeLabels = map[string]string{
		"branch":   "branch diff",
		"unstaged": "unstaged",
		"staged":   "staged",
		"all":      "unpushed",
		"file":     "file review",
	}
)

// Messages
type (
	tickMsg       time.Time
	diffLoadedMsg struct {
		raw   string
		files []models.FileDiff
	}
	notifyMsg struct {
		text     string
		severity string
	}
	errMsg struct{ err error }
)

// Model is the root Bubble Tea model for the nit-go app.
type Model struct {
	// CLI args
	args cli.Args

	// Git state
	repoRoot string
	branch   string
	base     string
	diffMode string

	// App state
	fileDiffs      []models.FileDiff
	currentFile    *models.FileDiff
	fileIndex      int
	commentsData   []models.ReviewComment
	lastRawDiff    string
	fileReviewMode bool
	ignoreWS       bool
	pendingG       bool

	// Sub-models
	fileTree FileTreeModel
	diffView DiffViewModel
	input    InputModel

	// Notification
	notification string
	notifyExpiry time.Time

	// Dimensions
	width  int
	height int

	// Auto-refresh debouncing
	lastStat string

	// Quit tracking for export
	Quitting bool
	Comments []models.ReviewComment
}

// NewModel creates a new app model.
func NewModel(args cli.Args) Model {
	return Model{
		args:     args,
		diffMode: "branch",
		fileTree: NewFileTreeModel(),
		diffView: NewDiffViewModel(),
		input:    NewInputModel(),
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		m.initGit,
		tickCmd(),
	)
}

func tickCmd() tea.Cmd {
	return tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) initGit() tea.Msg {
	if m.args.FilePath != "" {
		return nil // handled in Update
	}
	root, err := git.GetRepoRoot("")
	if err != nil {
		return errMsg{fmt.Errorf("not a git repository")}
	}
	return diffLoadedMsg{raw: root} // abuse: first load uses raw as repoRoot signal
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateDimensions()
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case tickMsg:
		return m.handleTick()

	case diffLoadedMsg:
		return m.handleDiffLoaded(msg)

	case errMsg:
		m.notify(msg.err.Error(), "error")
		return m, nil

	default:
		// Forward cursor blink and other messages to text input when active
		if m.input.IsActive() {
			var cmd tea.Cmd
			m.input.TextInput, cmd = m.input.TextInput.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

func (m *Model) updateDimensions() {
	sidebarWidth := 40
	// Status bar (1) + footer (1) + border top/bottom on each panel (2)
	contentHeight := m.height - 4
	// Clamp the border height so panels don't use Height() —
	// we'll let lipgloss borders add their own 2 lines
	// The .Height() on borderStyle sets INNER height, so pass contentHeight directly
	if contentHeight < 1 {
		contentHeight = 1
	}
	m.fileTree.Height = contentHeight
	m.fileTree.Width = sidebarWidth - 2
	m.diffView.Height = contentHeight
	m.diffView.Width = m.width - sidebarWidth - 2
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// If input is active, handle input keys
	if m.input.IsActive() {
		return m.handleInputKey(msg)
	}

	// Handle g-chord
	if m.pendingG {
		m.pendingG = false
		switch msg.String() {
		case "g":
			m.diffView.CursorIndex = 0
		case "a":
			m.stageHunk()
		case "u":
			m.unstageHunk()
		case "x":
			m.discardHunk()
		case "c":
			m.input.StartCommit()
			return m, m.input.TextInput.Cursor.BlinkCmd()
		}
		return m, nil
	}

	switch {
	case key.Matches(msg, keys.Quit):
		m.Quitting = true
		m.Comments = m.commentsData
		return m, tea.Quit

	case key.Matches(msg, keys.CursorDown):
		m.diffView.MoveCursor(1)

	case key.Matches(msg, keys.CursorUp):
		m.diffView.MoveCursor(-1)

	case key.Matches(msg, keys.NextHunk):
		m.diffView.JumpToNextHunk(true)

	case key.Matches(msg, keys.PrevHunk):
		m.diffView.JumpToNextHunk(false)

	case key.Matches(msg, keys.NextFile):
		if idx := m.fileTree.NextFile(); idx >= 0 {
			m.selectFile(idx)
		}

	case key.Matches(msg, keys.PrevFile):
		if idx := m.fileTree.PrevFile(); idx >= 0 {
			m.selectFile(idx)
		}

	case key.Matches(msg, keys.CursorEnd):
		if len(m.diffView.DiffLines) > 0 {
			m.diffView.CursorIndex = len(m.diffView.DiffLines) - 1
		}

	case key.Matches(msg, keys.GPrefix):
		m.pendingG = true

	case key.Matches(msg, keys.Comment):
		m.startComment()
		return m, m.input.TextInput.Cursor.BlinkCmd()

	case key.Matches(msg, keys.DeleteComment):
		m.deleteComment()

	case key.Matches(msg, keys.CycleMode):
		m.cycleMode()

	case key.Matches(msg, keys.Refresh):
		m.refresh()

	case key.Matches(msg, keys.NextComment):
		fileComments := m.getFileComments()
		m.diffView.JumpToNextComment(true, fileComments)

	case key.Matches(msg, keys.PrevComment):
		fileComments := m.getFileComments()
		m.diffView.JumpToNextComment(false, fileComments)

	case key.Matches(msg, keys.ToggleSBS):
		m.diffView.SideBySide = !m.diffView.SideBySide
		m.reloadCurrentFile()

	case key.Matches(msg, keys.ToggleWord):
		m.diffView.WordDiff = !m.diffView.WordDiff
		m.reloadCurrentFile()

	case key.Matches(msg, keys.ToggleWS):
		m.ignoreWS = !m.ignoreWS
		m.loadDiff()

	case key.Matches(msg, keys.Export):
		m.exportComments()

	case key.Matches(msg, keys.ToggleSyntax):
		m.diffView.SyntaxHighlight = !m.diffView.SyntaxHighlight
		m.reloadCurrentFile()
	}

	return m, nil
}

func (m Model) handleInputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.diffView.ShowInput = false
		m.diffView.InputView = ""
		m.input.Cancel()
		return m, nil
	case tea.KeyEnter:
		mode := m.input.Mode // save before Submit resets it
		m.diffView.ShowInput = false
		m.diffView.InputView = ""
		val := strings.TrimSpace(m.input.Submit())
		if val != "" {
			if mode == InputCommit {
				m.doCommit(val)
			} else {
				m.addComment(val)
			}
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.input.TextInput, cmd = m.input.TextInput.Update(msg)
	return m, cmd
}

func (m *Model) startComment() {
	dl := m.diffView.GetCurrentLine()
	if dl == nil || dl.LineType == models.LineHunkHeader {
		return
	}
	if m.currentFile == nil {
		return
	}
	m.input.StartComment()
	m.diffView.ShowInput = true
}

func (m *Model) addComment(text string) {
	dl := m.diffView.GetCurrentLine()
	if dl == nil || m.currentFile == nil || m.repoRoot == "" {
		return
	}
	ctx := m.diffView.GetHunkContext(m.diffView.CursorIndex, 2)
	comment := comments.MakeComment(m.currentFile.Path, *dl, text, ctx, m.diffMode)
	m.commentsData = append(m.commentsData, comment)
	comments.Save(m.repoRoot, m.commentsData, m.branch, m.base)
	m.reloadCurrentFile()
}

func (m *Model) deleteComment() {
	dl := m.diffView.GetCurrentLine()
	if dl == nil || m.currentFile == nil || m.repoRoot == "" {
		return
	}
	var kept []models.ReviewComment
	deleted := false
	for _, c := range m.commentsData {
		if c.FilePath == m.currentFile.Path && comments.MatchesLine(c, *dl) {
			deleted = true
			continue
		}
		kept = append(kept, c)
	}
	if deleted {
		m.commentsData = kept
		comments.Save(m.repoRoot, m.commentsData, m.branch, m.base)
		m.reloadCurrentFile()
	}
}

func (m *Model) doCommit(message string) {
	if m.repoRoot == "" || m.fileReviewMode {
		return
	}
	_, err := git.Commit(message, m.repoRoot)
	if err != nil {
		m.notify("Commit failed: "+err.Error(), "error")
		return
	}
	m.notify("Committed", "info")
	m.loadDiff()
}

func (m *Model) stageHunk() {
	if m.fileReviewMode {
		return
	}
	if m.diffMode != "unstaged" && m.diffMode != "all" {
		m.notify("Stage: switch to unstaged/all mode", "warning")
		return
	}
	fd, hunk := m.diffView.GetCurrentHunk()
	if fd == nil || hunk == nil {
		return
	}
	patch := diffpkg.BuildPatch(fd, hunk)
	_, err := git.ApplyPatch(patch, m.repoRoot, true, false)
	if err != nil {
		m.notify("Stage failed: "+err.Error(), "error")
		return
	}
	m.notify("Hunk staged", "info")
	m.loadDiff()
}

func (m *Model) unstageHunk() {
	if m.fileReviewMode {
		return
	}
	if m.diffMode != "staged" {
		m.notify("Unstage: switch to staged mode", "warning")
		return
	}
	fd, hunk := m.diffView.GetCurrentHunk()
	if fd == nil || hunk == nil {
		return
	}
	patch := diffpkg.BuildPatch(fd, hunk)
	_, err := git.ApplyPatch(patch, m.repoRoot, true, true)
	if err != nil {
		m.notify("Unstage failed: "+err.Error(), "error")
		return
	}
	m.notify("Hunk unstaged", "info")
	m.loadDiff()
}

func (m *Model) discardHunk() {
	if m.fileReviewMode {
		return
	}
	if m.diffMode != "unstaged" && m.diffMode != "all" {
		m.notify("Discard: switch to unstaged/all mode", "warning")
		return
	}
	fd, hunk := m.diffView.GetCurrentHunk()
	if fd == nil || hunk == nil {
		return
	}
	patch := diffpkg.BuildPatch(fd, hunk)
	_, err := git.ApplyPatch(patch, m.repoRoot, false, true)
	if err != nil {
		m.notify("Discard failed: "+err.Error(), "error")
		return
	}
	m.notify("Hunk discarded", "info")
	m.loadDiff()
}

func (m *Model) cycleMode() {
	if m.fileReviewMode || m.args.CommitRange != "" {
		return
	}
	for i, mode := range DiffModes {
		if mode == m.diffMode {
			m.diffMode = DiffModes[(i+1)%len(DiffModes)]
			break
		}
	}
	m.loadDiff()
}

func (m *Model) refresh() {
	if m.repoRoot != "" {
		m.commentsData = comments.Load(m.repoRoot)
	}
	m.loadDiff()
}

func (m *Model) exportComments() {
	if len(m.commentsData) == 0 {
		m.notify("No comments to export", "warning")
		return
	}
	text := comments.ExportMarkdown(m.commentsData)
	if err := copyToClipboard(text); err != nil {
		m.notify("Clipboard copy failed", "error")
		return
	}
	m.notify(fmt.Sprintf("Copied %d comments to clipboard", len(m.commentsData)), "info")
}

func copyToClipboard(text string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "linux":
		// Try xclip first
		if _, err := exec.LookPath("xclip"); err == nil {
			cmd = exec.Command("xclip", "-selection", "clipboard")
		} else if _, err := exec.LookPath("xsel"); err == nil {
			cmd = exec.Command("xsel", "-selection", "clipboard")
		} else {
			return fmt.Errorf("no clipboard command found")
		}
	default:
		return fmt.Errorf("unsupported platform")
	}
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

func (m *Model) reloadCurrentFile() {
	if m.currentFile == nil {
		return
	}
	saved := m.diffView.CursorIndex
	fileComments := m.getFileComments()
	m.diffView.LoadFileDiff(m.currentFile, fileComments, saved)
}

func (m *Model) selectFile(index int) {
	if index < 0 || index >= len(m.fileDiffs) {
		return
	}
	m.fileIndex = index
	m.currentFile = &m.fileDiffs[index]
	m.fileTree.SelectByFileIndex(index)
	fileComments := m.getFileComments()
	m.diffView.LoadFileDiff(m.currentFile, fileComments, 0)
}

func (m *Model) getFileComments() []models.ReviewComment {
	if m.currentFile == nil {
		return nil
	}
	var fc []models.ReviewComment
	for _, c := range m.commentsData {
		if c.FilePath == m.currentFile.Path {
			fc = append(fc, c)
		}
	}
	return fc
}

func (m *Model) getRawDiff() string {
	if m.repoRoot == "" {
		return ""
	}
	cwd := m.repoRoot
	pf := m.args.PathFilter

	if m.args.CommitRange != "" {
		d, _ := git.GetCommitRangeDiff(m.args.CommitRange, cwd, pf, m.ignoreWS)
		return d
	}

	var d string
	switch m.diffMode {
	case "branch":
		d, _ = git.GetBranchDiffWithBase(m.base, cwd, pf, m.ignoreWS)
	case "unstaged":
		d, _ = git.GetUnstagedDiff(cwd, pf, m.ignoreWS)
	case "staged":
		d, _ = git.GetStagedDiff(cwd, pf, m.ignoreWS)
		return d // no untracked for staged
	case "all":
		d, _ = git.GetUnpushedDiff(cwd, pf, m.ignoreWS)
	}

	// Append untracked
	untracked, _ := git.GetUntrackedDiff(cwd, pf)
	return d + untracked
}

func (m *Model) getQuickStat() string {
	if m.repoRoot == "" {
		return ""
	}
	cwd := m.repoRoot
	var args []string
	if m.args.CommitRange != "" {
		args = []string{m.args.CommitRange}
	} else {
		switch m.diffMode {
		case "branch":
			args = []string{m.base + "...HEAD"}
		case "staged":
			args = []string{"--cached"}
		case "all":
			upstream := git.GetUpstreamRef(cwd)
			if upstream != "" {
				args = []string{upstream}
			}
		// unstaged: no extra args
		}
	}
	stat, _ := git.GetDiffStat(cwd, args...)
	return stat
}

func (m *Model) loadDiff() {
	raw := m.getRawDiff()
	m.lastRawDiff = raw
	m.fileDiffs = diffpkg.ParseDiff(raw)
	m.updateFileList()
}

func (m *Model) updateFileList() {
	cc := m.commentCounts()
	m.fileTree.Update(m.fileDiffs, cc)

	if len(m.fileDiffs) > 0 {
		restoreIdx := 0
		if m.currentFile != nil {
			for i, fd := range m.fileDiffs {
				if fd.Path == m.currentFile.Path {
					restoreIdx = i
					break
				}
			}
		}
		m.selectFile(restoreIdx)
	} else {
		m.currentFile = nil
		m.diffView.Clear()
	}
}

func (m *Model) commentCounts() map[string]int {
	cc := make(map[string]int)
	for _, c := range m.commentsData {
		cc[c.FilePath]++
	}
	return cc
}

func (m *Model) notify(text, severity string) {
	m.notification = text
	m.notifyExpiry = time.Now().Add(5 * time.Second)
}

func (m Model) handleTick() (tea.Model, tea.Cmd) {
	// Clear expired notifications
	if m.notification != "" && time.Now().After(m.notifyExpiry) {
		m.notification = ""
	}

	// Auto-refresh — use cheap stat check before full diff
	if !m.fileReviewMode && !m.input.IsActive() && m.repoRoot != "" {
		stat := m.getQuickStat()
		if stat != m.lastStat {
			m.lastStat = stat
			m.loadDiff()
		}
	}

	return m, tickCmd()
}

func (m Model) handleDiffLoaded(msg diffLoadedMsg) (tea.Model, tea.Cmd) {
	// First load: msg.raw is the repo root
	if m.repoRoot == "" {
		m.repoRoot = msg.raw
		// Parallel git metadata fetch
		var branch, base string
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); branch, _ = git.GetCurrentBranch(m.repoRoot) }()
		go func() { defer wg.Done(); base = git.GetMainBranch(m.repoRoot) }()
		wg.Wait()
		m.branch = branch
		if m.branch == "" {
			m.branch = "(detached HEAD)"
		}
		m.base = base

		if m.args.Mode != "" {
			m.diffMode = m.args.Mode
		} else if m.branch == m.base {
			m.diffMode = "unstaged"
		}

		m.commentsData = comments.Load(m.repoRoot)

		if m.args.FilePath != "" {
			m.mountFileReview()
		} else {
			m.loadDiff()
		}
		m.updateDimensions()
	}
	return m, nil
}

func (m *Model) mountFileReview() {
	m.fileReviewMode = true
	m.branch = m.args.FilePath
	m.diffMode = "file"

	content, err := readFileContent(m.args.FilePath)
	if err != nil {
		m.notify("File not found: "+m.args.FilePath, "error")
		return
	}
	m.fileDiffs = diffpkg.FileToDiff(m.args.FilePath, content)
	m.updateFileList()
}

func readFileContent(path string) (string, error) {
	data, err := exec.Command("cat", path).Output()
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// View implements tea.Model.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	sidebarWidth := 40
	diffWidth := m.width - sidebarWidth

	// Status bar
	modeLabel := DiffModeLabels[m.diffMode]
	if m.args.CommitRange != "" {
		modeLabel = m.args.CommitRange
	}
	statusBar := RenderStatusBar(m.width, m.branch, modeLabel, m.ignoreWS, len(m.fileDiffs), len(m.commentsData))

	// File tree — trim trailing newline so .Height() counts correctly
	treeContent := strings.TrimRight(m.fileTree.Render(), "\n")
	treeBorder := borderStyle.Width(sidebarWidth - 2).Height(m.diffView.Height).Render(treeContent)

	// Pipe text input view into diff view for inline comment input
	if m.input.IsActive() && m.input.Mode == InputComment {
		m.diffView.InputView = m.input.TextInput.View()
	}

	// Diff view — trim trailing newline so .Height() counts correctly
	diffContent := strings.TrimRight(m.diffView.Render(), "\n")
	diffBorder := borderStyle.Width(diffWidth - 2).Height(m.diffView.Height).Render(diffContent)

	// Layout: sidebar | diff view
	body := lipgloss.JoinHorizontal(lipgloss.Top, treeBorder, diffBorder)

	// Footer or input
	var footer string
	if m.input.IsActive() && m.input.Mode == InputCommit {
		footer = m.input.TextInput.View()
		if lipgloss.Width(footer) < m.width {
			footer = footerStyle.Width(m.width).Render(footer)
		}
	} else if m.input.IsActive() && m.input.Mode == InputComment {
		footer = RenderInputFooter(m.width, "")
	} else if m.notification != "" && time.Now().Before(m.notifyExpiry) {
		footer = footerStyle.Width(m.width).Render(m.notification)
	} else {
		footer = RenderFooter(m.width)
	}

	// Clamp: keep status bar (top) and footer (bottom), truncate body if needed
	statusLines := strings.Split(statusBar, "\n")
	bodyLines := strings.Split(body, "\n")
	footerLines := strings.Split(footer, "\n")

	maxBodyLines := m.height - len(statusLines) - len(footerLines)
	if maxBodyLines < 0 {
		maxBodyLines = 0
	}
	if len(bodyLines) > maxBodyLines {
		bodyLines = bodyLines[:maxBodyLines]
	}

	var all []string
	all = append(all, statusLines...)
	all = append(all, bodyLines...)
	all = append(all, footerLines...)
	if len(all) > m.height {
		all = all[:m.height]
	}
	return strings.Join(all, "\n")
}

// Run starts the Bubble Tea program.
func Run(args cli.Args) error {
	m := NewModel(args)
	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	// Handle export on quit
	fm := finalModel.(Model)
	if fm.Quitting && fm.args.ExportComments != "" {
		exportOnQuit(fm.args, fm.Comments)
	}

	return nil
}

func exportOnQuit(args cli.Args, cmts []models.ReviewComment) {
	if args.ExportComments == "" || len(cmts) == 0 {
		return
	}
	var text string
	if args.ExportFormat == "json" {
		text = comments.ExportJSON(cmts)
	} else {
		text = comments.ExportMarkdown(cmts)
	}
	if args.ExportComments == "-" {
		fmt.Println(text)
	} else {
		_ = writeFile(args.ExportComments, text)
	}
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}
