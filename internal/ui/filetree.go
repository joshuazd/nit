package ui

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/joshuazd/nit/internal/models"
)

// FileTreeEntry represents a single file in the tree.
type FileTreeEntry struct {
	Index        int
	FileDiff     *models.FileDiff
	CommentCount int
}

// FileTreeNode represents a node in the file tree (directory or file).
type FileTreeNode struct {
	Label    string
	IsDir    bool
	Entry    *FileTreeEntry // nil for directories
	Children []*FileTreeNode
}

// displayLine is a flattened tree line for rendering.
type displayLine struct {
	prefix  string // tree guide prefix (e.g. "│ ├ ")
	node    *FileTreeNode
	fileIdx int // index into TreeOrder, or -1 for directories
}

// getOrCreateDir ensures all parent directories exist in the tree.
func getOrCreateDir(root *FileTreeNode, dir string, cache map[string]*FileTreeNode) *FileTreeNode {
	if dir == "." || dir == "" {
		return root
	}
	if node, ok := cache[dir]; ok {
		return node
	}
	parent := getOrCreateDir(root, filepath.Dir(dir), cache)
	node := &FileTreeNode{
		Label: filepath.Base(dir) + "/",
		IsDir: true,
	}
	parent.Children = append(parent.Children, node)
	cache[dir] = node
	return node
}

// buildFileTree creates a proper nested tree from file diffs.
func buildFileTree(files []models.FileDiff, commentCounts map[string]int) (*FileTreeNode, []int) {
	root := &FileTreeNode{IsDir: true}
	dirCache := map[string]*FileTreeNode{"": root, ".": root}

	for i := range files {
		fd := &files[i]
		dir := filepath.Dir(fd.Path)
		parent := getOrCreateDir(root, dir, dirCache)

		cc := commentCounts[fd.Path]
		leaf := &FileTreeNode{
			IsDir: false,
			Entry: &FileTreeEntry{Index: i, FileDiff: fd, CommentCount: cc},
		}
		parent.Children = append(parent.Children, leaf)
	}

	// Sort: directories first, then files, alphabetical within each group
	sortTree(root)

	// Build treeOrder from sorted traversal
	var treeOrder []int
	collectFileOrder(root, &treeOrder)

	return root, treeOrder
}

func sortTree(node *FileTreeNode) {
	sort.SliceStable(node.Children, func(i, j int) bool {
		ci, cj := node.Children[i], node.Children[j]
		if ci.IsDir != cj.IsDir {
			return ci.IsDir // dirs first
		}
		// Alphabetical within group
		nameI := ci.Label
		if !ci.IsDir {
			nameI = filepath.Base(ci.Entry.FileDiff.Path)
		}
		nameJ := cj.Label
		if !cj.IsDir {
			nameJ = filepath.Base(cj.Entry.FileDiff.Path)
		}
		return nameI < nameJ
	})
	for _, child := range node.Children {
		if child.IsDir {
			sortTree(child)
		}
	}
}

func collectFileOrder(node *FileTreeNode, order *[]int) {
	for _, child := range node.Children {
		if child.IsDir {
			collectFileOrder(child, order)
		} else {
			*order = append(*order, child.Entry.Index)
		}
	}
}

// flattenTree converts a tree into display lines with guide prefixes.
func flattenTree(node *FileTreeNode) []displayLine {
	var lines []displayLine
	fileIdx := 0
	flattenChildren(node, "", &lines, &fileIdx)
	return lines
}

func flattenChildren(node *FileTreeNode, prefix string, lines *[]displayLine, fileIdx *int) {
	for i, child := range node.Children {
		isLast := i == len(node.Children)-1

		var connector string
		var childPrefix string
		if isLast {
			connector = "└ "
			childPrefix = prefix + "  "
		} else {
			connector = "├ "
			childPrefix = prefix + "│ "
		}

		linePrefix := prefix + connector
		if child.IsDir {
			*lines = append(*lines, displayLine{prefix: linePrefix, node: child, fileIdx: -1})
			flattenChildren(child, childPrefix, lines, fileIdx)
		} else {
			*lines = append(*lines, displayLine{prefix: linePrefix, node: child, fileIdx: *fileIdx})
			*fileIdx++
		}
	}
}

// FileTreeModel holds the file tree state.
type FileTreeModel struct {
	root         *FileTreeNode
	displayLines []displayLine
	TreeOrder    []int // file indices in tree display order
	Cursor       int   // cursor position (index into TreeOrder, only file nodes)
	Scroll       int   // scroll offset into displayLines
	Height       int
	Width        int
}

// NewFileTreeModel creates a new file tree model.
func NewFileTreeModel() FileTreeModel {
	return FileTreeModel{}
}

// Update rebuilds the tree from file diffs.
func (m *FileTreeModel) Update(files []models.FileDiff, commentCounts map[string]int) {
	m.root, m.TreeOrder = buildFileTree(files, commentCounts)
	m.displayLines = flattenTree(m.root)
	if m.Cursor >= len(m.TreeOrder) && len(m.TreeOrder) > 0 {
		m.Cursor = len(m.TreeOrder) - 1
	}
}

