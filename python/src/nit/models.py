from __future__ import annotations

from dataclasses import dataclass, field


@dataclass
class DiffLine:
    content: str
    line_type: str  # "context", "add", "remove", "hunk_header"
    old_line_no: int | None = None
    new_line_no: int | None = None
    raw: str = ""


@dataclass
class DiffHunk:
    header: str
    lines: list[DiffLine] = field(default_factory=list)
    old_start: int = 0
    new_start: int = 0


@dataclass
class FileDiff:
    path: str
    old_path: str | None = None
    status: str = "modified"  # "modified", "added", "deleted", "renamed"
    hunks: list[DiffHunk] = field(default_factory=list)
    is_binary: bool = False


@dataclass
class SideBySideRow:
    left: DiffLine | None  # old side (remove or context)
    right: DiffLine | None  # new side (add or context)
    row_type: str  # "context", "change", "hunk_header"


@dataclass
class ReviewComment:
    file_path: str
    new_line_no: int | None
    old_line_no: int | None
    line_content: str
    comment: str
    hunk_context: list[str] = field(default_factory=list)
    timestamp: str = ""
    diff_mode: str = "branch"
