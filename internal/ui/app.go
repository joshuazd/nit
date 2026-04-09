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
	repoInitMsg   struct{ repoRoot string }
	diffLoadedMsg struct {
		raw       string
		fileDiffs []models.FileDiff
		gen       uint64
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

	// Async diff loading
	loading bool
	loadGen uint64

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
	return repoInitMsg{repoRoot: root}
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

	case repoInitMsg:
		return m.handleRepoInit(msg)

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
	m.diffView.Height = contentHeight - 1 // reserve 1 line for file path header
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
		var cmd tea.Cmd
		switch msg.String() {
		case "g":
			m.diffView.CursorIndex = 0
		case "a":
			cmd = m.stageHunk()
		case "u":
			cmd = m.unstageHunk()
		case "x":
			cmd = m.discardHunk()
		case "c":
			m.input.StartCommit()
			return m, m.input.TextInput.Cursor.BlinkCmd()
		}
		return m, cmd
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

	case key.Matches(msg, keys.HalfPageDown):
		m.diffView.MoveCursor(m.diffView.Height / 2)

	case key.Matches(msg, keys.HalfPageUp):
		m.diffView.MoveCursor(-m.diffView.Height / 2)

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
		return m, m.cycleMode()

	case key.Matches(msg, keys.Refresh):
		return m, m.refresh()

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
		return m, m.startLoadDiff()

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
				return m, m.doCommit(val)
			}
			m.addComment(val)
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
	_ = comments.Save(m.repoRoot, m.commentsData, m.branch, m.base)
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
		_ = comments.Save(m.repoRoot, m.commentsData, m.branch, m.base)
		m.reloadCurrentFile()
	}
}

func (m *Model) doCommit(message string) tea.Cmd {
	if m.repoRoot == "" || m.fileReviewMode {
		return nil
	}
	_, err := git.Commit(message, m.repoRoot)
	if err != nil {
		m.notify("Commit failed: "+err.Error(), "error")
		return nil
	}
	m.notify("Committed", "info")
	return m.startLoadDiff()
}

func (m *Model) stageHunk() tea.Cmd {
	if m.fileReviewMode {
		return nil
	}
	if m.diffMode != "unstaged" && m.diffMode != "all" {
		m.notify("Stage: switch to unstaged/all mode", "warning")
		return nil
	}
	fd, hunk := m.diffView.GetCurrentHunk()
	if fd == nil || hunk == nil {
		return nil
	}
	patch := diffpkg.BuildPatch(fd, hunk)
	_, err := git.ApplyPatch(patch, m.repoRoot, true, false)
	if err != nil {
		m.notify("Stage failed: "+err.Error(), "error")
		return nil
	}
	m.notify("Hunk staged", "info")
	return m.startLoadDiff()
}

func (m *Model) unstageHunk() tea.Cmd {
	if m.fileReviewMode {
		return nil
	}
	if m.diffMode != "staged" {
		m.notify("Unstage: switch to staged mode", "warning")
		return nil
	}
	fd, hunk := m.diffView.GetCurrentHunk()
	if fd == nil || hunk == nil {
		return nil
	}
	patch := diffpkg.BuildPatch(fd, hunk)
	_, err := git.ApplyPatch(patch, m.repoRoot, true, true)
	if err != nil {
		m.notify("Unstage failed: "+err.Error(), "error")
		return nil
	}
	m.notify("Hunk unstaged", "info")
	return m.startLoadDiff()
}

func (m *Model) discardHunk() tea.Cmd {
	if m.fileReviewMode {
		return nil
	}
	if m.diffMode != "unstaged" && m.diffMode != "all" {
		m.notify("Discard: switch to unstaged/all mode", "warning")
		return nil
	}
	fd, hunk := m.diffView.GetCurrentHunk()
	if fd == nil || hunk == nil {
		return nil
	}
	patch := diffpkg.BuildPatch(fd, hunk)
	_, err := git.ApplyPatch(patch, m.repoRoot, false, true)
	if err != nil {
		m.notify("Discard failed: "+err.Error(), "error")
		return nil
	}
	m.notify("Hunk discarded", "info")
	return m.startLoadDiff()
}

func (m *Model) cycleMode() tea.Cmd {
	if m.fileReviewMode || m.args.CommitRange != "" {
		return nil
	}
	for i, mode := range DiffModes {
		if mode == m.diffMode {
			m.diffMode = DiffModes[(i+1)%len(DiffModes)]
			break
		}
	}
	return m.startLoadDiff()
}

