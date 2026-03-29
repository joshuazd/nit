import subprocess

import pytest

from nit import git


@pytest.fixture
def git_repo(tmp_path):
    """Create a temporary git repo with one commit."""
    subprocess.run(["git", "init"], cwd=tmp_path, capture_output=True)
    subprocess.run(["git", "checkout", "-b", "main"], cwd=tmp_path, capture_output=True)
    (tmp_path / "file.txt").write_text("hello\n")
    subprocess.run(["git", "add", "."], cwd=tmp_path, capture_output=True)
    subprocess.run(
        ["git", "commit", "-m", "init"],
        cwd=tmp_path,
        capture_output=True,
        env={
            "GIT_AUTHOR_NAME": "test",
            "GIT_AUTHOR_EMAIL": "t@t",
            "GIT_COMMITTER_NAME": "test",
            "GIT_COMMITTER_EMAIL": "t@t",
            "HOME": str(tmp_path),
            "PATH": subprocess.check_output(["bash", "-c", "echo $PATH"], text=True).strip(),
        },
    )
    return tmp_path


def test_get_repo_root(git_repo):
    root = git.get_repo_root(cwd=git_repo)
    assert root.resolve() == git_repo.resolve()


def test_get_current_branch(git_repo):
    branch = git.get_current_branch(cwd=git_repo)
    assert branch == "main"


def test_get_main_branch(git_repo):
    assert git.get_main_branch(cwd=git_repo) == "main"


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


def test_timeout_raises(monkeypatch):
    def mock_run(*args, **kwargs):
        raise subprocess.TimeoutExpired(cmd="git", timeout=30)

    monkeypatch.setattr(subprocess, "run", mock_run)
    with pytest.raises(subprocess.CalledProcessError) as exc_info:
        git.get_repo_root()
    assert "timed out" in exc_info.value.stderr
