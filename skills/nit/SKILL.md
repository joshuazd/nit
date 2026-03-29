---
description: Read and address inline review comments from .nit.json left in the nit TUI diff viewer
allowed-tools: Read, Bash(test *)
---

# Nit Review

Read and address inline code review comments from `.nit.json`.

## Comment format

Each comment in `.nit.json` has:
- `file` — the file path
- `line` — new line number (added/context lines)
- `old_line` — old line number (removed lines)
- `line_content` — the diff line the comment is on
- `comment` — the review feedback
- `hunk_context` — surrounding diff lines for context
- `diff_mode` — which diff mode was active (branch/unstaged/all)

## Instructions

1. Check for comments:

!`test -f .nit.json && echo "FOUND" || echo "NO_COMMENTS"`

2. If NO_COMMENTS: tell the user there are no nit comments to review, then stop.

3. If FOUND: read the file with the Read tool at `.nit.json`.

4. For each comment:
   - Read the referenced file at the specified line
   - Use `hunk_context` to understand surrounding changes
   - Address the feedback: fix, refactor, or explain why the current approach is correct
   - If ambiguous, ask for clarification

5. After addressing all comments, summarize what changed per comment.

6. Do NOT delete `.nit.json` automatically. Tell the user they can clear it when satisfied:
   ```
   rm .nit.json
   ```
