package ui

import "github.com/charmbracelet/lipgloss"

// ANSI standard colors — adapts to the user's terminal theme.
var (
	Black       = lipgloss.Color("0")
	Red         = lipgloss.Color("1")
	Green       = lipgloss.Color("2")
	Yellow      = lipgloss.Color("3")
	Magenta     = lipgloss.Color("5")
	Blue        = lipgloss.Color("4")
	Cyan        = lipgloss.Color("6")
	White       = lipgloss.Color("7")
	BrightBlack = lipgloss.Color("8")
	BrightWhite = lipgloss.Color("15")

	// Semantic aliases
	Dim = BrightBlack
)

// Status bar segment styles
var (
	segBranchStyle = lipgloss.NewStyle().
			Background(Magenta).
			Foreground(Black).
			Bold(true).
			Padding(0, 1)

	segModeStyle = lipgloss.NewStyle().
			Background(Cyan).
			Foreground(Black).
			Bold(true).
			Padding(0, 1)

	segFilesStyle = lipgloss.NewStyle().
			Background(Green).
			Foreground(Black).
			Bold(true).
			Padding(0, 1)

	segCommentsStyle = lipgloss.NewStyle().
				Background(Yellow).
				Foreground(Black).
				Bold(true).
				Padding(0, 1)
)

// File path bar (above diff viewer)
var filePathBarStyle = lipgloss.NewStyle().
	Background(Blue).
	Foreground(Black).
	Bold(true).
	Padding(0, 1)

// Diff line styles
var (
	lineAddStyle = lipgloss.NewStyle().
			Foreground(Green)

	lineRemoveStyle = lipgloss.NewStyle().
			Foreground(Red)

	lineContextStyle = lipgloss.NewStyle().
				Faint(true)

	lineHunkHeaderStyle = lipgloss.NewStyle().
				Foreground(Magenta).
				Bold(true)

	cursorStyle = lipgloss.NewStyle().
			Background(Black)

	lineNumberStyle = lipgloss.NewStyle().
			Faint(true)
)

// File tree styles
var (
	fileDimStyle = lipgloss.NewStyle().Faint(true)
	dirStyle     = lipgloss.NewStyle().Faint(true)
)

// Comment styles
var (
	commentStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Yellow).
			PaddingLeft(1).
			PaddingRight(1).
			MarginLeft(6)

	commentInputStyle = lipgloss.NewStyle().
				PaddingLeft(6)
)

// Border style
var (
	borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(BrightBlack)
)

// Footer — matches vigil: Black bg, faint keys, plain descriptions
var (
	footerStyle = lipgloss.NewStyle().
			Background(Black)

	footerKeyStyle = lipgloss.NewStyle().
			Background(Black).
			Faint(true)
)
