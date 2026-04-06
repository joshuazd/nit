package diff

import (
	"testing"

	"github.com/joshuazd/nit/internal/models"
)

func dl(content string, lineType models.LineType, oldNo, newNo int) models.DiffLine {
	return models.DiffLine{Content: content, LineType: lineType, OldLineNo: oldNo, NewLineNo: newNo}
}

func TestAlignPureAdds(t *testing.T) {
	lines := []models.DiffLine{
		dl("", models.LineHunkHeader, 0, 0),
		dl("a", models.LineAdd, 0, 1),
		dl("b", models.LineAdd, 0, 2),
	}
	rows := AlignHunkLines(lines)
	if rows[0].RowType != models.RowHunkHeader {
		t.Error("expected hunk_header")
	}
	if rows[1].Left != nil {
		t.Error("expected left nil")
	}
	if rows[1].Right.Content != "a" {
		t.Errorf("right = %q", rows[1].Right.Content)
	}
	if rows[2].Left != nil {
		t.Error("expected left nil")
	}
	if rows[2].Right.Content != "b" {
		t.Errorf("right = %q", rows[2].Right.Content)
	}
}

func TestAlignPureRemoves(t *testing.T) {
	lines := []models.DiffLine{
		dl("", models.LineHunkHeader, 0, 0),
		dl("a", models.LineRemove, 1, 0),
		dl("b", models.LineRemove, 2, 0),
	}
	rows := AlignHunkLines(lines)
	if rows[1].Left.Content != "a" {
		t.Errorf("left = %q", rows[1].Left.Content)
	}
	if rows[1].Right != nil {
		t.Error("expected right nil")
	}
	if rows[2].Left.Content != "b" {
		t.Errorf("left = %q", rows[2].Left.Content)
	}
	if rows[2].Right != nil {
		t.Error("expected right nil")
	}
}

func TestAlignMatchedPairs(t *testing.T) {
	lines := []models.DiffLine{
		dl("", models.LineHunkHeader, 0, 0),
		dl("old", models.LineRemove, 1, 0),
		dl("new", models.LineAdd, 0, 1),
	}
	rows := AlignHunkLines(lines)
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[1].RowType != models.RowChange {
		t.Error("expected change")
	}
	if rows[1].Left.Content != "old" {
		t.Errorf("left = %q", rows[1].Left.Content)
	}
	if rows[1].Right.Content != "new" {
		t.Errorf("right = %q", rows[1].Right.Content)
	}
}

func TestAlignUnequalCounts(t *testing.T) {
	lines := []models.DiffLine{
		dl("", models.LineHunkHeader, 0, 0),
		dl("a", models.LineRemove, 1, 0),
		dl("b", models.LineRemove, 2, 0),
		dl("c", models.LineRemove, 3, 0),
		dl("x", models.LineAdd, 0, 1),
		dl("y", models.LineAdd, 0, 2),
	}
	rows := AlignHunkLines(lines)
	if len(rows) != 4 {
		t.Fatalf("expected 4 rows, got %d", len(rows))
	}
	if rows[1].Left.Content != "a" || rows[1].Right.Content != "x" {
		t.Errorf("row 1: %q / %q", rows[1].Left.Content, rows[1].Right.Content)
	}
	if rows[2].Left.Content != "b" || rows[2].Right.Content != "y" {
		t.Errorf("row 2: %q / %q", rows[2].Left.Content, rows[2].Right.Content)
	}
	if rows[3].Left.Content != "c" || rows[3].Right != nil {
		t.Error("row 3: expected left only")
	}
}

func TestAlignContextLines(t *testing.T) {
	lines := []models.DiffLine{
		dl("", models.LineHunkHeader, 0, 0),
		dl("ctx", models.LineContext, 1, 1),
	}
	rows := AlignHunkLines(lines)
	if rows[1].RowType != models.RowContext {
		t.Error("expected context")
	}
	if rows[1].Left.Content != "ctx" {
		t.Errorf("left = %q", rows[1].Left.Content)
	}
	if rows[1].Right.Content != "ctx" {
		t.Errorf("right = %q", rows[1].Right.Content)
	}
}

func TestAlignMixed(t *testing.T) {
	lines := []models.DiffLine{
		dl("", models.LineHunkHeader, 0, 0),
		dl("before", models.LineContext, 1, 1),
		dl("old", models.LineRemove, 2, 0),
		dl("new", models.LineAdd, 0, 2),
		dl("after", models.LineContext, 3, 3),
	}
	rows := AlignHunkLines(lines)
	if len(rows) != 4 {
		t.Fatalf("expected 4 rows, got %d", len(rows))
	}
	if rows[1].RowType != models.RowContext {
		t.Error("expected context")
	}
	if rows[2].RowType != models.RowChange {
		t.Error("expected change")
	}
	if rows[3].RowType != models.RowContext {
		t.Error("expected context")
	}
}
