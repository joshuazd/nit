import subprocess

import pytest

from nit import git


@pytest.fixture
def git_repo(tmp_path):
    """Create a temporary git repo with one commit."""
    subprocess.run(["git", "init"], cwd=tmp_path, capture_output=True)
    subprocess.run(["git", "checkout", "-b", "main"], cwd=tmp_path, capture_output=True)
    subprocess.run(["git", "config", "user.name", "test"], cwd=tmp_path, capture_output=True)
    subprocess.run(["git", "config", "user.email", "t@t"], cwd=tmp_path, capture_output=True)
    (tmp_path / "file.txt").write_text("hello\n")
    subprocess.run(["git", "add", "."], cwd=tmp_path, capture_output=True)
    subprocess.run(["git", "commit", "-m", "init"], cwd=tmp_path, capture_output=True)
    return tmp_path


def test_get_repo_root(git_repo):
    root = git.get_repo_root(cwd=git_repo)
    assert root.resolve() == git_repo.resolve()


def test_get_current_branch(git_repo):
    branch = git.get_current_branch(cwd=git_repo)
    assert branch == "main"


def test_get_main_branch_local_only(git_repo):
    assert git.get_main_branch(cwd=git_repo) == "main"


def test_get_main_branch_prefers_remote(git_repo):
    # Add a fake remote ref for origin/main
    subprocess.run(
        ["git", "remote", "add", "origin", "https://example.com/repo.git"],
        cwd=git_repo, capture_output=True,
    )
    subprocess.run(
        ["git", "update-ref", "refs/remotes/origin/main", "HEAD"],
        cwd=git_repo, capture_output=True,
    )
    assert git.get_main_branch(cwd=git_repo) == "origin/main"


def test_get_unstaged_diff_empty(git_repo):
    diff = git.get_unstaged_diff(cwd=git_repo)
    assert diff == ""


def test_get_unstaged_diff_with_changes(git_repo):
    (git_repo / "file.txt").write_text("changed\n")
    diff = git.get_unstaged_diff(cwd=git_repo)
    assert "changed" in diff


def test_not_a_repo(tmp_path):
    with pytest.raises(subprocess.CalledProcessError):
        git.get_repo_root(cwd=tmp_path)


def test_bad_commit_range(git_repo):
    with pytest.raises(subprocess.CalledProcessError):
        git.get_commit_range_diff("nonexistent..refs", cwd=git_repo)


def test_detached_head(git_repo):
    sha = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=git_repo, text=True).strip()
    subprocess.run(["git", "checkout", sha], cwd=git_repo, capture_output=True)
    branch = git.get_current_branch(cwd=git_repo)
    assert branch == ""


def test_get_staged_diff_empty(git_repo):
    diff = git.get_staged_diff(cwd=git_repo)
    assert diff == ""


def test_get_staged_diff_with_staged_changes(git_repo):
    (git_repo / "file.txt").write_text("staged content\n")
    subprocess.run(["git", "add", "file.txt"], cwd=git_repo, capture_output=True)
    diff = git.get_staged_diff(cwd=git_repo)
    assert "staged content" in diff


def test_apply_patch_stage_hunk(git_repo):
    (git_repo / "file.txt").write_text("changed\n")
    diff = git.get_unstaged_diff(cwd=git_repo)
    # Apply the patch to staging area
    git.apply_patch(diff, cwd=git_repo, cached=True)
    staged = git.get_staged_diff(cwd=git_repo)
    assert "changed" in staged
    # Unstaged should now be empty
    unstaged = git.get_unstaged_diff(cwd=git_repo)
    assert unstaged == ""


def test_apply_patch_unstage_hunk(git_repo):
    (git_repo / "file.txt").write_text("changed\n")
    subprocess.run(["git", "add", "file.txt"], cwd=git_repo, capture_output=True)
    staged = git.get_staged_diff(cwd=git_repo)
    # Reverse-apply to unstage
    git.apply_patch(staged, cwd=git_repo, cached=True, reverse=True)
    assert git.get_staged_diff(cwd=git_repo) == ""
    assert "changed" in git.get_unstaged_diff(cwd=git_repo)


def test_apply_patch_discard_hunk(git_repo):
    (git_repo / "file.txt").write_text("changed\n")
    diff = git.get_unstaged_diff(cwd=git_repo)
    # Reverse-apply to discard
    git.apply_patch(diff, cwd=git_repo, reverse=True)
    assert (git_repo / "file.txt").read_text() == "hello\n"


def test_commit(git_repo):
    (git_repo / "file.txt").write_text("committed\n")
    subprocess.run(["git", "add", "file.txt"], cwd=git_repo, capture_output=True)
    git.commit("test commit", cwd=git_repo)
    log = subprocess.check_output(["git", "log", "--oneline", "-1"], cwd=git_repo, text=True)
    assert "test commit" in log


def test_timeout_raises(monkeypatch):
    def mock_run(*args, **kwargs):
        raise subprocess.TimeoutExpired(cmd="git", timeout=30)

    monkeypatch.setattr(subprocess, "run", mock_run)
    with pytest.raises(subprocess.CalledProcessError) as exc_info:
        git.get_repo_root()
    assert "timed out" in exc_info.value.stderr
