package diff

import (
	"testing"

	"github.com/joshuazd/nit/internal/models"
)

func TestEmptyInput(t *testing.T) {
	if files := ParseDiff(""); files != nil {
		t.Errorf("expected nil, got %v", files)
	}
	if files := ParseDiff("   \n  "); files != nil {
		t.Errorf("expected nil, got %v", files)
	}
}

func TestSingleModifiedFile(t *testing.T) {
	diff := `diff --git a/foo.py b/foo.py
index abc1234..def5678 100644
--- a/foo.py
+++ b/foo.py
@@ -1,3 +1,4 @@
 line1
-old line
+new line
+added line
 line3
`
	files := ParseDiff(diff)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	f := files[0]
	if f.Path != "foo.py" {
		t.Errorf("path = %q", f.Path)
	}
	if f.Status != models.StatusModified {
		t.Errorf("status = %v", f.Status)
	}
	if len(f.Hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(f.Hunks))
	}

	lines := f.Hunks[0].Lines
	// hunk_header, context, remove, add, add, context
	assertLineType(t, lines[0], models.LineHunkHeader)
	assertLineType(t, lines[1], models.LineContext)
	if lines[1].Content != "line1" {
		t.Errorf("content = %q", lines[1].Content)
	}
	if lines[1].OldLineNo != 1 || lines[1].NewLineNo != 1 {
		t.Errorf("line nos = %d, %d", lines[1].OldLineNo, lines[1].NewLineNo)
	}

	assertLineType(t, lines[2], models.LineRemove)
	if lines[2].Content != "old line" {
		t.Errorf("content = %q", lines[2].Content)
	}
	if lines[2].OldLineNo != 2 || lines[2].NewLineNo != 0 {
		t.Errorf("line nos = %d, %d", lines[2].OldLineNo, lines[2].NewLineNo)
	}

	assertLineType(t, lines[3], models.LineAdd)
	if lines[3].Content != "new line" {
		t.Errorf("content = %q", lines[3].Content)
	}
	if lines[3].NewLineNo != 2 || lines[3].OldLineNo != 0 {
		t.Errorf("line nos = %d, %d", lines[3].OldLineNo, lines[3].NewLineNo)
	}

	assertLineType(t, lines[4], models.LineAdd)
	if lines[4].Content != "added line" {
		t.Errorf("content = %q", lines[4].Content)
	}
	if lines[4].NewLineNo != 3 {
		t.Errorf("new_line_no = %d", lines[4].NewLineNo)
	}

	assertLineType(t, lines[5], models.LineContext)
	if lines[5].OldLineNo != 3 || lines[5].NewLineNo != 4 {
		t.Errorf("line nos = %d, %d", lines[5].OldLineNo, lines[5].NewLineNo)
	}
}

func TestAddedFile(t *testing.T) {
	diff := `diff --git a/new.py b/new.py
new file mode 100644
index 0000000..abc1234
--- /dev/null
+++ b/new.py
@@ -0,0 +1,2 @@
+line1
+line2
`
	files := ParseDiff(diff)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].Status != models.StatusAdded {
		t.Errorf("status = %v", files[0].Status)
	}
	if files[0].Path != "new.py" {
		t.Errorf("path = %q", files[0].Path)
	}
}

func TestDeletedFile(t *testing.T) {
	diff := `diff --git a/old.py b/old.py
deleted file mode 100644
index abc1234..0000000
--- a/old.py
+++ /dev/null
@@ -1,2 +0,0 @@
-line1
-line2
`
	files := ParseDiff(diff)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].Status != models.StatusDeleted {
		t.Errorf("status = %v", files[0].Status)
	}
}

