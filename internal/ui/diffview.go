package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/joshuazd/nit/internal/comments"
	"github.com/joshuazd/nit/internal/diff"
	"github.com/joshuazd/nit/internal/models"
	"github.com/joshuazd/nit/internal/syntax"
)

// wordDiffResult stores cached word diff segments for a line pair.
type wordDiffResult struct {
	OldSegs []diff.DiffSegment
	NewSegs []diff.DiffSegment
}

// DiffViewModel holds the state for the diff view panel.
type DiffViewModel struct {
	DiffLines   []models.DiffLine
	LineHunkMap []*models.DiffHunk
	SBSRows     []models.SideBySideRow
	CurrentFile *models.FileDiff

	CursorIndex     int
	SideBySide      bool
	WordDiff        bool
	SyntaxHighlight bool

	Scroll int
	Height int
	Width  int

	// Inline comment input
	ShowInput bool
	InputView string

	// Comment data (line index → comment texts)
	commentLines map[int][]string

	// Cached render state — invalidated on toggle change
	renderCache    map[int]string // line index → rendered string
	renderCacheKey uint32         // hash of toggle state for invalidation

	// Word diff cache — computed once per file load
	wordPairs     map[int]models.DiffLine  // flat index → paired line (unified)
	wordDiffCache map[int]*wordDiffResult  // flat index → cached segments

	// Detected language (cached per file)
	lang string
}

// NewDiffViewModel creates a new diff view model.
func NewDiffViewModel() DiffViewModel {
	return DiffViewModel{
		commentLines: make(map[int][]string),
	}
}

// Clear resets the diff view.
func (m *DiffViewModel) Clear() {
	m.DiffLines = nil
	m.LineHunkMap = nil
	m.SBSRows = nil
	m.CurrentFile = nil
	m.CursorIndex = 0
	m.Scroll = 0
	m.commentLines = make(map[int][]string)
	m.renderCache = nil
	m.wordPairs = nil
	m.wordDiffCache = nil
	m.lang = ""
}

func (m *DiffViewModel) toggleKey() uint32 {
	var k uint32
	if m.SideBySide {
		k |= 1
	}
	if m.WordDiff {
		k |= 2
	}
	if m.SyntaxHighlight {
		k |= 4
	}
	// Include width so resize invalidates SBS cache
	k |= uint32(m.Width) << 8
	return k
}



// LoadFileDiff loads a file diff into the view.
// Only builds data structures — rendering is deferred to Render().
func (m *DiffViewModel) LoadFileDiff(fd *models.FileDiff, fileComments []models.ReviewComment, restoreCursor int) {
	m.Clear()
	m.CurrentFile = fd

	if fd.IsBinary {
		m.DiffLines = []models.DiffLine{{Content: "Binary file", LineType: models.LineContext}}
		return
	}

	// Build comment lookup
	commentMap := make(map[int][]models.ReviewComment)
	for i := range fileComments {
		c := &fileComments[i]
		var k int
		if c.NewLineNo != nil {
			k = *c.NewLineNo
		} else if c.OldLineNo != nil {
			k = *c.OldLineNo
		}
		commentMap[k] = append(commentMap[k], *c)
	}

	if m.SideBySide {
		m.loadSideBySideData(fd, commentMap)
	} else {
		m.loadUnifiedData(fd, commentMap)
	}

	if restoreCursor > 0 && restoreCursor < len(m.DiffLines) {
		m.CursorIndex = restoreCursor
	}
}

