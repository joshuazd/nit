from __future__ import annotations

import logging
import subprocess
from pathlib import Path

logger = logging.getLogger(__name__)

GIT_TIMEOUT = 30


def _run(args: list[str], cwd: Path | None = None) -> str:
    logger.debug("Running: %s", " ".join(args))
    try:
        result = subprocess.run(
            args,
            capture_output=True,
            text=True,
            cwd=cwd,
            timeout=GIT_TIMEOUT,
        )
    except subprocess.TimeoutExpired:
        logger.warning("Git command timed out: %s", " ".join(args))
        raise subprocess.CalledProcessError(1, args, "", "Command timed out")
    if result.returncode != 0:
        raise subprocess.CalledProcessError(
            result.returncode,
            args,
            result.stdout,
            result.stderr,
        )
    return result.stdout


def get_repo_root(cwd: Path | None = None) -> Path:
    out = _run(["git", "rev-parse", "--show-toplevel"], cwd=cwd)
    return Path(out.strip())


def get_current_branch(cwd: Path | None = None) -> str:
    return _run(["git", "branch", "--show-current"], cwd=cwd).strip()


def get_main_branch(cwd: Path | None = None) -> str:
    for name in ("main", "master"):
        result = subprocess.run(
            ["git", "rev-parse", "--verify", f"refs/heads/{name}"],
            capture_output=True,
            text=True,
            cwd=cwd,
            timeout=GIT_TIMEOUT,
        )
        if result.returncode == 0:
            return name
    return "main"


def get_merge_base(base: str, cwd: Path | None = None) -> str:
    return _run(["git", "merge-base", base, "HEAD"], cwd=cwd).strip()


def _append_path_filter(cmd: list[str], path_filter: str | None) -> list[str]:
    if path_filter:
        return cmd + ["--", path_filter]
    return cmd


def get_branch_diff(cwd: Path | None = None, path_filter: str | None = None) -> str:
    base = get_main_branch(cwd)
    cmd = ["git", "diff", f"{base}...HEAD"]
    return _run(_append_path_filter(cmd, path_filter), cwd=cwd)


def get_unstaged_diff(cwd: Path | None = None, path_filter: str | None = None) -> str:
    cmd = ["git", "diff"]
    return _run(_append_path_filter(cmd, path_filter), cwd=cwd)


def get_all_uncommitted_diff(cwd: Path | None = None, path_filter: str | None = None) -> str:
    cmd = ["git", "diff", "HEAD"]
    return _run(_append_path_filter(cmd, path_filter), cwd=cwd)


def get_commit_range_diff(
    commit_range: str, cwd: Path | None = None, path_filter: str | None = None
) -> str:
    cmd = ["git", "diff", commit_range]
    return _run(_append_path_filter(cmd, path_filter), cwd=cwd)
