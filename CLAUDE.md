# nit

Terminal diff viewer with inline review comments.

## Architecture

Python + Textual TUI. Single-screen app with two panels (file list + diff view).

- `app.py` — Textual app, widgets (DiffView, FileTree, CommentBlock, CommitInput), keybindings, git operations
- `cli.py` — argparse CLI: `--mode`, `--version`, `--path`, positional commit range
- `diff_parser.py` — Unified diff text → structured dataclasses; `align_hunk_lines()` for side-by-side; `build_patch()` for git apply; `word_diff_segments()` for word-level highlighting
- `git.py` — Thin subprocess wrappers: diff (branch/unstaged/staged/all/range), `apply_patch()`, `commit()`
- `models.py` — `DiffLine`, `DiffHunk`, `FileDiff`, `SideBySideRow`, `ReviewComment`
- `comments.py` — Read/write `.nit.json` (atomic writes via temp file + rename)

## Package

- PyPI name: `nit-cli` (import name: `nit`, CLI command: `nit`)
- License: GPL-3.0

## Development

```bash
pip install -e ".[dev]"   # development install with test/lint deps
make test                 # run tests via venv
make lint                 # run ruff via venv
make release              # tag, push, publish to PyPI, update Homebrew formula
```

The `./nit` bootstrap script auto-creates a venv at `~/.local/share/nit/venv` for quick local use.

## Key Conventions

- All colors use Textual theme tokens in TCSS — inherits terminal color scheme
- Command palette is disabled (`COMMANDS = set()`)
- Theme is `textual-ansi` with `ansi_color=True` (inherits terminal palette)
- Comments persist to `.nit.json` at the git repo root (globally gitignored)
- Git operations use `g`-prefix chords (like vim's `g` namespace), dispatched via `on_key()` handler
- Side-by-side view uses `SideBySideRow` alignment model; word diff uses `word_diff_segments()`
- Claude reads comments via the `/nit` skill
- Homebrew formula lives in `joshuazd/homebrew-tap`
