from __future__ import annotations

import json
import logging
import tempfile
from datetime import datetime, timezone
from pathlib import Path

from .models import DiffLine, ReviewComment

logger = logging.getLogger(__name__)

COMMENT_FILE = ".nit.json"


def _comment_to_dict(c: ReviewComment) -> dict:
    d: dict = {"file": c.file_path, "line_content": c.line_content, "comment": c.comment}
    if c.new_line_no is not None:
        d["line"] = c.new_line_no
    if c.old_line_no is not None:
        d["old_line"] = c.old_line_no
    d["hunk_context"] = c.hunk_context
    d["timestamp"] = c.timestamp
    d["diff_mode"] = c.diff_mode
    return d


def _dict_to_comment(d: dict) -> ReviewComment:
    return ReviewComment(
        file_path=d["file"],
        new_line_no=d.get("line"),
        old_line_no=d.get("old_line"),
        line_content=d.get("line_content", ""),
        comment=d["comment"],
        hunk_context=d.get("hunk_context", []),
        timestamp=d.get("timestamp", ""),
        diff_mode=d.get("diff_mode", "branch"),
    )


def load_comments(repo_root: Path) -> list[ReviewComment]:
    path = repo_root / COMMENT_FILE
    if not path.exists():
        return []
    try:
        data = json.loads(path.read_text())
        return [_dict_to_comment(c) for c in data.get("comments", [])]
    except (json.JSONDecodeError, KeyError, TypeError, AttributeError) as e:
        logger.warning("Failed to load %s: %s", path, e)
        return []


def save_comments(
    repo_root: Path,
    comments: list[ReviewComment],
    branch: str = "",
    base: str = "",
) -> None:
    data = {
        "version": 1,
        "branch": branch,
        "base": base,
        "comments": [_comment_to_dict(c) for c in comments],
    }
    path = repo_root / COMMENT_FILE
    # Atomic write
    fd, tmp = tempfile.mkstemp(dir=repo_root, suffix=".nit.tmp")
    try:
        with open(fd, "w") as f:
            json.dump(data, f, indent=2)
            f.write("\n")
        Path(tmp).replace(path)
    except Exception:
        Path(tmp).unlink(missing_ok=True)
        raise


def comment_matches_line(c: ReviewComment, dl: DiffLine) -> bool:
    if c.new_line_no is not None and dl.new_line_no == c.new_line_no:
        return True
    if c.old_line_no is not None and dl.old_line_no == c.old_line_no and dl.new_line_no is None:
        return True
    return False


def make_comment(
    file_path: str,
    line: DiffLine,
    comment_text: str,
    hunk_context: list[str],
    diff_mode: str = "branch",
) -> ReviewComment:
    return ReviewComment(
        file_path=file_path,
        new_line_no=line.new_line_no,
        old_line_no=line.old_line_no,
        line_content=line.content,
        comment=comment_text,
        hunk_context=hunk_context,
        timestamp=datetime.now(timezone.utc).isoformat(),
        diff_mode=diff_mode,
    )
