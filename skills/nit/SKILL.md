---
description: Read inline review comments left in the nit TUI diff viewer
allowed-tools: Bash(cat *), Bash(test *), Bash(rm *)
---

# Review Feedback

Read and address inline code review comments from `.nit.json`.

## Instructions

1. Read the review file:

!`cat .nit.json 2>/dev/null || echo '{"comments":[]}'`

2. For each comment:
   - Read the referenced file at the specified line
   - Understand the surrounding context and the comment
   - Address the feedback: fix, refactor, or explain why the current approach is correct
   - If ambiguous, ask for clarification

3. After addressing all comments, summarize what changed per comment.

4. Clean up:
```bash
rm .nit.json
```