func (m *DiffViewModel) loadUnifiedData(fd *models.FileDiff, commentMap map[int][]models.ReviewComment) {
	// Estimate capacity
	totalLines := 0
	for _, h := range fd.Hunks {
		totalLines += len(h.Lines)
	}
	m.DiffLines = make([]models.DiffLine, 0, totalLines)
	m.LineHunkMap = make([]*models.DiffHunk, 0, totalLines)

	// Build word diff pairs map (cheap — just index pairing, no actual diff)
	if m.WordDiff {
		m.wordPairs = make(map[int]models.DiffLine)
		allLines := m.collectAllLines(fd)
		i := 0
		for i < len(allLines) {
			if allLines[i].LineType == models.LineRemove {
				var removes, adds []int
				for i < len(allLines) && allLines[i].LineType == models.LineRemove {
					removes = append(removes, i)
					i++
				}
				for i < len(allLines) && allLines[i].LineType == models.LineAdd {
					adds = append(adds, i)
					i++
				}
				for ri, ai := 0, 0; ri < len(removes) && ai < len(adds); ri, ai = ri+1, ai+1 {
					m.wordPairs[removes[ri]] = allLines[adds[ai]]
					m.wordPairs[adds[ai]] = allLines[removes[ri]]
				}
			} else {
				i++
			}
		}
		m.wordDiffCache = make(map[int]*wordDiffResult)
	}

	flatIdx := 0
	for hi := range fd.Hunks {
		hunk := &fd.Hunks[hi]
		for _, dl := range hunk.Lines {
			m.DiffLines = append(m.DiffLines, dl)
			m.LineHunkMap = append(m.LineHunkMap, hunk)

			// Map comments
			lineKey := dl.NewLineNo
			if lineKey == 0 {
				lineKey = dl.OldLineNo
			}
			if cs, ok := commentMap[lineKey]; ok {
				for _, c := range cs {
					if comments.MatchesLine(c, dl) {
						m.commentLines[flatIdx] = append(m.commentLines[flatIdx], c.Comment)
					}
				}
			}
			flatIdx++
		}
	}

	m.lang = ""
	if m.SyntaxHighlight {
		m.lang = syntax.DetectLanguage(fd.Path)
	}
}

func (m *DiffViewModel) loadSideBySideData(fd *models.FileDiff, commentMap map[int][]models.ReviewComment) {
	m.lang = ""
	if m.SyntaxHighlight {
		m.lang = syntax.DetectLanguage(fd.Path)
	}

	idx := 0
	for hi := range fd.Hunks {
		hunk := &fd.Hunks[hi]
		rows := diff.AlignHunkLines(hunk.Lines)
		for _, row := range rows {
			m.SBSRows = append(m.SBSRows, row)
			m.LineHunkMap = append(m.LineHunkMap, hunk)

			primary := row.Right
			if primary == nil {
				primary = row.Left
			}
			m.DiffLines = append(m.DiffLines, *primary)

			lineKey := primary.NewLineNo
			if lineKey == 0 {
				lineKey = primary.OldLineNo
			}
			if cs, ok := commentMap[lineKey]; ok {
				for _, c := range cs {
					if comments.MatchesLine(c, *primary) {
						m.commentLines[idx] = append(m.commentLines[idx], c.Comment)
					}
				}
			}
			idx++
		}
	}

	// Pre-compute word diff cache for SBS
	if m.WordDiff {
		m.wordDiffCache = make(map[int]*wordDiffResult)
	}
}

// renderLine renders a single line on-demand, using cache.
// bg is non-nil for cursor lines — baked into every style to avoid ANSI nesting issues.
func (m *DiffViewModel) renderLine(idx int, bg *lipgloss.Color) string {
	// Only cache non-cursor lines (cursor bg changes every frame)
	if bg == nil {
		key := m.toggleKey()
		if m.renderCache != nil && m.renderCacheKey == key {
			if cached, ok := m.renderCache[idx]; ok {
				return cached
			}
		} else {
			m.renderCache = make(map[int]string)
			m.renderCacheKey = key
		}
	}

	var line string
	if m.SideBySide && idx < len(m.SBSRows) {
		half := m.Width/2 - 8
		if half < 20 {
			half = 20
		}
		lang := ""
		if m.SBSRows[idx].RowType == models.RowContext {
			lang = m.lang
		}
		line = m.formatSBSRow(m.SBSRows[idx], half, lang, idx, bg)
	} else if idx < len(m.DiffLines) {
		dl := m.DiffLines[idx]
		lineNoStr := formatLineNo(dl)
		prefix := formatPrefix(dl)

		if m.WordDiff {
			if pair, ok := m.wordPairs[idx]; ok {
				line = m.formatWordDiffLine(dl, pair, lineNoStr, prefix, idx, bg)
			} else {
				line = formatPlainLine(dl, lineNoStr, prefix, m.lang, bg)
			}
		} else {
			line = formatPlainLine(dl, lineNoStr, prefix, m.lang, bg)
		}
	}

	if bg == nil {
		m.renderCache[idx] = line
	}
	return line
}

