package ui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Quit          key.Binding
	CursorDown    key.Binding
	CursorUp      key.Binding
	NextHunk      key.Binding
	PrevHunk      key.Binding
	NextFile      key.Binding
	PrevFile      key.Binding
	CursorEnd     key.Binding
	GPrefix       key.Binding
	Comment       key.Binding
	DeleteComment key.Binding
	CycleMode     key.Binding
	Refresh       key.Binding
	NextComment   key.Binding
	PrevComment   key.Binding
	ToggleSBS     key.Binding
	ToggleWord    key.Binding
	ToggleWS      key.Binding
	Export        key.Binding
	ToggleSyntax  key.Binding
}

var keys = keyMap{
	Quit:          key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	CursorDown:    key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j", "down")),
	CursorUp:      key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k", "up")),
	NextHunk:      key.NewBinding(key.WithKeys("J"), key.WithHelp("J", "next hunk")),
	PrevHunk:      key.NewBinding(key.WithKeys("K"), key.WithHelp("K", "prev hunk")),
	NextFile:      key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "next file")),
	PrevFile:      key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "prev file")),
	CursorEnd:     key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "end")),
	GPrefix:       key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "g-chord")),
	Comment:       key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "comment")),
	DeleteComment: key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
	CycleMode:     key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "mode")),
	Refresh:       key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
	NextComment:   key.NewBinding(key.WithKeys("]"), key.WithHelp("]", "next comment")),
	PrevComment:   key.NewBinding(key.WithKeys("["), key.WithHelp("[", "prev comment")),
	ToggleSBS:     key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "split")),
	ToggleWord:    key.NewBinding(key.WithKeys("w"), key.WithHelp("w", "word")),
	ToggleWS:      key.NewBinding(key.WithKeys("W"), key.WithHelp("W", "whitespace")),
	Export:        key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "export")),
	ToggleSyntax:  key.NewBinding(key.WithKeys("h"), key.WithHelp("h", "highlight")),
}

// FooterBindings returns the visible keybinding hints for the footer.
func FooterBindings() []key.Binding {
	return []key.Binding{
		keys.NextFile,
		keys.PrevFile,
		keys.Comment,
		keys.DeleteComment,
		keys.CycleMode,
		keys.ToggleSBS,
		keys.ToggleWord,
		keys.ToggleWS,
	}
}
