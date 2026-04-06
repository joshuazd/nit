package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
)

// RenderFooter renders the keybinding hints bar (vigil-style).
func RenderFooter(width int) string {
	bindings := FooterBindings()
	p := footerStyle   // plain on bar bg
	k := footerKeyStyle // faint on bar bg
	var parts []string
	for _, b := range bindings {
		help := b.Help()
		parts = append(parts, k.Render(help.Key)+p.Render(" "+help.Desc))
	}
	content := strings.Join(parts, p.Render("  "))
	return footerStyle.Width(width).Padding(0, 1).MaxHeight(1).Render(content)
}

// RenderInputFooter renders a footer hint for input mode.
func RenderInputFooter(width int, hint string) string {
	p := footerStyle
	k := footerKeyStyle
	return footerStyle.Width(width).Padding(0, 1).MaxHeight(1).Render(
		k.Render("enter") + p.Render(" submit") +
			p.Render("  ") +
			k.Render("esc") + p.Render(" cancel"),
	)
}

// Ensure key.Binding satisfies what we need
var _ = key.NewBinding
