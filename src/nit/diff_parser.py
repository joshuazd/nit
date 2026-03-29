from __future__ import annotations

import re
from difflib import SequenceMatcher

from .models import DiffHunk, DiffLine, FileDiff, SideBySideRow

DIFF_HEADER_RE = re.compile(r"^diff --git a/(.*) b/(.*)")
HUNK_HEADER_RE = re.compile(r"^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@(.*)")
RENAME_FROM_RE = re.compile(r"^rename from (.*)")
RENAME_TO_RE = re.compile(r"^rename to (.*)")
NEW_FILE_RE = re.compile(r"^new file mode")
DELETED_FILE_RE = re.compile(r"^deleted file mode")
BINARY_RE = re.compile(r"^Binary files")


def parse_diff(text: str) -> list[FileDiff]:
    if not text.strip():
        return []

    files: list[FileDiff] = []
    current_file: FileDiff | None = None
    current_hunk: DiffHunk | None = None
    old_no = 0
    new_no = 0

    for line in text.splitlines():
        # New file diff
        m = DIFF_HEADER_RE.match(line)
        if m:
            current_file = FileDiff(path=m.group(2), old_path=m.group(1))
            if m.group(1) != m.group(2):
                current_file.status = "renamed"
            files.append(current_file)
            current_hunk = None
            continue

        if current_file is None:
            continue

        # File metadata
        if NEW_FILE_RE.match(line):
            current_file.status = "added"
            continue
        if DELETED_FILE_RE.match(line):
            current_file.status = "deleted"
            continue
        m = RENAME_FROM_RE.match(line)
        if m:
            current_file.old_path = m.group(1)
            current_file.status = "renamed"
            continue
        m = RENAME_TO_RE.match(line)
        if m:
            current_file.path = m.group(1)
            continue
        if BINARY_RE.match(line):
            current_file.is_binary = True
            continue

        # Skip index and ---/+++ lines
        if line.startswith("index ") or line.startswith("--- ") or line.startswith("+++ "):
            continue

        # Hunk header
        m = HUNK_HEADER_RE.match(line)
        if m:
            old_no = int(m.group(1))
            new_no = int(m.group(2))
            current_hunk = DiffHunk(
                header=line,
                old_start=old_no,
                new_start=new_no,
            )
            current_hunk.lines.append(
                DiffLine(
                    content=m.group(3).strip(),
                    line_type="hunk_header",
                    raw=line,
                )
            )
            current_file.hunks.append(current_hunk)
            continue

        if current_hunk is None:
            continue

        # Diff lines
        if line.startswith("+"):
            current_hunk.lines.append(
                DiffLine(
                    content=line[1:],
                    line_type="add",
                    new_line_no=new_no,
                    raw=line,
                )
            )
            new_no += 1
        elif line.startswith("-"):
            current_hunk.lines.append(
                DiffLine(
                    content=line[1:],
                    line_type="remove",
                    old_line_no=old_no,
                    raw=line,
                )
            )
            old_no += 1
        elif line.startswith("\\"):
            # "\ No newline at end of file"
            continue
        else:
            # Context line (starts with space or is empty)
            content = line[1:] if line.startswith(" ") else line
            current_hunk.lines.append(
                DiffLine(
                    content=content,
                    line_type="context",
                    old_line_no=old_no,
                    new_line_no=new_no,
                    raw=line,
                )
            )
            old_no += 1
            new_no += 1

    return files


def file_to_diff(path: str, content: str) -> list[FileDiff]:
    """Create a synthetic FileDiff showing all lines as context for file review."""
    lines_text = content.splitlines()
    n = len(lines_text)
    diff_lines = [
        DiffLine(
            content=f"@@ -1,{n} +1,{n} @@",
            line_type="hunk_header",
            raw=f"@@ -1,{n} +1,{n} @@",
        )
    ]
    for i, line in enumerate(lines_text, 1):
        diff_lines.append(
            DiffLine(
                content=line,
                line_type="context",
                old_line_no=i,
                new_line_no=i,
                raw=f" {line}",
            )
        )
    hunk = DiffHunk(header=diff_lines[0].content, lines=diff_lines, old_start=1, new_start=1)
    return [FileDiff(path=path, status="modified", hunks=[hunk])]


