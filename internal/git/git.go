package git

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const GitTimeout = 30 * time.Second

// run executes a git command with timeout and returns stdout.
func run(args []string, cwd string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), GitTimeout)
	defer cancel()
	return runCtx(ctx, args, cwd, "")
}

// runWithStdin executes a git command with stdin input.
func runWithStdin(args []string, cwd string, stdin string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), GitTimeout)
	defer cancel()
	return runCtx(ctx, args, cwd, stdin)
}

func runCtx(ctx context.Context, args []string, cwd string, stdin string) (string, error) {
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("git command timed out: %s", strings.Join(args, " "))
	}
	if err != nil {
		return "", fmt.Errorf("%s: %s", strings.Join(args, " "), strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// GetRepoRoot returns the absolute path to the git repository root.
func GetRepoRoot(cwd string) (string, error) {
	out, err := run([]string{"git", "rev-parse", "--show-toplevel"}, cwd)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// GetCurrentBranch returns the current branch name, or "" if detached.
func GetCurrentBranch(cwd string) (string, error) {
	out, err := run([]string{"git", "branch", "--show-current"}, cwd)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// GetMainBranch returns the main/master branch name, preferring remote refs.
func GetMainBranch(cwd string) string {
	for _, name := range []string{"main", "master"} {
		for _, ref := range []string{
			fmt.Sprintf("refs/remotes/origin/%s", name),
			fmt.Sprintf("refs/heads/%s", name),
		} {
			cmd := exec.Command("git", "rev-parse", "--verify", ref)
			if cwd != "" {
				cmd.Dir = cwd
			}
			cmd.Stdout = nil
			cmd.Stderr = nil
			if cmd.Run() == nil {
				if strings.HasPrefix(ref, "refs/remotes") {
					return "origin/" + name
				}
				return name
			}
		}
	}
	return "main"
}

// GetMergeBase returns the merge-base commit SHA between base and HEAD.
func GetMergeBase(base, cwd string) (string, error) {
	out, err := run([]string{"git", "merge-base", base, "HEAD"}, cwd)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func appendWhitespaceFlag(cmd []string, ignoreWhitespace bool) []string {
	if ignoreWhitespace {
		return append(cmd, "-w")
	}
	return cmd
}

func appendPathFilter(cmd []string, pathFilter string) []string {
	if pathFilter != "" {
		return append(cmd, "--", pathFilter)
	}
	return cmd
}

func buildDiffCmd(baseCmd []string, ignoreWhitespace bool, pathFilter string) []string {
	cmd := appendWhitespaceFlag(baseCmd, ignoreWhitespace)
	return appendPathFilter(cmd, pathFilter)
}

// GetBranchDiff returns the diff between the main branch and HEAD.
func GetBranchDiff(cwd, pathFilter string, ignoreWhitespace bool) (string, error) {
	base := GetMainBranch(cwd)
	return GetBranchDiffWithBase(base, cwd, pathFilter, ignoreWhitespace)
}

// GetBranchDiffWithBase returns the diff using a pre-resolved base branch.
func GetBranchDiffWithBase(base, cwd, pathFilter string, ignoreWhitespace bool) (string, error) {
	cmd := buildDiffCmd([]string{"git", "diff", base + "...HEAD"}, ignoreWhitespace, pathFilter)
	return run(cmd, cwd)
}

// GetDiffStat returns a compact stat summary for change detection.
// Much cheaper than full diff output — used for auto-refresh debouncing.
func GetDiffStat(cwd string, args ...string) (string, error) {
	cmd := append([]string{"git", "diff", "--stat"}, args...)
	return run(cmd, cwd)
}

// GetUnstagedDiff returns the diff of unstaged changes.
func GetUnstagedDiff(cwd, pathFilter string, ignoreWhitespace bool) (string, error) {
	cmd := buildDiffCmd([]string{"git", "diff"}, ignoreWhitespace, pathFilter)
	return run(cmd, cwd)
}

// GetStagedDiff returns the diff of staged changes.
func GetStagedDiff(cwd, pathFilter string, ignoreWhitespace bool) (string, error) {
	cmd := buildDiffCmd([]string{"git", "diff", "--cached"}, ignoreWhitespace, pathFilter)
	return run(cmd, cwd)
}

// GetUpstreamRef returns the upstream tracking ref, or "" if not set.
func GetUpstreamRef(cwd string) string {
	out, err := run([]string{"git", "rev-parse", "--abbrev-ref", "@{upstream}"}, cwd)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// GetUnpushedDiff returns the diff between the upstream tracking branch and HEAD.
func GetUnpushedDiff(cwd, pathFilter string, ignoreWhitespace bool) (string, error) {
	upstream := GetUpstreamRef(cwd)
	if upstream == "" {
		return "", fmt.Errorf("no upstream branch set")
	}
	cmd := buildDiffCmd([]string{"git", "diff", upstream}, ignoreWhitespace, pathFilter)
	return run(cmd, cwd)
}

// GetCommitRangeDiff returns the diff for an arbitrary commit range.
func GetCommitRangeDiff(commitRange, cwd, pathFilter string, ignoreWhitespace bool) (string, error) {
	cmd := buildDiffCmd([]string{"git", "diff", commitRange}, ignoreWhitespace, pathFilter)
	return run(cmd, cwd)
}

// ApplyPatch applies a patch via git apply.
func ApplyPatch(patchText, cwd string, cached, reverse bool) (string, error) {
	cmd := []string{"git", "apply"}
	if cached {
		cmd = append(cmd, "--cached")
	}
	if reverse {
		cmd = append(cmd, "--reverse")
	}
	return runWithStdin(cmd, cwd, patchText)
}

// Commit creates a commit with the given message.
func Commit(message, cwd string) (string, error) {
	return run([]string{"git", "commit", "-m", message}, cwd)
}

// GetUntrackedFiles returns a list of untracked file paths.
func GetUntrackedFiles(cwd string) ([]string, error) {
	out, err := run([]string{"git", "ls-files", "--others", "--exclude-standard"}, cwd)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, f := range strings.Split(strings.TrimSpace(out), "\n") {
		if f != "" {
			files = append(files, f)
		}
	}
	return files, nil
}

// isBinaryFile checks if a file appears to be binary by looking for null bytes.
func isBinaryFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	buf := make([]byte, 8192)
	n, _ := f.Read(buf)
	return bytes.Contains(buf[:n], []byte{0})
}

// untrackedFileDiff generates a unified diff for an untracked file as if newly added.
func untrackedFileDiff(path, cwd string) string {
	dir := cwd
	if dir == "" {
		dir, _ = os.Getwd()
	}
	fullPath := filepath.Join(dir, path)

	// Skip binary files — emit a marker instead of reading content
	if isBinaryFile(fullPath) {
		return fmt.Sprintf("diff --git a/%s b/%s\nBinary files /dev/null and b/%s differ\n", path, path, path)
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return ""
	}
	content := string(data)
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	n := len(lines)

	var b strings.Builder
	fmt.Fprintf(&b, "diff --git a/%s b/%s\n", path, path)
	b.WriteString("new file mode 100644\n")
	b.WriteString("--- /dev/null\n")
	fmt.Fprintf(&b, "+++ b/%s\n", path)
	fmt.Fprintf(&b, "@@ -0,0 +1,%d @@\n", n)
	for _, line := range lines {
		fmt.Fprintf(&b, "+%s\n", line)
	}
	return b.String()
}

// GetUntrackedDiff returns synthetic diff text for all untracked files.
func GetUntrackedDiff(cwd, pathFilter string) (string, error) {
	files, err := GetUntrackedFiles(cwd)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, f := range files {
		if pathFilter != "" && !strings.HasPrefix(f, pathFilter) && f != pathFilter {
			continue
		}
		b.WriteString(untrackedFileDiff(f, cwd))
	}
	return b.String(), nil
}
