package syntax

import (
	"os"
	"strings"
	"sync"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

var (
	lexerCache sync.Map // map[string]chroma.Lexer
)

func getFormatter() chroma.Formatter {
	if os.Getenv("NO_COLOR") != "" {
		return nil
	}
	return formatters.Get("terminal256")
}

func getStyle() *chroma.Style {
	if os.Getenv("NO_COLOR") != "" {
		return styles.Get("pygments")
	}
	return styles.Get("monokai")
}

func getLexer(language string) chroma.Lexer {
	if v, ok := lexerCache.Load(language); ok {
		return v.(chroma.Lexer)
	}
	l := lexers.Get(language)
	if l == nil {
		return nil
	}
	l = chroma.Coalesce(l)
	lexerCache.Store(language, l)
	return l
}

// DetectLanguage detects the language from a file path extension.
// Returns "" if the language is unknown.
func DetectLanguage(filePath string) string {
	l := lexers.Match(filePath)
	if l == nil {
		return ""
	}
	name := strings.ToLower(l.Config().Name)
	if name == "text" || name == "default" || name == "plaintext" {
		return ""
	}
	return name
}

// HighlightLine returns ANSI-highlighted text for a single line of code.
// Returns the original text if highlighting fails or NO_COLOR is set.
func HighlightLine(code, language string) string {
	f := getFormatter()
	if f == nil {
		return code
	}
	l := getLexer(language)
	if l == nil {
		return code
	}
	style := getStyle()

	iter, err := l.Tokenise(nil, code)
	if err != nil {
		return code
	}

	var b strings.Builder
	err = f.Format(&b, style, iter)
	if err != nil {
		return code
	}

	result := strings.TrimRight(b.String(), " \t\n\r")
	return result
}
