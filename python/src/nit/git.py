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
        for ref in (f"refs/remotes/origin/{name}", f"refs/heads/{name}"):
            result = subprocess.run(
                ["git", "rev-parse", "--verify", ref],
                capture_output=True,
                text=True,
                cwd=cwd,
                timeout=GIT_TIMEOUT,
            )
            if result.returncode == 0:
                return f"origin/{name}" if ref.startswith("refs/remotes") else name
    return "main"


def get_merge_base(base: str, cwd: Path | None = None) -> str:
    return _run(["git", "merge-base", base, "HEAD"], cwd=cwd).strip()


def _append_path_filter(cmd: list[str], path_filter: str | None) -> list[str]:
    if path_filter:
        return cmd + ["--", path_filter]
    return cmd


def _append_whitespace_flag(cmd: list[str], ignore_whitespace: bool) -> list[str]:
    if ignore_whitespace:
        cmd = cmd + ["-w"]
    return cmd


def _build_diff_cmd(
    base_cmd: list[str], ignore_whitespace: bool, path_filter: str | None
) -> list[str]:
    cmd = _append_whitespace_flag(base_cmd, ignore_whitespace)
    return _append_path_filter(cmd, path_filter)


def get_branch_diff(
    cwd: Path | None = None,
    path_filter: str | None = None,
    ignore_whitespace: bool = False,
) -> str:
    base = get_main_branch(cwd)
    cmd = _build_diff_cmd(["git", "diff", f"{base}...HEAD"], ignore_whitespace, path_filter)
    return _run(cmd, cwd=cwd)


def get_unstaged_diff(
    cwd: Path | None = None,
    path_filter: str | None = None,
    ignore_whitespace: bool = False,
) -> str:
    cmd = _build_diff_cmd(["git", "diff"], ignore_whitespace, path_filter)
    return _run(cmd, cwd=cwd)


def get_upstream_ref(cwd: Path | None = None) -> str | None:
    """Return the upstream tracking ref, or None if not set."""
    try:
        return _run(["git", "rev-parse", "--abbrev-ref", "@{upstream}"], cwd=cwd).strip()
    except subprocess.CalledProcessError:
        return None


def get_unpushed_diff(
    cwd: Path | None = None,
    path_filter: str | None = None,
    ignore_whitespace: bool = False,
) -> str:
    upstream = get_upstream_ref(cwd)
    if upstream is None:
        raise subprocess.CalledProcessError(1, "git", stderr="No upstream branch set")
    cmd = _build_diff_cmd(["git", "diff", upstream], ignore_whitespace, path_filter)
    return _run(cmd, cwd=cwd)


def get_staged_diff(
    cwd: Path | None = None,
    path_filter: str | None = None,
    ignore_whitespace: bool = False,
) -> str:
    cmd = _build_diff_cmd(["git", "diff", "--cached"], ignore_whitespace, path_filter)
    return _run(cmd, cwd=cwd)


def apply_patch(
    patch_text: str, cwd: Path | None = None, cached: bool = False, reverse: bool = False
) -> str:
    cmd = ["git", "apply"]
    if cached:
        cmd.append("--cached")
    if reverse:
        cmd.append("--reverse")
    logger.debug("Running: %s (with stdin patch)", " ".join(cmd))
    try:
        result = subprocess.run(
            cmd,
            input=patch_text,
            capture_output=True,
            text=True,
            cwd=cwd,
            timeout=GIT_TIMEOUT,
        )
    except subprocess.TimeoutExpired:
        raise subprocess.CalledProcessError(1, cmd, "", "Command timed out")
    if result.returncode != 0:
        raise subprocess.CalledProcessError(result.returncode, cmd, result.stdout, result.stderr)
    return result.stdout


def commit(message: str, cwd: Path | None = None) -> str:
    return _run(["git", "commit", "-m", message], cwd=cwd)


def get_untracked_files(cwd: Path | None = None) -> list[str]:
    """Return list of untracked file paths (respecting .gitignore)."""
    out = _run(["git", "ls-files", "--others", "--exclude-standard"], cwd=cwd)
    return [f for f in out.splitlines() if f]


def _untracked_file_diff(path: str, cwd: Path | None = None) -> str:
    """Generate a unified diff for an untracked file as if it were newly added."""
    full = (cwd or Path.cwd()) / path
    try:
        content = full.read_text(errors="replace")
    except (OSError, UnicodeDecodeError):
        return ""
    lines = content.splitlines()
    n = len(lines)
    parts = [
        f"diff --git a/{path} b/{path}",
        "new file mode 100644",
        "--- /dev/null",
        f"+++ b/{path}",
        f"@@ -0,0 +1,{n} @@",
    ]
    for line in lines:
        parts.append(f"+{line}")
    return "\n".join(parts) + "\n"


def get_untracked_diff(cwd: Path | None = None, path_filter: str | None = None) -> str:
    """Return synthetic diff text for all untracked files."""
    files = get_untracked_files(cwd)
    if path_filter:
        files = [f for f in files if f.startswith(path_filter) or f == path_filter]
    return "".join(_untracked_file_diff(f, cwd) for f in files)


def get_commit_range_diff(
    commit_range: str,
    cwd: Path | None = None,
    path_filter: str | None = None,
    ignore_whitespace: bool = False,
) -> str:
    cmd = _build_diff_cmd(["git", "diff", commit_range], ignore_whitespace, path_filter)
    return _run(cmd, cwd=cwd)