func TestRenamedFile(t *testing.T) {
	diff := `diff --git a/old_name.py b/new_name.py
similarity index 100%
rename from old_name.py
rename to new_name.py
`
	files := ParseDiff(diff)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].Status != models.StatusRenamed {
		t.Errorf("status = %v", files[0].Status)
	}
	if files[0].Path != "new_name.py" {
		t.Errorf("path = %q", files[0].Path)
	}
	if files[0].OldPath != "old_name.py" {
		t.Errorf("old_path = %q", files[0].OldPath)
	}
}

func TestBinaryFile(t *testing.T) {
	diff := `diff --git a/image.png b/image.png
Binary files a/image.png and b/image.png differ
`
	files := ParseDiff(diff)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if !files[0].IsBinary {
		t.Errorf("expected binary")
	}
}

func TestMultipleHunks(t *testing.T) {
	diff := `diff --git a/foo.py b/foo.py
index abc..def 100644
--- a/foo.py
+++ b/foo.py
@@ -1,3 +1,3 @@
 a
-b
+B
 c
@@ -10,3 +10,3 @@
 x
-y
+Y
 z
`
	files := ParseDiff(diff)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if len(files[0].Hunks) != 2 {
		t.Fatalf("expected 2 hunks, got %d", len(files[0].Hunks))
	}
	if files[0].Hunks[0].OldStart != 1 {
		t.Errorf("hunk[0].OldStart = %d", files[0].Hunks[0].OldStart)
	}
	if files[0].Hunks[1].OldStart != 10 {
		t.Errorf("hunk[1].OldStart = %d", files[0].Hunks[1].OldStart)
	}
}

func TestMultipleFiles(t *testing.T) {
	diff := `diff --git a/a.py b/a.py
index abc..def 100644
--- a/a.py
+++ b/a.py
@@ -1,1 +1,1 @@
-old
+new
diff --git a/b.py b/b.py
new file mode 100644
index 0000000..abc1234
--- /dev/null
+++ b/b.py
@@ -0,0 +1 @@
+content
`
	files := ParseDiff(diff)
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
	if files[0].Path != "a.py" {
		t.Errorf("files[0].path = %q", files[0].Path)
	}
	if files[1].Path != "b.py" {
		t.Errorf("files[1].path = %q", files[1].Path)
	}
	if files[1].Status != models.StatusAdded {
		t.Errorf("files[1].status = %v", files[1].Status)
	}
}

func TestNoNewlineAtEOF(t *testing.T) {
	diff := `diff --git a/foo.py b/foo.py
index abc..def 100644
--- a/foo.py
+++ b/foo.py
@@ -1,1 +1,1 @@
-old
\ No newline at end of file
+new
\ No newline at end of file
`
	files := ParseDiff(diff)
	lines := files[0].Hunks[0].Lines
	for _, dl := range lines {
		if dl.LineType == models.LineContext && dl.Content == ` No newline at end of file` {
			t.Error("backslash line should be skipped")
		}
	}
}

func TestFileToDiff(t *testing.T) {
	content := "line1\nline2\nline3\n"
	files := FileToDiff("test.py", content)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	f := files[0]
	if f.Path != "test.py" {
		t.Errorf("path = %q", f.Path)
	}
	if len(f.Hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(f.Hunks))
	}
	// hunk header + 3 context lines
	if len(f.Hunks[0].Lines) != 4 {
		t.Errorf("expected 4 lines, got %d", len(f.Hunks[0].Lines))
	}
	for i, dl := range f.Hunks[0].Lines[1:] {
		if dl.LineType != models.LineContext {
			t.Errorf("line %d: type = %v", i+1, dl.LineType)
		}
		if dl.OldLineNo != i+1 || dl.NewLineNo != i+1 {
			t.Errorf("line %d: line nos = %d, %d", i+1, dl.OldLineNo, dl.NewLineNo)
		}
	}
}

func assertLineType(t *testing.T, dl models.DiffLine, expected models.LineType) {
	t.Helper()
	if dl.LineType != expected {
		t.Errorf("expected line type %v, got %v (content=%q)", expected, dl.LineType, dl.Content)
	}
}
