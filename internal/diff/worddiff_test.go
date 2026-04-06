package diff

import (
	"testing"
)

func assertSegments(t *testing.T, name string, got []DiffSegment, expected []DiffSegment) {
	t.Helper()
	if len(got) != len(expected) {
		t.Errorf("%s: len = %d, want %d\ngot:  %v\nwant: %v", name, len(got), len(expected), got, expected)
		return
	}
	for i := range got {
		if got[i].Text != expected[i].Text || got[i].Changed != expected[i].Changed {
			t.Errorf("%s[%d]: got %v, want %v", name, i, got[i], expected[i])
		}
	}
}

func TestWordDiffIdentical(t *testing.T) {
	oldSegs, newSegs := WordDiffSegments("hello world", "hello world")
	assertSegments(t, "old", oldSegs, []DiffSegment{{Text: "hello world", Changed: false}})
	assertSegments(t, "new", newSegs, []DiffSegment{{Text: "hello world", Changed: false}})
}

func TestWordDiffSingleWordChange(t *testing.T) {
	oldSegs, newSegs := WordDiffSegments("hello world", "hello earth")
	assertSegments(t, "old", oldSegs, []DiffSegment{
		{Text: "hello ", Changed: false},
		{Text: "world", Changed: true},
	})
	assertSegments(t, "new", newSegs, []DiffSegment{
		{Text: "hello ", Changed: false},
		{Text: "earth", Changed: true},
	})
}

func TestWordDiffInsertion(t *testing.T) {
	oldSegs, newSegs := WordDiffSegments("a b", "a x b")
	assertSegments(t, "old", oldSegs, []DiffSegment{
		{Text: "a ", Changed: false},
		{Text: "b", Changed: false},
	})
	assertSegments(t, "new", newSegs, []DiffSegment{
		{Text: "a ", Changed: false},
		{Text: "x ", Changed: true},
		{Text: "b", Changed: false},
	})
}

func TestWordDiffDeletion(t *testing.T) {
	oldSegs, newSegs := WordDiffSegments("a x b", "a b")
	assertSegments(t, "old", oldSegs, []DiffSegment{
		{Text: "a ", Changed: false},
		{Text: "x ", Changed: true},
		{Text: "b", Changed: false},
	})
	assertSegments(t, "new", newSegs, []DiffSegment{
		{Text: "a ", Changed: false},
		{Text: "b", Changed: false},
	})
}

func TestWordDiffCompletelyDifferent(t *testing.T) {
	oldSegs, newSegs := WordDiffSegments("aaa", "bbb")
	assertSegments(t, "old", oldSegs, []DiffSegment{{Text: "aaa", Changed: true}})
	assertSegments(t, "new", newSegs, []DiffSegment{{Text: "bbb", Changed: true}})
}