// getWordDiff returns cached word diff segments, computing if needed.
func (m *DiffViewModel) getWordDiff(idx int, oldText, newText string) *wordDiffResult {
	if m.wordDiffCache == nil {
		m.wordDiffCache = make(map[int]*wordDiffResult)
	}
	if cached, ok := m.wordDiffCache[idx]; ok {
		return cached
	}
	oldSegs, newSegs := diff.WordDiffSegments(oldText, newText)
	result := &wordDiffResult{OldSegs: oldSegs, NewSegs: newSegs}
	m.wordDiffCache[idx] = result
	return result
}

func (m *DiffViewModel) collectAllLines(fd *models.FileDiff) []models.DiffLine {
	total := 0
	for _, hunk := range fd.Hunks {
		total += len(hunk.Lines)
	}
	all := make([]models.DiffLine, 0, total)
	for _, hunk := range fd.Hunks {
		all = append(all, hunk.Lines...)
	}
	return all
}

// MoveCursor moves the cursor by delta, clamping to bounds.
func (m *DiffViewModel) MoveCursor(delta int) {
	if len(m.DiffLines) == 0 {
		return
	}
	newIdx := m.CursorIndex + delta
	if newIdx < 0 {
		newIdx = 0
	}
	if newIdx >= len(m.DiffLines) {
		newIdx = len(m.DiffLines) - 1
	}
	m.CursorIndex = newIdx
}

// JumpToNextHunk jumps to the next/previous hunk header.
func (m *DiffViewModel) JumpToNextHunk(forward bool) {
	if len(m.DiffLines) == 0 {
		return
	}
	step := 1
	if !forward {
		step = -1
	}
	idx := m.CursorIndex + step
	for idx >= 0 && idx < len(m.DiffLines) {
		if m.DiffLines[idx].LineType == models.LineHunkHeader {
			m.CursorIndex = idx
			return
		}
		idx += step
	}
}

// JumpToNextComment jumps to the next/previous commented line.
func (m *DiffViewModel) JumpToNextComment(forward bool, fileComments []models.ReviewComment) {
	if len(m.DiffLines) == 0 || len(fileComments) == 0 {
		return
	}

	commented := make(map[[2]int]bool)
	for _, c := range fileComments {
		var newNo, oldNo int
		if c.NewLineNo != nil {
			newNo = *c.NewLineNo
		}
		if c.OldLineNo != nil {
			oldNo = *c.OldLineNo
		}
		commented[[2]int{newNo, oldNo}] = true
	}

	step := 1
	if !forward {
		step = -1
	}
	idx := m.CursorIndex + step
	for idx >= 0 && idx < len(m.DiffLines) {
		dl := m.DiffLines[idx]
		if commented[[2]int{dl.NewLineNo, dl.OldLineNo}] {
			m.CursorIndex = idx
			return
		}
		idx += step
	}
}

// GetCurrentHunk returns the file diff and hunk at the cursor.
func (m *DiffViewModel) GetCurrentHunk() (*models.FileDiff, *models.DiffHunk) {
	if m.CurrentFile == nil || len(m.LineHunkMap) == 0 {
		return nil, nil
	}
	if m.CursorIndex >= 0 && m.CursorIndex < len(m.LineHunkMap) {
		return m.CurrentFile, m.LineHunkMap[m.CursorIndex]
	}
	return nil, nil
}

// GetCurrentLine returns the diff line at the cursor.
func (m *DiffViewModel) GetCurrentLine() *models.DiffLine {
	if m.CursorIndex >= 0 && m.CursorIndex < len(m.DiffLines) {
		return &m.DiffLines[m.CursorIndex]
	}
	return nil
}

// GetHunkContext returns surrounding line contents for a comment.
func (m *DiffViewModel) GetHunkContext(centerIdx, radius int) []string {
	start := centerIdx - radius
	if start < 0 {
		start = 0
	}
	end := centerIdx + radius + 1
	if end > len(m.DiffLines) {
		end = len(m.DiffLines)
	}
	ctx := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		ctx = append(ctx, m.DiffLines[i].Content)
	}
	return ctx
}

