import pytest

from nit.cli import parse_args


def test_no_args():
    args = parse_args([])
    assert args.mode is None
    assert args.commit_range is None
    assert args.path_filter is None


def test_mode_branch():
    args = parse_args(["--mode", "branch"])
    assert args.mode == "branch"


def test_mode_unstaged():
    args = parse_args(["--mode", "unstaged"])
    assert args.mode == "unstaged"


def test_mode_all():
    args = parse_args(["--mode", "all"])
    assert args.mode == "all"


def test_commit_range():
    args = parse_args(["HEAD~3..HEAD"])
    assert args.commit_range == "HEAD~3..HEAD"


def test_commit_range_with_branches():
    args = parse_args(["main..feature"])
    assert args.commit_range == "main..feature"


def test_path_filter():
    args = parse_args(["--path", "src/"])
    assert args.path_filter == "src/"


def test_all_args():
    args = parse_args(["--mode", "branch", "--path", "src/", "main..feature"])
    assert args.mode == "branch"
    assert args.path_filter == "src/"
    assert args.commit_range == "main..feature"


def test_invalid_mode():
    with pytest.raises(SystemExit):
        parse_args(["--mode", "invalid"])


def test_version_exits():
    with pytest.raises(SystemExit):
        parse_args(["--version"])
