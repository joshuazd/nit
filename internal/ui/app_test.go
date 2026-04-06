package ui

import (
	"testing"

	"github.com/joshuazd/nit/internal/cli"
	"github.com/joshuazd/nit/internal/models"
)

func TestNewModel(t *testing.T) {
	args := cli.Args{Mode: "branch"}
	m := NewModel(args)
	if m.diffMode != "branch" {
		t.Errorf("diffMode = %q", m.diffMode)
	}
	if m.input.IsActive() {
		t.Error("input should not be active")
	}
}

func TestCycleMode(t *testing.T) {
	m := NewModel(cli.Args{})
	m.diffMode = "branch"
	m.repoRoot = "/tmp/test" // fake
	// We can't actually call cycleMode since it calls git, but test the mode list
	for i, mode := range DiffModes {
		if DiffModes[(i+1)%len(DiffModes)] == "" {
			t.Errorf("empty mode at index %d", i+1)
		}
		_ = mode
	}
}

func TestDiffViewModel(t *testing.T) {
	dv := NewDiffViewModel()
	dv.Height = 20
	dv.Width = 80

	fd := models.FileDiff{
		Path:   "test.py",
		Status: models.StatusModified,
		Hunks: []models.DiffHunk{
			{
				Header:   "@@ -1,3 +1,3 @@",
				OldStart: 1,
				NewStart: 1,
				Lines: []models.DiffLine{
					{Content: "", LineType: models.LineHunkHeader, Raw: "@@ -1,3 +1,3 @@"},
					{Content: "line1", LineType: models.LineContext, OldLineNo: 1, NewLineNo: 1, Raw: " line1"},
					{Content: "old", LineType: models.LineRemove, OldLineNo: 2, Raw: "-old"},
					{Content: "new", LineType: models.LineAdd, NewLineNo: 2, Raw: "+new"},
					{Content: "line3", LineType: models.LineContext, OldLineNo: 3, NewLineNo: 3, Raw: " line3"},
				},
			},
		},
	}

	dv.LoadFileDiff(&fd, nil, 0)
	if len(dv.DiffLines) != 5 {
		t.Fatalf("expected 5 diff lines, got %d", len(dv.DiffLines))
	}
	if dv.CursorIndex != 0 {
		t.Errorf("cursor = %d", dv.CursorIndex)
	}

	// Test cursor movement
	dv.MoveCursor(2)
	if dv.CursorIndex != 2 {
		t.Errorf("cursor = %d after move +2", dv.CursorIndex)
	}

	// Test clamping
	dv.MoveCursor(-100)
	if dv.CursorIndex != 0 {
		t.Errorf("cursor = %d after move -100", dv.CursorIndex)
	}
	dv.MoveCursor(100)
	if dv.CursorIndex != 4 {
		t.Errorf("cursor = %d after move +100", dv.CursorIndex)
	}

	// Test hunk jumping
	dv.CursorIndex = 2
	dv.JumpToNextHunk(false)
	if dv.CursorIndex != 0 {
		t.Errorf("cursor = %d after jump prev hunk", dv.CursorIndex)
	}

	// Test render
	rendered := dv.Render()
	if rendered == "" {
		t.Error("empty render")
	}
}

func TestDiffViewSideBySide(t *testing.T) {
	dv := NewDiffViewModel()
	dv.Height = 20
	dv.Width = 120
	dv.SideBySide = true

	fd := models.FileDiff{
		Path:   "test.py",
		Status: models.StatusModified,
		Hunks: []models.DiffHunk{
			{
				Header:   "@@ -1,2 +1,2 @@",
				OldStart: 1,
				NewStart: 1,
				Lines: []models.DiffLine{
					{Content: "", LineType: models.LineHunkHeader, Raw: "@@ -1,2 +1,2 @@"},
					{Content: "old", LineType: models.LineRemove, OldLineNo: 1, Raw: "-old"},
					{Content: "new", LineType: models.LineAdd, NewLineNo: 1, Raw: "+new"},
				},
			},
		},
	}

	dv.LoadFileDiff(&fd, nil, 0)
	if len(dv.DiffLines) != 2 { // header + 1 change row
		t.Fatalf("expected 2 lines, got %d", len(dv.DiffLines))
	}

	rendered := dv.Render()
	if rendered == "" {
		t.Error("empty render")
	}
}

func TestFileTreeModel(t *testing.T) {
	ft := NewFileTreeModel()
	files := []models.FileDiff{
		{Path: "src/app.py", Status: models.StatusModified},
		{Path: "src/git.py", Status: models.StatusAdded},
		{Path: "README.md", Status: models.StatusModified},
	}
	cc := map[string]int{"src/app.py": 2}

	ft.Height = 20
	ft.Width = 38
	ft.Update(files, cc)

	if len(ft.TreeOrder) != 3 {
		t.Fatalf("tree order len = %d", len(ft.TreeOrder))
	}

	// Test navigation
	idx := ft.NextFile()
	if idx < 0 {
		t.Error("NextFile returned -1")
	}

	rendered := ft.Render()
	if rendered == "" {
		t.Error("empty render")
	}
}

func TestInputModel(t *testing.T) {
	im := NewInputModel()
	if im.IsActive() {
		t.Error("should not be active")
	}

	im.StartComment()
	if !im.IsActive() {
		t.Error("should be active after StartComment")
	}
	if im.Mode != InputComment {
		t.Error("mode should be InputComment")
	}

	im.Cancel()
	if im.IsActive() {
		t.Error("should not be active after Cancel")
	}

	im.StartCommit()
	if im.Mode != InputCommit {
		t.Error("mode should be InputCommit")
	}
	val := im.Submit()
	if val != "" {
		t.Errorf("submit val = %q", val)
	}
	if im.IsActive() {
		t.Error("should not be active after Submit")
	}
}