func (m *Model) refresh() tea.Cmd {
	if m.repoRoot != "" {
		m.commentsData = comments.Load(m.repoRoot)
	}
	return m.startLoadDiff()
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

func getRawDiffPure(repoRoot, diffMode, base, pathFilter, commitRange string, ignoreWS bool) string {
	if repoRoot == "" {
		return ""
	}

	if commitRange != "" {
		d, _ := git.GetCommitRangeDiff(commitRange, repoRoot, pathFilter, ignoreWS)
		return d
	}

	var d string
	switch diffMode {
	case "branch":
		d, _ = git.GetBranchDiffWithBase(base, repoRoot, pathFilter, ignoreWS)
		return d // no untracked for branch diff
	case "unstaged":
		d, _ = git.GetUnstagedDiff(repoRoot, pathFilter, ignoreWS)
	case "staged":
		d, _ = git.GetStagedDiff(repoRoot, pathFilter, ignoreWS)
		return d // no untracked for staged
	case "all":
		d, _ = git.GetUnpushedDiff(repoRoot, pathFilter, ignoreWS)
	}

	// Append untracked
	untracked, _ := git.GetUntrackedDiff(repoRoot, pathFilter)
	return d + untracked
}

func loadDiffCmd(repoRoot, diffMode, base, pathFilter, commitRange string, ignoreWS bool, gen uint64) tea.Cmd {
	return func() tea.Msg {
		raw := getRawDiffPure(repoRoot, diffMode, base, pathFilter, commitRange, ignoreWS)
		return diffLoadedMsg{raw: raw, fileDiffs: diffpkg.ParseDiff(raw), gen: gen}
	}
}

func (m *Model) startLoadDiff() tea.Cmd {
	m.loading = true
	m.loadGen++
	return loadDiffCmd(m.repoRoot, m.diffMode, m.base, m.args.PathFilter, m.args.CommitRange, m.ignoreWS, m.loadGen)
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
	if !m.fileReviewMode && !m.input.IsActive() && m.repoRoot != "" && !m.loading {
		stat := m.getQuickStat()
		if stat != m.lastStat {
			m.lastStat = stat
			cmd := m.startLoadDiff()
			return m, tea.Batch(cmd, tickCmd())
		}
	}

	return m, tickCmd()
}

func (m Model) handleRepoInit(msg repoInitMsg) (tea.Model, tea.Cmd) {
	m.repoRoot = msg.repoRoot
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
	m.updateDimensions()

	if m.args.FilePath != "" {
		m.mountFileReview()
		return m, nil
	}
	return m, m.startLoadDiff()
}

func (m Model) handleDiffLoaded(msg diffLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.gen != m.loadGen {
		return m, nil // stale
	}
	m.loading = false
	m.lastRawDiff = msg.raw
	m.fileDiffs = msg.fileDiffs
	m.updateFileList()
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
	if m.loading {
		modeLabel += " ..."
	}
	statusBar := RenderStatusBar(m.width, m.branch, modeLabel, m.ignoreWS, len(m.fileDiffs), len(m.commentsData))

	// File tree — trim trailing newline so .Height() counts correctly
	treeContent := strings.TrimRight(m.fileTree.Render(), "\n")
	treeBorder := borderStyle.Width(sidebarWidth - 2).Height(m.diffView.Height + 1).Render(treeContent)

	// Pipe text input view into diff view for inline comment input
	if m.input.IsActive() && m.input.Mode == InputComment {
		m.diffView.InputView = m.input.TextInput.View()
	}

	// File path header above diff content
	filePathLine := ""
	if m.currentFile != nil {
		filePathLine = filePathBarStyle.Width(diffWidth - 2).MaxHeight(1).Render(m.currentFile.Path)
	} else {
		filePathLine = filePathBarStyle.Width(diffWidth - 2).MaxHeight(1).Render("")
	}

	// Diff view — trim trailing newline so .Height() counts correctly
	diffContent := strings.TrimRight(m.diffView.Render(), "\n")
	diffWithHeader := filePathLine + "\n" + diffContent
	diffBorder := borderStyle.Width(diffWidth - 2).Height(m.diffView.Height + 1).Render(diffWithHeader)

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
