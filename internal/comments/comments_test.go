package comments

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joshuazd/nit/internal/models"
)

func intPtr(v int) *int { return &v }

func TestLoadMissingFile(t *testing.T) {
	dir := t.TempDir()
	got := Load(dir)
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestSaveAndLoadRoundtrip(t *testing.T) {
	dir := t.TempDir()
	comment := models.ReviewComment{
		FilePath:    "src/app.py",
		NewLineNo:   intPtr(42),
		OldLineNo:   nil,
		LineContent: "return result",
		Comment:     "Handle empty case",
		HunkContext: []string{"line1", "return result", "line3"},
		Timestamp:   "2025-01-15T10:00:00+00:00",
		DiffMode:    "branch",
	}
	if err := Save(dir, []models.ReviewComment{comment}, "feature", "main"); err != nil {
		t.Fatal(err)
	}

	loaded := Load(dir)
	if len(loaded) != 1 {
		t.Fatalf("expected 1, got %d", len(loaded))
	}
	c := loaded[0]
	if c.FilePath != "src/app.py" {
		t.Errorf("file_path = %q", c.FilePath)
	}
	if c.NewLineNo == nil || *c.NewLineNo != 42 {
		t.Errorf("new_line_no = %v", c.NewLineNo)
	}
	if c.OldLineNo != nil {
		t.Errorf("old_line_no = %v", c.OldLineNo)
	}
	if c.Comment != "Handle empty case" {
		t.Errorf("comment = %q", c.Comment)
	}
	if c.DiffMode != "branch" {
		t.Errorf("diff_mode = %q", c.DiffMode)
	}
}

func TestSaveCreatesValidJSON(t *testing.T) {
	dir := t.TempDir()
	if err := Save(dir, nil, "main", "main"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".nit.json"))
	if err != nil {
		t.Fatal(err)
	}
	var nj map[string]interface{}
	if err := json.Unmarshal(data, &nj); err != nil {
		t.Fatal(err)
	}
	if nj["version"].(float64) != 1 {
		t.Errorf("version = %v", nj["version"])
	}
	if nj["branch"].(string) != "main" {
		t.Errorf("branch = %v", nj["branch"])
	}
}

func TestCommentMatchesLineNewLine(t *testing.T) {
	c := models.ReviewComment{NewLineNo: intPtr(10)}
	dl := models.DiffLine{LineType: models.LineAdd, NewLineNo: 10}
	if !MatchesLine(c, dl) {
		t.Error("should match")
	}
}

func TestCommentMatchesLineOldLineOnly(t *testing.T) {
	c := models.ReviewComment{OldLineNo: intPtr(5)}
	dl := models.DiffLine{LineType: models.LineRemove, OldLineNo: 5, NewLineNo: 0}
	if !MatchesLine(c, dl) {
		t.Error("should match")
	}
}

func TestCommentNoMatch(t *testing.T) {
	c := models.ReviewComment{NewLineNo: intPtr(10)}
	dl := models.DiffLine{LineType: models.LineAdd, NewLineNo: 99}
	if MatchesLine(c, dl) {
		t.Error("should not match")
	}
}

func TestMakeComment(t *testing.T) {
	dl := models.DiffLine{Content: "code", LineType: models.LineAdd, NewLineNo: 5}
	c := MakeComment("file.py", dl, "looks wrong", []string{"ctx1", "code", "ctx2"}, "unstaged")
	if c.FilePath != "file.py" {
		t.Errorf("file_path = %q", c.FilePath)
	}
	if c.NewLineNo == nil || *c.NewLineNo != 5 {
		t.Errorf("new_line_no = %v", c.NewLineNo)
	}
	if c.Comment != "looks wrong" {
		t.Errorf("comment = %q", c.Comment)
	}
	if c.DiffMode != "unstaged" {
		t.Errorf("diff_mode = %q", c.DiffMode)
	}
	if c.Timestamp == "" {
		t.Error("timestamp is empty")
	}
}

func TestAtomicWriteNoLeftoverTemp(t *testing.T) {
	dir := t.TempDir()
	if err := Save(dir, nil, "main", "main"); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".nit.tmp") {
			t.Errorf("leftover temp file: %s", e.Name())
		}
	}
	found := false
	for _, e := range entries {
		if e.Name() == ".nit.json" {
			found = true
		}
	}
	if !found {
		t.Error(".nit.json not found")
	}
}

func TestLoadCorruptedJSON(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".nit.json"), []byte("not valid json{{{"), 0644)
	got := Load(dir)
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestLoadWrongType(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".nit.json"), []byte(`"just a string"`), 0644)
	got := Load(dir)
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestLoadMissingCommentFields(t *testing.T) {
	dir := t.TempDir()
	data := `{"comments": [{"file": "x.py", "comment": "ok"}]}`
	os.WriteFile(filepath.Join(dir, ".nit.json"), []byte(data), 0644)
	loaded := Load(dir)
	if len(loaded) != 1 {
		t.Fatalf("expected 1, got %d", len(loaded))
	}
	if loaded[0].FilePath != "x.py" {
		t.Errorf("file = %q", loaded[0].FilePath)
	}
}

func TestExportMarkdownEmpty(t *testing.T) {
	if got := ExportMarkdown(nil); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestExportMarkdown(t *testing.T) {
	comments := []models.ReviewComment{
		{FilePath: "b.py", NewLineNo: intPtr(5), Comment: "second file", LineContent: "code"},
		{FilePath: "a.py", NewLineNo: intPtr(10), Comment: "note here", LineContent: "return result"},
		{FilePath: "a.py", OldLineNo: intPtr(3), Comment: "old line comment", LineContent: "old code"},
	}
	md := ExportMarkdown(comments)
	if !strings.HasPrefix(md, "# Code Review Comments") {
		t.Error("missing header")
	}
	aIdx := strings.Index(md, "## a.py")
	bIdx := strings.Index(md, "## b.py")
	if aIdx < 0 || bIdx < 0 || aIdx > bIdx {
		t.Error("files not sorted")
	}
	if !strings.Contains(md, "**L10**: note here") {
		t.Error("missing L10 comment")
	}
	if !strings.Contains(md, "**old L3**: old line comment") {
		t.Error("missing old L3 comment")
	}
	if !strings.Contains(md, "**L5**: second file") {
		t.Error("missing L5 comment")
	}
	if !strings.Contains(md, "```") {
		t.Error("missing code fence")
	}
}

func TestExportJSON(t *testing.T) {
	comments := []models.ReviewComment{
		{
			FilePath:    "src/app.py",
			NewLineNo:   intPtr(10),
			Comment:     "test comment",
			LineContent: "return result",
			HunkContext: []string{"line1"},
			Timestamp:   "2025-01-15T10:00:00+00:00",
			DiffMode:    "branch",
		},
	}
	result := ExportJSON(comments)
	var parsed []map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatal(err)
	}
	if len(parsed) != 1 {
		t.Fatalf("expected 1, got %d", len(parsed))
	}
	if parsed[0]["comment"].(string) != "test comment" {
		t.Errorf("comment = %v", parsed[0]["comment"])
	}
	if parsed[0]["file"].(string) != "src/app.py" {
		t.Errorf("file = %v", parsed[0]["file"])
	}
	if parsed[0]["line"].(float64) != 10 {
		t.Errorf("line = %v", parsed[0]["line"])
	}
}
