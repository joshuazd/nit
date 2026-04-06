package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func setupGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmds := [][]string{
		{"git", "init"},
		{"git", "checkout", "-b", "main"},
		{"git", "config", "user.name", "test"},
		{"git", "config", "user.email", "t@t"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %s", args, out)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("hello\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "init")
	return dir
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %s", args, out)
	}
	return string(out)
}

func TestGetRepoRoot(t *testing.T) {
	dir := setupGitRepo(t)
	root, err := GetRepoRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Resolve symlinks for macOS /private/tmp
	expected, _ := filepath.EvalSymlinks(dir)
	got, _ := filepath.EvalSymlinks(root)
	if got != expected {
		t.Errorf("root = %q, want %q", got, expected)
	}
}

func TestGetCurrentBranch(t *testing.T) {
	dir := setupGitRepo(t)
	branch, err := GetCurrentBranch(dir)
	if err != nil {
		t.Fatal(err)
	}
	if branch != "main" {
		t.Errorf("branch = %q", branch)
	}
}

func TestGetMainBranchLocalOnly(t *testing.T) {
	dir := setupGitRepo(t)
	if got := GetMainBranch(dir); got != "main" {
		t.Errorf("got %q", got)
	}
}

func TestGetMainBranchPrefersRemote(t *testing.T) {
	dir := setupGitRepo(t)
	runGit(t, dir, "remote", "add", "origin", "https://example.com/repo.git")
	runGit(t, dir, "update-ref", "refs/remotes/origin/main", "HEAD")
	if got := GetMainBranch(dir); got != "origin/main" {
		t.Errorf("got %q", got)
	}
}

func TestGetUnstagedDiffEmpty(t *testing.T) {
	dir := setupGitRepo(t)
	diff, err := GetUnstagedDiff(dir, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if diff != "" {
		t.Errorf("expected empty, got %q", diff)
	}
}

func TestGetUnstagedDiffWithChanges(t *testing.T) {
	dir := setupGitRepo(t)
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("changed\n"), 0644)
	diff, err := GetUnstagedDiff(dir, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(diff, "changed") {
		t.Errorf("diff = %q", diff)
	}
}

func TestNotARepo(t *testing.T) {
	dir := t.TempDir()
	_, err := GetRepoRoot(dir)
	if err == nil {
		t.Error("expected error")
	}
}

func TestBadCommitRange(t *testing.T) {
	dir := setupGitRepo(t)
	_, err := GetCommitRangeDiff("nonexistent..refs", dir, "", false)
	if err == nil {
		t.Error("expected error")
	}
}

func TestDetachedHead(t *testing.T) {
	// Use manual temp dir — t.TempDir cleanup races with git on macOS
	dir, err := os.MkdirTemp("", "TestDetachedHead")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	cmds := [][]string{
		{"git", "init"},
		{"git", "checkout", "-b", "main"},
		{"git", "config", "user.name", "test"},
		{"git", "config", "user.email", "t@t"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.CombinedOutput()
	}
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("hello\n"), 0644)
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "init")

	sha := strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
	runGit(t, dir, "checkout", sha)
	branch, err2 := GetCurrentBranch(dir)
	if err2 != nil {
		t.Fatal(err2)
	}
	if branch != "" {
		t.Errorf("expected empty, got %q", branch)
	}
}

func TestGetStagedDiffEmpty(t *testing.T) {
	dir := setupGitRepo(t)
	diff, err := GetStagedDiff(dir, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if diff != "" {
		t.Errorf("expected empty, got %q", diff)
	}
}

func TestGetStagedDiffWithStagedChanges(t *testing.T) {
	dir := setupGitRepo(t)
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("staged content\n"), 0644)
	runGit(t, dir, "add", "file.txt")
	diff, err := GetStagedDiff(dir, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(diff, "staged content") {
		t.Errorf("diff = %q", diff)
	}
}

func TestApplyPatchStageHunk(t *testing.T) {
	dir := setupGitRepo(t)
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("changed\n"), 0644)
	diff, err := GetUnstagedDiff(dir, "", false)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ApplyPatch(diff, dir, true, false)
	if err != nil {
		t.Fatal(err)
	}
	staged, _ := GetStagedDiff(dir, "", false)
	if !strings.Contains(staged, "changed") {
		t.Error("not staged")
	}
	unstaged, _ := GetUnstagedDiff(dir, "", false)
	if unstaged != "" {
		t.Error("still unstaged")
	}
}

func TestApplyPatchUnstageHunk(t *testing.T) {
	dir := setupGitRepo(t)
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("changed\n"), 0644)
	runGit(t, dir, "add", "file.txt")
	staged, _ := GetStagedDiff(dir, "", false)
	_, err := ApplyPatch(staged, dir, true, true)
	if err != nil {
		t.Fatal(err)
	}
	staged2, _ := GetStagedDiff(dir, "", false)
	if staged2 != "" {
		t.Error("still staged")
	}
	unstaged, _ := GetUnstagedDiff(dir, "", false)
	if !strings.Contains(unstaged, "changed") {
		t.Error("not unstaged")
	}
}

func TestApplyPatchDiscardHunk(t *testing.T) {
	dir := setupGitRepo(t)
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("changed\n"), 0644)
	diff, _ := GetUnstagedDiff(dir, "", false)
	_, err := ApplyPatch(diff, dir, false, true)
	if err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(filepath.Join(dir, "file.txt"))
	if string(content) != "hello\n" {
		t.Errorf("content = %q", content)
	}
}

func TestCommit(t *testing.T) {
	dir := setupGitRepo(t)
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("committed\n"), 0644)
	runGit(t, dir, "add", "file.txt")
	_, err := Commit("test commit", dir)
	if err != nil {
		t.Fatal(err)
	}
	log := runGit(t, dir, "log", "--oneline", "-1")
	if !strings.Contains(log, "test commit") {
		t.Errorf("log = %q", log)
	}
}