// SelectedIndex returns the file index of the currently selected tree entry.
func (m *FileTreeModel) SelectedIndex() int {
	if m.Cursor >= 0 && m.Cursor < len(m.TreeOrder) {
		return m.TreeOrder[m.Cursor]
	}
	return -1
}

// SelectByFileIndex moves cursor to the tree entry matching the given file index.
func (m *FileTreeModel) SelectByFileIndex(fileIdx int) {
	for i, idx := range m.TreeOrder {
		if idx == fileIdx {
			m.Cursor = i
			return
		}
	}
}

// NextFile advances to the next file in tree order.
func (m *FileTreeModel) NextFile() int {
	if len(m.TreeOrder) == 0 {
		return -1
	}
	m.Cursor = (m.Cursor + 1) % len(m.TreeOrder)
	return m.TreeOrder[m.Cursor]
}

// PrevFile goes to the previous file in tree order.
func (m *FileTreeModel) PrevFile() int {
	if len(m.TreeOrder) == 0 {
		return -1
	}
	m.Cursor = (m.Cursor - 1 + len(m.TreeOrder)) % len(m.TreeOrder)
	return m.TreeOrder[m.Cursor]
}

// cursorDisplayIdx returns the index into displayLines for the current cursor file.
func (m *FileTreeModel) cursorDisplayIdx() int {
	for i, dl := range m.displayLines {
		if dl.fileIdx == m.Cursor {
			return i
		}
	}
	return -1
}

// Render renders the file tree as a string.
func (m *FileTreeModel) Render() string {
	if len(m.displayLines) == 0 {
		return ""
	}

	// Ensure scroll keeps cursor visible
	cursorIdx := m.cursorDisplayIdx()
	if cursorIdx >= 0 {
		if cursorIdx < m.Scroll {
			m.Scroll = cursorIdx
		}
		if m.Height > 0 && cursorIdx >= m.Scroll+m.Height {
			m.Scroll = cursorIdx - m.Height + 1
		}
	}

	var b strings.Builder
	visibleHeight := m.Height
	if visibleHeight <= 0 {
		visibleHeight = len(m.displayLines)
	}

	end := m.Scroll + visibleHeight
	if end > len(m.displayLines) {
		end = len(m.displayLines)
	}

	for i := m.Scroll; i < end; i++ {
		dl := m.displayLines[i]

		guidePrefix := dirStyle.Render(dl.prefix)

		if dl.node.IsDir {
			line := guidePrefix + dirStyle.Render(dl.node.Label)
			if m.Width > 0 {
				line = truncate(line, m.Width)
			}
			b.WriteString(line)
		} else {
			isCursor := dl.fileIdx == m.Cursor
			line := m.renderFileEntry(dl.node.Entry, guidePrefix, isCursor)
			if m.Width > 0 {
				line = truncate(line, m.Width)
			}
			b.WriteString(line)
		}
		b.WriteByte('\n')
	}

	return b.String()
}

func (m *FileTreeModel) renderFileEntry(e *FileTreeEntry, prefix string, isCursor bool) string {
	fd := e.FileDiff
	letter := fileStatusLetter(fd.Status)
	name := filepath.Base(fd.Path)
	commentSuffix := ""
	if e.CommentCount > 0 {
		commentSuffix = fmt.Sprintf(" (%d)", e.CommentCount)
	}

	var bg *lipgloss.Color
	if isCursor {
		bg = &Black
	}

	styledLetter := styledFg(fileStatusColor(fd.Status), bg).Bold(true).Render(letter)
	plain := styledPlain(bg)
	commentStr := ""
	if commentSuffix != "" {
		commentStr = styledFaint(bg).Render(commentSuffix)
	}

	if isCursor {
		// Re-render prefix with cursor bg too
		prefix = styledPlain(bg).Render(stripAnsi(prefix))
	}

	return prefix + styledLetter + plain.Render(" "+name) + commentStr
}

// stripAnsi removes ANSI escape sequences from a string.
func stripAnsi(s string) string {
	return ansi.Strip(s)
}

func styledFg(fg lipgloss.Color, bg *lipgloss.Color) lipgloss.Style {
	s := lipgloss.NewStyle().Foreground(fg)
	if bg != nil {
		s = s.Background(*bg)
	}
	return s
}

func styledPlain(bg *lipgloss.Color) lipgloss.Style {
	s := lipgloss.NewStyle()
	if bg != nil {
		s = s.Background(*bg)
	}
	return s
}

func styledFaint(bg *lipgloss.Color) lipgloss.Style {
	s := lipgloss.NewStyle().Faint(true)
	if bg != nil {
		s = s.Background(*bg)
	}
	return s
}

func fileStatusColor(status models.FileStatus) lipgloss.Color {
	switch status {
	case models.StatusAdded:
		return Green
	case models.StatusDeleted:
		return Red
	case models.StatusRenamed:
		return Cyan
	default:
		return Yellow
	}
}

func fileStatusLetter(status models.FileStatus) string {
	switch status {
	case models.StatusAdded:
		return "A"
	case models.StatusDeleted:
		return "D"
	case models.StatusRenamed:
		return "R"
	default:
		return "M"
	}
}


func truncate(s string, maxWidth int) string {
	if lipgloss.Width(s) <= maxWidth {
		return s
	}
	return ansi.Truncate(s, maxWidth, "")
}
