package diff

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/joshuazd/nit/internal/models"
)

var (
	diffHeaderRE  = regexp.MustCompile(`^diff --git a/(.*) b/(.*)`)
	hunkHeaderRE  = regexp.MustCompile(`^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@(.*)`)
	renameFromRE  = regexp.MustCompile(`^rename from (.*)`)
	renameToRE    = regexp.MustCompile(`^rename to (.*)`)
	newFileRE     = regexp.MustCompile(`^new file mode`)
	deletedFileRE = regexp.MustCompile(`^deleted file mode`)
	binaryRE      = regexp.MustCompile(`^Binary files`)
)

// ParseDiff parses unified diff text into structured FileDiff objects.
func ParseDiff(text string) []models.FileDiff {
	if strings.TrimSpace(text) == "" {
		return nil
	}

	lines := strings.Split(text, "\n")
	// Pre-allocate with heuristic capacities
	files := make([]models.FileDiff, 0, len(lines)/50+1)
	var currentFile *models.FileDiff
	var currentHunk *models.DiffHunk
	oldNo := 0
	newNo := 0

	for _, line := range lines {
		// New file diff
		if m := diffHeaderRE.FindStringSubmatch(line); m != nil {
			fd := models.FileDiff{
				Path:    m[2],
				OldPath: m[1],
				Status:  models.StatusModified,
			}
			if m[1] != m[2] {
				fd.Status = models.StatusRenamed
			}
			files = append(files, fd)
			currentFile = &files[len(files)-1]
			currentHunk = nil
			continue
		}

		if currentFile == nil {
			continue
		}

		// File metadata
		if newFileRE.MatchString(line) {
			currentFile.Status = models.StatusAdded
			continue
		}
		if deletedFileRE.MatchString(line) {
			currentFile.Status = models.StatusDeleted
			continue
		}
		if m := renameFromRE.FindStringSubmatch(line); m != nil {
			currentFile.OldPath = m[1]
			currentFile.Status = models.StatusRenamed
			continue
		}
		if m := renameToRE.FindStringSubmatch(line); m != nil {
			currentFile.Path = m[1]
			continue
		}
		if binaryRE.MatchString(line) {
			currentFile.IsBinary = true
			continue
		}

		// Skip index and ---/+++ lines
		if strings.HasPrefix(line, "index ") || strings.HasPrefix(line, "--- ") || strings.HasPrefix(line, "+++ ") {
			continue
		}

		// Hunk header
		if m := hunkHeaderRE.FindStringSubmatch(line); m != nil {
			oldNo, _ = strconv.Atoi(m[1])
			newNo, _ = strconv.Atoi(m[2])
			hunkLines := make([]models.DiffLine, 1, 64)
			hunkLines[0] = models.DiffLine{
				Content:  strings.TrimSpace(m[3]),
				LineType: models.LineHunkHeader,
				Raw:      line,
			}
			hunk := models.DiffHunk{
				Header:   line,
				OldStart: oldNo,
				NewStart: newNo,
				Lines:    hunkLines,
			}
			currentFile.Hunks = append(currentFile.Hunks, hunk)
			currentHunk = &currentFile.Hunks[len(currentFile.Hunks)-1]
			continue
		}

		if currentHunk == nil {
			continue
		}

		// Diff lines
		if strings.HasPrefix(line, "+") {
			currentHunk.Lines = append(currentHunk.Lines, models.DiffLine{
				Content:   line[1:],
				LineType:  models.LineAdd,
				NewLineNo: newNo,
				Raw:       line,
			})
			newNo++
		} else if strings.HasPrefix(line, "-") {
			currentHunk.Lines = append(currentHunk.Lines, models.DiffLine{
				Content:   line[1:],
				LineType:  models.LineRemove,
				OldLineNo: oldNo,
				Raw:       line,
			})
			oldNo++
		} else if strings.HasPrefix(line, `\`) {
			// "\ No newline at end of file"
			continue
		} else {
			// Context line (starts with space or is empty)
			content := line
			if strings.HasPrefix(line, " ") {
				content = line[1:]
			}
			currentHunk.Lines = append(currentHunk.Lines, models.DiffLine{
				Content:   content,
				LineType:  models.LineContext,
				OldLineNo: oldNo,
				NewLineNo: newNo,
				Raw:       line,
			})
			oldNo++
			newNo++
		}
	}

	return files
}

// FileToDiff creates a synthetic FileDiff showing all lines as context for file review.
func FileToDiff(path string, content string) []models.FileDiff {
	linesText := strings.Split(content, "\n")
	// Remove trailing empty line from final newline
	if len(linesText) > 0 && linesText[len(linesText)-1] == "" {
		linesText = linesText[:len(linesText)-1]
	}
	n := len(linesText)

	header := fmt.Sprintf("@@ -1,%d +1,%d @@", n, n)
	diffLines := make([]models.DiffLine, 0, n+1)
	diffLines = append(diffLines, models.DiffLine{
		Content:  header,
		LineType: models.LineHunkHeader,
		Raw:      header,
	})

	for i, line := range linesText {
		lineNo := i + 1
		diffLines = append(diffLines, models.DiffLine{
			Content:   line,
			LineType:  models.LineContext,
			OldLineNo: lineNo,
			NewLineNo: lineNo,
			Raw:       " " + line,
		})
	}

	hunk := models.DiffHunk{
		Header:   header,
		Lines:    diffLines,
		OldStart: 1,
		NewStart: 1,
	}
	return []models.FileDiff{
		{Path: path, Status: models.StatusModified, Hunks: []models.DiffHunk{hunk}},
	}
}
