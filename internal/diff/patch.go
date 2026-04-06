package diff

import (
	"fmt"
	"strings"

	"github.com/joshuazd/nit/internal/models"
)

// BuildPatch constructs a minimal git-apply-compatible patch for a single hunk.
func BuildPatch(fileDiff *models.FileDiff, hunk *models.DiffHunk) string {
	path := fileDiff.Path
	oldPath := fileDiff.OldPath
	if oldPath == "" {
		oldPath = path
	}

	var b strings.Builder

	fmt.Fprintf(&b, "diff --git a/%s b/%s\n", oldPath, path)

	switch fileDiff.Status {
	case models.StatusAdded:
		b.WriteString("--- /dev/null\n")
		fmt.Fprintf(&b, "+++ b/%s\n", path)
	case models.StatusDeleted:
		fmt.Fprintf(&b, "--- a/%s\n", oldPath)
		b.WriteString("+++ /dev/null\n")
	default:
		fmt.Fprintf(&b, "--- a/%s\n", oldPath)
		fmt.Fprintf(&b, "+++ b/%s\n", path)
	}

	b.WriteString(hunk.Header)
	b.WriteByte('\n')

	for _, dl := range hunk.Lines {
		if dl.LineType == models.LineHunkHeader {
			continue
		}
		b.WriteString(dl.Raw)
		b.WriteByte('\n')
	}

	return b.String()
}
