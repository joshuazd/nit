package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseTargetEmpty(t *testing.T) {
	fp, cr := ParseTarget("")
	if fp != "" || cr != "" {
		t.Errorf("got %q, %q", fp, cr)
	}
}

func TestParseTargetCommitRange(t *testing.T) {
	fp, cr := ParseTarget("HEAD~3..HEAD")
	if fp != "" {
		t.Errorf("file_path = %q", fp)
	}
	if cr != "HEAD~3..HEAD" {
		t.Errorf("commit_range = %q", cr)
	}
}

func TestParseTargetBranchRange(t *testing.T) {
	fp, cr := ParseTarget("main..feature")
	if fp != "" {
		t.Errorf("file_path = %q", fp)
	}
	if cr != "main..feature" {
		t.Errorf("commit_range = %q", cr)
	}
}

func TestParseTargetExistingFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.txt")
	os.WriteFile(f, []byte("hello"), 0644)

	fp, cr := ParseTarget(f)
	if fp != f {
		t.Errorf("file_path = %q", fp)
	}
	if cr != "" {
		t.Errorf("commit_range = %q", cr)
	}
}

func TestParseTargetExistingDir(t *testing.T) {
	dir := t.TempDir()
	fp, cr := ParseTarget(dir)
	if fp != dir {
		t.Errorf("file_path = %q", fp)
	}
	if cr != "" {
		t.Errorf("commit_range = %q", cr)
	}
}
