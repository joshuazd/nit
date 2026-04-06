package ui

import (
	"github.com/charmbracelet/bubbles/textinput"
)

// InputMode represents the current input state.
type InputMode int

const (
	InputNone   InputMode = iota
	InputComment
	InputCommit
)

// InputModel wraps a text input for comment and commit entry.
type InputModel struct {
	Mode        InputMode
	TextInput   textinput.Model
	Placeholder string
}

// NewInputModel creates a new input model.
func NewInputModel() InputModel {
	ti := textinput.New()
	ti.CharLimit = 500
	return InputModel{
		Mode:      InputNone,
		TextInput: ti,
	}
}

// StartComment activates comment input mode.
func (m *InputModel) StartComment() {
	m.Mode = InputComment
	m.TextInput.Placeholder = "comment (enter to submit, esc to cancel)"
	m.TextInput.Reset()
	m.TextInput.Focus()
}

// StartCommit activates commit input mode.
func (m *InputModel) StartCommit() {
	m.Mode = InputCommit
	m.TextInput.Placeholder = "commit message (enter to commit, esc to cancel)"
	m.TextInput.Reset()
	m.TextInput.Focus()
}

// Cancel cancels the current input.
func (m *InputModel) Cancel() {
	m.Mode = InputNone
	m.TextInput.Blur()
	m.TextInput.Reset()
}

// Submit returns the current value and resets.
func (m *InputModel) Submit() string {
	val := m.TextInput.Value()
	m.Mode = InputNone
	m.TextInput.Blur()
	m.TextInput.Reset()
	return val
}

// IsActive returns true if an input is active.
func (m *InputModel) IsActive() bool {
	return m.Mode != InputNone
}