// Render renders only the visible viewport.
func (m *DiffViewModel) Render() string {
	if len(m.DiffLines) == 0 {
		return ""
	}

	m.ensureScrollVisible()

	var b strings.Builder
	displayHeight := m.Height
	if displayHeight <= 0 {
		displayHeight = len(m.DiffLines)
	}

	visibleLine := 0
	renderedIdx := m.Scroll
	for renderedIdx < len(m.DiffLines) && visibleLine < displayHeight {
		isCursor := renderedIdx == m.CursorIndex

		// Blank line before hunk headers (except first)
		if renderedIdx > 0 && m.DiffLines[renderedIdx].LineType == models.LineHunkHeader &&
			visibleLine+1 < displayHeight {
			b.WriteByte('\n')
			visibleLine++
		}

		var line string
		if m.SideBySide {
			// SBS: bake bg into every segment (wrapping handled internally)
			var bg *lipgloss.Color
			if isCursor {
				bg = &Black
			}
			line = m.renderLine(renderedIdx, bg)
		} else {
			// Unified: render without bg, then wrap+bg in a single style pass
			line = m.renderLine(renderedIdx, nil)
			if m.Width > 0 {
				wrapStyle := lipgloss.NewStyle().Width(m.Width)
				if isCursor {
					wrapStyle = wrapStyle.Background(Black)
				}
				line = wrapStyle.Render(line)
			} else if isCursor {
				line = cursorStyle.Render(line)
			}
		}

		lineHeight := strings.Count(line, "\n") + 1
		if visibleLine+lineHeight > displayHeight {
			break
		}
		b.WriteString(line)
		b.WriteByte('\n')
		visibleLine += lineHeight

		// Inline comment input
		if isCursor && m.ShowInput && m.InputView != "" && visibleLine < displayHeight {
			inputLine := commentInputStyle.Render(m.InputView)
			b.WriteString(inputLine)
			b.WriteByte('\n')
			visibleLine++
		}

		// Comment blocks
		if cmts, ok := m.commentLines[renderedIdx]; ok {
			for _, cmt := range cmts {
				commentLine := commentStyle.Render(cmt)
				commentHeight := strings.Count(commentLine, "\n") + 1
				if visibleLine+commentHeight > displayHeight {
					break
				}
				b.WriteString(commentLine)
				b.WriteByte('\n')
				visibleLine += commentHeight
			}
		}

		renderedIdx++
	}

	// Pad remaining
	for visibleLine < displayHeight {
		b.WriteByte('\n')
		visibleLine++
	}

	return b.String()
}

func (m *DiffViewModel) ensureScrollVisible() {
	if m.Height <= 0 || len(m.DiffLines) == 0 {
		return
	}
	if m.Scroll < 0 {
		m.Scroll = 0
	}

	// Cursor above scroll — snap to cursor
	if m.CursorIndex < m.Scroll {
		m.Scroll = m.CursorIndex
		return
	}

	// Cursor below viewport — compute visual height from scroll to cursor
	// to determine if we need to advance scroll
	for {
		visLines := 0
		cursorFound := false
		for idx := m.Scroll; idx < len(m.DiffLines); idx++ {
			// Blank line before hunk headers
			if idx > 0 && m.DiffLines[idx].LineType == models.LineHunkHeader {
				visLines++
			}
			h := m.visualLineHeight(idx)
			visLines += h
			// Account for comment blocks after this line
			if cmts, ok := m.commentLines[idx]; ok {
				for _, cmt := range cmts {
					commentLine := commentStyle.Render(cmt)
					visLines += strings.Count(commentLine, "\n") + 1
				}
			}
			if idx == m.CursorIndex {
				cursorFound = true
				break
			}
			if visLines >= m.Height {
				break
			}
		}
		if cursorFound && visLines <= m.Height {
			break // cursor is visible
		}
		// Advance scroll
		if m.Scroll >= m.CursorIndex {
			break // safety: don't scroll past cursor
		}
		m.Scroll++
	}
}

// visualLineHeight returns how many visual lines a logical line occupies.
func (m *DiffViewModel) visualLineHeight(idx int) int {
	line := m.renderLine(idx, nil)
	if m.Width > 0 && !m.SideBySide {
		line = lipgloss.NewStyle().Width(m.Width).Render(line)
	}
	return strings.Count(line, "\n") + 1
}

// --- Formatting helpers ---

