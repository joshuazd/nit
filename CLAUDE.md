# nit

Terminal diff viewer with inline review comments.

## Architecture

Python + Textual TUI. Single-screen app with two panels (file list + diff view).

- `app.py` — Textual app, widgets (DiffView, FileList, CommentEditor), keybindings
- `cli.py` — argparse CLI: `--mode`, `--version`, `--path`, positional commit range
- `diff_parser.py` — Unified diff text → structured `FileDiff`/`DiffHunk`/`DiffLine` dataclasses
- `git.py` — Thin subprocess wrappers around git commands
- `models.py` — Dataclasses shared across modules
- `comments.py` — Read/write `.nit.json` (atomic writes via temp file + rename)

## Package

- PyPI name: `nit-cli` (import name: `nit`, CLI command: `nit`)
- License: GPL-3.0

## Testing

```bash
pytest           # run all tests
pytest -v        # verbose
pytest tests/test_diff_parser.py  # single module
```

## Linting

```bash
ruff check src/ tests/
ruff format --check src/ tests/
```

## Installation

```bash
pip install -e ".[dev]"   # development install with test/lint deps
```

The `./nit` bootstrap script auto-creates a venv at `~/.local/share/nit/venv` for quick local use.

## Key Conventions

- All colors use Textual theme tokens in TCSS — inherits terminal color scheme
- Command palette is disabled (`COMMANDS = set()`)
- Theme is `nord`
- Comments persist to `.nit.json` at the git repo root (globally gitignored)
- Claude reads comments via the `/review-feedback` skill
