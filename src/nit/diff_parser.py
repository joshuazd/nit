from __future__ import annotations

import re

from .models import DiffHunk, DiffLine, FileDiff

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