func formatLineNo(dl models.DiffLine) string {
	if dl.LineType == models.LineHunkHeader {
		return ""
	}
	old := ""
	if dl.OldLineNo != 0 {
		old = fmt.Sprintf("%d", dl.OldLineNo)
	}
	new := ""
	if dl.NewLineNo != 0 {
		new = fmt.Sprintf("%d", dl.NewLineNo)
	}
	return fmt.Sprintf("%4s %4s │ ", old, new)
}

func formatPrefix(dl models.DiffLine) string {
	switch dl.LineType {
	case models.LineAdd:
		return "+"
	case models.LineRemove:
		return "-"
	case models.LineHunkHeader:
		return ""
	default:
		return " "
	}
}

func formatPlainLine(dl models.DiffLine, lineNoStr, prefix, lang string, bg *lipgloss.Color) string {
	addStyle := withBg(lineAddStyle, bg)
	removeStyle := withBg(lineRemoveStyle, bg)
	ctxStyle := withBg(lineContextStyle, bg)
	hdrStyle := withBg(lineHunkHeaderStyle, bg)
	numStyle := withBg(lineNumberStyle, bg)

	switch dl.LineType {
	case models.LineHunkHeader:
		return hdrStyle.Render(dl.Content)
	case models.LineAdd:
		return addStyle.Render(lineNoStr + prefix + dl.Content)
	case models.LineRemove:
		return removeStyle.Render(lineNoStr + prefix + dl.Content)
	case models.LineContext:
		if lang != "" {
			highlighted := syntax.HighlightLine(dl.Content, lang)
			return numStyle.Render(lineNoStr+prefix) + highlighted
		}
		return ctxStyle.Render(lineNoStr + prefix + dl.Content)
	default:
		plain := lipgloss.NewStyle()
		if bg != nil {
			plain = plain.Background(*bg)
		}
		return plain.Render(lineNoStr + prefix + dl.Content)
	}
}

// withBg returns a copy of the style with bg added (if non-nil).
func withBg(s lipgloss.Style, bg *lipgloss.Color) lipgloss.Style {
	if bg != nil {
		return s.Background(*bg)
	}
	return s
}

func (m *DiffViewModel) formatWordDiffLine(dl, pair models.DiffLine, lineNoStr, prefix string, idx int, bg *lipgloss.Color) string {
	var segs []diff.DiffSegment
	var baseStyle, highlightStyle lipgloss.Style

	if dl.LineType == models.LineRemove {
		result := m.getWordDiff(idx, dl.Content, pair.Content)
		segs = result.OldSegs
		baseStyle = withBg(lineRemoveStyle, bg)
		highlightStyle = withBg(lipgloss.NewStyle().Foreground(Red).Bold(true).Reverse(true), bg)
	} else {
		result := m.getWordDiff(idx, pair.Content, dl.Content)
		segs = result.NewSegs
		baseStyle = withBg(lineAddStyle, bg)
		highlightStyle = withBg(lipgloss.NewStyle().Foreground(Green).Bold(true).Reverse(true), bg)
	}

	var b strings.Builder
	b.WriteString(baseStyle.Render(lineNoStr + prefix))
	for _, seg := range segs {
		if seg.Changed {
			b.WriteString(highlightStyle.Render(seg.Text))
		} else {
			b.WriteString(baseStyle.Render(seg.Text))
		}
	}
	return b.String()
}

