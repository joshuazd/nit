package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// RenderStatusBar renders the 4-segment status bar.
func RenderStatusBar(width int, branch, modeLabel string, ignoreWS bool, fileCount, commentCount int) string {
	wsIndicator := ""
	if ignoreWS {
		wsIndicator = " [no-ws]"
	}

	branchText := fmt.Sprintf("⎇ %s", branch)
	modeText := fmt.Sprintf("⇄  %s%s", modeLabel, wsIndicator)
	filesText := fmt.Sprintf("▤ %d files", fileCount)
	commentsText := fmt.Sprintf("✎ %d comments", commentCount)

	// Fixed-width segments
	modeW := lipgloss.Width(modeText) + 2 // padding
	filesW := lipgloss.Width(filesText) + 2
	commentsW := lipgloss.Width(commentsText) + 2
	branchW := width - modeW - filesW - commentsW
	if branchW < 10 {
		branchW = 10
	}

	// Truncate branch name if needed
	branchContent := branchText
	if lipgloss.Width(branchContent) > branchW-2 {
		branchContent = branchContent[:branchW-3] + "…"
	}

	seg1 := segBranchStyle.Width(branchW).MaxHeight(1).Render(branchContent)
	seg2 := segModeStyle.Width(modeW).MaxHeight(1).Render(modeText)
	seg3 := segFilesStyle.Width(filesW).MaxHeight(1).Render(filesText)
	seg4 := segCommentsStyle.Width(commentsW).MaxHeight(1).Render(commentsText)

	return lipgloss.JoinHorizontal(lipgloss.Top, seg1, seg2, seg3, seg4)
}