def align_hunk_lines(lines: list[DiffLine]) -> list[SideBySideRow]:
    """Pair remove/add lines for side-by-side display."""
    rows: list[SideBySideRow] = []
    removes: list[DiffLine] = []
    adds: list[DiffLine] = []

    def flush() -> None:
        # Zip removes and adds pairwise, pad shorter with None
        for i in range(max(len(removes), len(adds))):
            left = removes[i] if i < len(removes) else None
            right = adds[i] if i < len(adds) else None
            rows.append(SideBySideRow(left=left, right=right, row_type="change"))
        removes.clear()
        adds.clear()

    for dl in lines:
        if dl.line_type == "hunk_header":
            flush()
            rows.append(SideBySideRow(left=dl, right=None, row_type="hunk_header"))
        elif dl.line_type == "remove":
            # If we had pending adds without removes, flush first
            if adds and not removes:
                flush()
            removes.append(dl)
        elif dl.line_type == "add":
            adds.append(dl)
        else:
            # Context line — flush any pending changes first
            flush()
            rows.append(SideBySideRow(left=dl, right=dl, row_type="context"))

    flush()
    return rows


def build_patch(file_diff: FileDiff, hunk: DiffHunk) -> str:
    """Construct a minimal git-apply-compatible patch for a single hunk."""
    path = file_diff.path
    old_path = file_diff.old_path or path

    lines = [f"diff --git a/{old_path} b/{path}"]

    if file_diff.status == "added":
        lines.append("--- /dev/null")
        lines.append(f"+++ b/{path}")
    elif file_diff.status == "deleted":
        lines.append(f"--- a/{old_path}")
        lines.append("+++ /dev/null")
    else:
        lines.append(f"--- a/{old_path}")
        lines.append(f"+++ b/{path}")

    lines.append(hunk.header)
    for dl in hunk.lines:
        if dl.line_type == "hunk_header":
            continue
        lines.append(dl.raw)

    # Ensure trailing newline
    return "\n".join(lines) + "\n"


_WORD_SPLIT_RE = re.compile(r"(\S+|\s+)")


def _tokenize(text: str) -> list[str]:
    """Split text into word and whitespace tokens."""
    return _WORD_SPLIT_RE.findall(text)


def word_diff_segments(
    old_text: str, new_text: str
) -> tuple[list[tuple[str, bool]], list[tuple[str, bool]]]:
    """Compute word-level diff between two strings.

    Tokenizes into words/whitespace first so that SequenceMatcher compares
    meaningful units rather than individual characters.

    Returns two lists of (text, changed) tuples — one for the old line,
    one for the new line. ``changed=True`` marks segments that differ.
    """
    old_tokens = _tokenize(old_text)
    new_tokens = _tokenize(new_text)
    sm = SequenceMatcher(None, old_tokens, new_tokens)
    old_segs: list[tuple[str, bool]] = []
    new_segs: list[tuple[str, bool]] = []
    for op, i1, i2, j1, j2 in sm.get_opcodes():
        if op == "equal":
            old_segs.append(("".join(old_tokens[i1:i2]), False))
            new_segs.append(("".join(new_tokens[j1:j2]), False))
        elif op == "replace":
            old_segs.append(("".join(old_tokens[i1:i2]), True))
            new_segs.append(("".join(new_tokens[j1:j2]), True))
        elif op == "delete":
            old_segs.append(("".join(old_tokens[i1:i2]), True))
        elif op == "insert":
            new_segs.append(("".join(new_tokens[j1:j2]), True))
    return old_segs, new_segs
