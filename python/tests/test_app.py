import subprocess
from pathlib import Path
from unittest.mock import patch

import pytest

from nit.app import NitApp
from nit.cli import CLIArgs

SAMPLE_DIFF = """\
diff --git a/test.py b/test.py
index abc..def 100644
--- a/test.py
+++ b/test.py
@@ -1,3 +1,3 @@
 line1
-old
+new
 line3
"""


@pytest.fixture
def mock_git():
    with patch("nit.app.git") as mock:
        mock.get_repo_root.return_value = Path("/tmp/fake-repo")
        mock.get_current_branch.return_value = "test-branch"
        mock.get_main_branch.return_value = "main"
        mock.get_branch_diff.return_value = SAMPLE_DIFF
        mock.get_unstaged_diff.return_value = ""
        mock.get_unpushed_diff.return_value = ""
        mock.get_untracked_diff.return_value = ""
        yield mock


@pytest.fixture
def mock_comments():
    with patch("nit.app.comments_mod") as mock:
        mock.load_comments.return_value = []
        yield mock


async def test_app_mounts(mock_git, mock_comments):
    app = NitApp()
    async with app.run_test():
        assert app.query_one("#sidebar") is not None
        assert app.query_one("#diff-view") is not None


async def test_app_loads_diff(mock_git, mock_comments):
    app = NitApp()
    async with app.run_test():
        assert len(app.file_diffs) == 1
        assert app.file_diffs[0].path == "test.py"


async def test_quit(mock_git, mock_comments):
    app = NitApp()
    async with app.run_test() as pilot:
        await pilot.press("q")


async def test_cli_args_mode(mock_git, mock_comments):
    app = NitApp(cli_args=CLIArgs(mode="unstaged"))
    async with app.run_test():
        assert app.diff_mode == "unstaged"


async def test_cli_args_commit_range(mock_git, mock_comments):
    mock_git.get_commit_range_diff.return_value = SAMPLE_DIFF
    app = NitApp(cli_args=CLIArgs(commit_range="HEAD~1..HEAD"))
    async with app.run_test():
        mock_git.get_commit_range_diff.assert_called_once()


async def test_app_git_diff_failure(mock_git, mock_comments):
    mock_git.get_branch_diff.side_effect = subprocess.CalledProcessError(
        1, ["git", "diff"], "", "fatal: bad revision"
    )
    app = NitApp()
    async with app.run_test():
        assert app.file_diffs == []


async def test_app_corrupted_comments(mock_git, mock_comments):
    mock_comments.load_comments.side_effect = Exception("corrupt")
    app = NitApp()
    async with app.run_test():
        assert app.comments == []


async def test_app_detached_head(mock_git, mock_comments):
    mock_git.get_current_branch.side_effect = Exception("not on branch")
    app = NitApp()
    async with app.run_test():
        assert app.branch == "(detached HEAD)"


async def test_g_chord_unrecognized_key_not_swallowed(mock_git, mock_comments):
    app = NitApp()
    async with app.run_test() as pilot:
        # Press g — sets pending flag
        await pilot.press("g")
        assert app._pending_g is True
        # Press unrecognized key — flag resets, key not swallowed
        await pilot.press("z")
        assert app._pending_g is False
        # Valid chord still works after: gg = cursor to start
        await pilot.press("g", "g")
        dv = app.query_one("#diff-view")
        assert dv.cursor_index == 0
