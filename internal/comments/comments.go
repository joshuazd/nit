package comments

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/joshuazd/nit/internal/models"
)

const CommentFile = ".nit.json"

// nitJSON is the on-disk format for .nit.json.
type nitJSON struct {
	Version  int               `json:"version"`
	Branch   string            `json:"branch"`
	Base     string            `json:"base"`
	Comments []commentJSON     `json:"comments"`
}

type commentJSON struct {
	File        string   `json:"file"`
	Line        *int     `json:"line,omitempty"`
	OldLine     *int     `json:"old_line,omitempty"`
	LineContent string   `json:"line_content"`
	Comment     string   `json:"comment"`
	HunkContext []string `json:"hunk_context"`
	Timestamp   string   `json:"timestamp"`
	DiffMode    string   `json:"diff_mode"`
}

func toCommentJSON(c models.ReviewComment) commentJSON {
	return commentJSON{
		File:        c.FilePath,
		Line:        c.NewLineNo,
		OldLine:     c.OldLineNo,
		LineContent: c.LineContent,
		Comment:     c.Comment,
		HunkContext: c.HunkContext,
		Timestamp:   c.Timestamp,
		DiffMode:    c.DiffMode,
	}
}

func fromCommentJSON(c commentJSON) models.ReviewComment {
	hunkCtx := c.HunkContext
	if hunkCtx == nil {
		hunkCtx = []string{}
	}
	diffMode := c.DiffMode
	if diffMode == "" {
		diffMode = "branch"
	}
	return models.ReviewComment{
		FilePath:    c.File,
		NewLineNo:   c.Line,
		OldLineNo:   c.OldLine,
		LineContent: c.LineContent,
		Comment:     c.Comment,
		HunkContext: hunkCtx,
		Timestamp:   c.Timestamp,
		DiffMode:    diffMode,
	}
}

// Load reads comments from .nit.json in the given repo root.
func Load(repoRoot string) []models.ReviewComment {
	path := filepath.Join(repoRoot, CommentFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var nj nitJSON
	if err := json.Unmarshal(data, &nj); err != nil {
		return nil
	}
	comments := make([]models.ReviewComment, 0, len(nj.Comments))
	for _, c := range nj.Comments {
		comments = append(comments, fromCommentJSON(c))
	}
	return comments
}

// Save writes comments to .nit.json atomically.
func Save(repoRoot string, comments []models.ReviewComment, branch, base string) error {
	cjs := make([]commentJSON, len(comments))
	for i, c := range comments {
		cjs[i] = toCommentJSON(c)
	}
	nj := nitJSON{
		Version:  1,
		Branch:   branch,
		Base:     base,
		Comments: cjs,
	}

	data, err := json.MarshalIndent(nj, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	// Atomic write via temp file + rename
	tmp, err := os.CreateTemp(repoRoot, ".nit.tmp.*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		// Clean up on error
		os.Remove(tmpPath)
	}()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	return os.Rename(tmpPath, filepath.Join(repoRoot, CommentFile))
}

// ExportMarkdown exports comments as markdown grouped by file.
func ExportMarkdown(comments []models.ReviewComment) string {
	if len(comments) == 0 {
		return ""
	}

	byFile := make(map[string][]models.ReviewComment)
	for _, c := range comments {
		byFile[c.FilePath] = append(byFile[c.FilePath], c)
	}

	var paths []string
	for p := range byFile {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	var b strings.Builder
	b.WriteString("# Code Review Comments\n\n")

	for _, path := range paths {
		fmt.Fprintf(&b, "## %s\n\n", path)
		cs := byFile[path]
		sort.Slice(cs, func(i, j int) bool {
			li := lineNo(cs[i])
			lj := lineNo(cs[j])
			return li < lj
		})
		for _, c := range cs {
			var lineRef string
			if c.NewLineNo != nil {
				lineRef = fmt.Sprintf("L%d", *c.NewLineNo)
			} else if c.OldLineNo != nil {
				lineRef = fmt.Sprintf("old L%d", *c.OldLineNo)
			}
			fmt.Fprintf(&b, "- **%s**: %s\n", lineRef, c.Comment)
			if c.LineContent != "" {
				b.WriteString("  ```\n")
				fmt.Fprintf(&b, "  %s\n", c.LineContent)
				b.WriteString("  ```\n")
			}
		}
		b.WriteString("\n")
	}

	return b.String()
}

func lineNo(c models.ReviewComment) int {
	if c.NewLineNo != nil {
		return *c.NewLineNo
	}
	if c.OldLineNo != nil {
		return *c.OldLineNo
	}
	return 0
}

// ExportJSON exports comments as a JSON array.
func ExportJSON(comments []models.ReviewComment) string {
	cjs := make([]commentJSON, len(comments))
	for i, c := range comments {
		cjs[i] = toCommentJSON(c)
	}
	data, _ := json.MarshalIndent(cjs, "", "  ")
	return string(data)
}

// MatchesLine checks if a comment matches a given diff line.
func MatchesLine(c models.ReviewComment, dl models.DiffLine) bool {
	if c.NewLineNo != nil && dl.NewLineNo == *c.NewLineNo {
		return true
	}
	if c.OldLineNo != nil && dl.OldLineNo == *c.OldLineNo && dl.NewLineNo == 0 {
		return true
	}
	return false
}

// MakeComment creates a new ReviewComment for a diff line.
func MakeComment(filePath string, line models.DiffLine, commentText string, hunkContext []string, diffMode string) models.ReviewComment {
	var newLineNo, oldLineNo *int
	if line.NewLineNo != 0 {
		n := line.NewLineNo
		newLineNo = &n
	}
	if line.OldLineNo != 0 {
		o := line.OldLineNo
		oldLineNo = &o
	}
	return models.ReviewComment{
		FilePath:    filePath,
		NewLineNo:   newLineNo,
		OldLineNo:   oldLineNo,
		LineContent: line.Content,
		Comment:     commentText,
		HunkContext: hunkContext,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		DiffMode:    diffMode,
	}
}
