package syntax

import (
	"testing"
)

func TestDetectLanguagePython(t *testing.T) {
	if got := DetectLanguage("app.py"); got != "python" {
		t.Errorf("got %q", got)
	}
}

func TestDetectLanguageJavascript(t *testing.T) {
	if got := DetectLanguage("index.js"); got != "javascript" {
		t.Errorf("got %q", got)
	}
}

func TestDetectLanguageUnknown(t *testing.T) {
	if got := DetectLanguage("file.xyzunknown"); got != "" {
		t.Errorf("got %q", got)
	}
}

func TestHighlightLineReturnsString(t *testing.T) {
	result := HighlightLine("x = 1", "python")
	if result == "" {
		t.Error("empty result")
	}
}

func TestHighlightLineEmpty(t *testing.T) {
	result := HighlightLine("", "python")
	// Should not panic, should return something
	_ = result
}