func (m *DiffViewModel) formatSBSRow(row models.SideBySideRow, half int, lang string, idx int, bg *lipgloss.Color) string {
	if row.RowType == models.RowHunkHeader && row.Left != nil {
		return withBg(lineHunkHeaderStyle, bg).Render(row.Left.Content)
	}

	sep := withBg(fileDimStyle, bg).Render(" │ ")
	lineNoW := 6 // "NNNN P" — 4 digits + space + prefix

	// Build styled content for each side (without line numbers)
	leftContent, leftStyle := m.sbsSideContent(row.Left, row, half, lang, idx, true, bg)
	rightContent, rightStyle := m.sbsSideContent(row.Right, row, half, lang, idx, false, bg)

	// Build line number strings
	leftLineNo := sbsLineNo(row.Left, true)
	rightLineNo := sbsLineNo(row.Right, false)

	// Wrap each side's content within half width
	leftWrapped := wrapToWidth(leftContent, half)
	rightWrapped := wrapToWidth(rightContent, half)

	// Pad shorter side
	maxLines := len(leftWrapped)
	if len(rightWrapped) > maxLines {
		maxLines = len(rightWrapped)
	}

	var b strings.Builder
	plain := lipgloss.NewStyle()
	if bg != nil {
		plain = plain.Background(*bg)
	}
	blankLineNo := plain.Render(strings.Repeat(" ", lineNoW))
	blankHalf := plain.Render(strings.Repeat(" ", half))

	for i := 0; i < maxLines; i++ {
		if i > 0 {
			b.WriteByte('\n')
		}

		// Left
		if i == 0 {
			b.WriteString(leftStyle.Render(leftLineNo))
		} else {
			b.WriteString(blankLineNo)
		}
		if i < len(leftWrapped) {
			content := leftWrapped[i]
			b.WriteString(leftStyle.Render(fmt.Sprintf("%-*s", half, content)))
		} else {
			b.WriteString(blankHalf)
		}

		b.WriteString(sep)

		// Right
		if i == 0 {
			b.WriteString(rightStyle.Render(rightLineNo))
		} else {
			b.WriteString(blankLineNo)
		}
		if i < len(rightWrapped) {
			b.WriteString(rightStyle.Render(rightWrapped[i]))
		}
	}

	return b.String()
}

// sbsSideContent returns the styled text content for one side of a SBS row.
func (m *DiffViewModel) sbsSideContent(dl *models.DiffLine, row models.SideBySideRow, half int, lang string, idx int, isLeft bool, bg *lipgloss.Color) (string, lipgloss.Style) {
	if dl == nil {
		plain := lipgloss.NewStyle()
		if bg != nil {
			plain = plain.Background(*bg)
		}
		return "", plain
	}

	if dl.LineType == models.LineRemove {
		style := withBg(lineRemoveStyle, bg)
		if m.WordDiff && row.Right != nil {
			result := m.getWordDiff(idx, row.Left.Content, row.Right.Content)
			return renderWordDiffSegments(result.OldSegs, style,
				withBg(lipgloss.NewStyle().Foreground(Red).Bold(true).Reverse(true), bg)), style
		}
		return dl.Content, style
	}

	if dl.LineType == models.LineAdd {
		style := withBg(lineAddStyle, bg)
		if m.WordDiff && row.Left != nil {
			result := m.getWordDiff(idx, row.Left.Content, row.Right.Content)
			return renderWordDiffSegments(result.NewSegs, style,
				withBg(lipgloss.NewStyle().Foreground(Green).Bold(true).Reverse(true), bg)), style
		}
		return dl.Content, style
	}

	// Context
	if lang != "" {
		return syntax.HighlightLine(dl.Content, lang), withBg(lineNumberStyle, bg)
	}
	return dl.Content, withBg(lineContextStyle, bg)
}

func renderWordDiffSegments(segs []diff.DiffSegment, base, highlight lipgloss.Style) string {
	var b strings.Builder
	for _, seg := range segs {
		if seg.Changed {
			b.WriteString(highlight.Render(seg.Text))
		} else {
			b.WriteString(base.Render(seg.Text))
		}
	}
	return b.String()
}

func sbsLineNo(dl *models.DiffLine, isLeft bool) string {
	if dl == nil {
		return "      "
	}
	no := ""
	prefix := " "
	if isLeft {
		if dl.OldLineNo != 0 {
			no = fmt.Sprintf("%d", dl.OldLineNo)
		}
		if dl.LineType == models.LineRemove {
			prefix = "-"
		}
	} else {
		if dl.NewLineNo != 0 {
			no = fmt.Sprintf("%d", dl.NewLineNo)
		}
		if dl.LineType == models.LineAdd {
			prefix = "+"
		}
	}
	return fmt.Sprintf("%4s %s", no, prefix)
}

// wrapToWidth splits text into lines that fit within maxWidth.
func wrapToWidth(text string, maxWidth int) []string {
	if maxWidth <= 0 {
		return []string{text}
	}
	wrapped := lipgloss.NewStyle().Width(maxWidth).Render(text)
	return strings.Split(wrapped, "\n")
}

