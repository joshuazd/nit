package diff

import (
	"strings"
	"testing"

	"github.com/joshuazd/nit/internal/models"
)

func TestBuildPatchModified(t *testing.T) {
	hunk := models.DiffHunk{
		Header:   "@@ -1,3 +1,3 @@",
		OldStart: 1,
		NewStart: 1,
		Lines: []models.DiffLine{
			{Content: "", LineType: models.LineHunkHeader, Raw: "@@ -1,3 +1,3 @@"},
			{Content: "a", LineType: models.LineContext, OldLineNo: 1, NewLineNo: 1, Raw: " a"},
			{Content: "old", LineType: models.LineRemove, OldLineNo: 2, Raw: "-old"},
			{Content: "new", LineType: models.LineAdd, NewLineNo: 2, Raw: "+new"},
			{Content: "c", LineType: models.LineContext, OldLineNo: 3, NewLineNo: 3, Raw: " c"},
		},
	}
	fd := models.FileDiff{Path: "foo.py", Status: models.StatusModified}
	patch := BuildPatch(&fd, &hunk)

	if !strings.HasPrefix(patch, "diff --git a/foo.py b/foo.py\n") {
		t.Errorf("bad header: %s", patch)
	}
	if !strings.Contains(patch, "--- a/foo.py\n") {
		t.Error("missing --- a/")
	}
	if !strings.Contains(patch, "+++ b/foo.py\n") {
		t.Error("missing +++ b/")
	}
	if !strings.Contains(patch, "@@ -1,3 +1,3 @@\n") {
		t.Error("missing hunk header")
	}
	if !strings.Contains(patch, " a\n") {
		t.Error("missing context line")
	}
	if !strings.Contains(patch, "-old\n") {
		t.Error("missing remove line")
	}
	if !strings.Contains(patch, "+new\n") {
		t.Error("missing add line")
	}
}

func TestBuildPatchAddedFile(t *testing.T) {
	hunk := models.DiffHunk{
		Header: "@@ -0,0 +1,1 @@",
		Lines: []models.DiffLine{
			{Content: "", LineType: models.LineHunkHeader, Raw: "@@ -0,0 +1,1 @@"},
			{Content: "hello", LineType: models.LineAdd, NewLineNo: 1, Raw: "+hello"},
		},
	}
	fd := models.FileDiff{Path: "new.py", Status: models.StatusAdded}
	patch := BuildPatch(&fd, &hunk)

	if !strings.Contains(patch, "--- /dev/null\n") {
		t.Error("missing --- /dev/null")
	}
	if !strings.Contains(patch, "+++ b/new.py\n") {
		t.Error("missing +++ b/new.py")
	}
}

func TestBuildPatchDeletedFile(t *testing.T) {
	hunk := models.DiffHunk{
		Header: "@@ -1,1 +0,0 @@",
		Lines: []models.DiffLine{
			{Content: "", LineType: models.LineHunkHeader, Raw: "@@ -1,1 +0,0 @@"},
			{Content: "bye", LineType: models.LineRemove, OldLineNo: 1, Raw: "-bye"},
		},
	}
	fd := models.FileDiff{Path: "old.py", Status: models.StatusDeleted}
	patch := BuildPatch(&fd, &hunk)

	if !strings.Contains(patch, "--- a/old.py\n") {
		t.Error("missing --- a/old.py")
	}
	if !strings.Contains(patch, "+++ /dev/null\n") {
		t.Error("missing +++ /dev/null")
	}
}
