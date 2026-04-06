# nit

Terminal diff viewer with inline review comments.

## Architecture

Go + Bubble Tea TUI. Single-screen app with two panels (file tree + diff view).

- `cmd/nit/main.go` — Entry point, cobra CLI, version injection via ldflags
- `internal/ui/app.go` — Bubble Tea model, state management, git operations, View()
- `internal/ui/diffview.go` — Diff viewport: lazy rendering, cursor, scroll, unified + SBS, word wrap
- `internal/ui/filetree.go` — Nested tree with guide characters, directory grouping, cursor
- `internal/ui/statusbar.go` — 4-segment status bar (branch, mode, files, comments)
- `internal/ui/footer.go` — Keybinding hints
- `internal/ui/input.go` — Comment + commit text input
- `internal/ui/keys.go` — All keybindings in one place
- `internal/ui/styles.go` — Lip Gloss ANSI styles (vigil pattern: string color refs, bg threading)
- `internal/diff/` — Parser, side-by-side alignment, patch builder, word-level diff
- `internal/git/` — Subprocess wrappers with 30s timeout, all diff modes
- `internal/comments/` — `.nit.json` load/save (atomic), export markdown/JSON
- `internal/syntax/` — Chroma-based highlighting, language detection
- `internal/models/` — DiffLine, DiffHunk, FileDiff, SideBySideRow, ReviewComment
- `internal/cli/` — CLIArgs struct, target parsing

## Package

- Binary: `nit`
- License: GPL-3.0
- Homebrew: `joshuazd/homebrew-tap`

## Development

```bash
make build    # build ./nit binary
make test     # run tests with race detection
make lint     # golangci-lint
make vet      # go vet
make check    # test + lint + vet
make install  # install to ~/.local/bin/nit
make run      # build and run (usage: make run ARGS="--mode unstaged")
```

## Legacy Python Version

The original Python implementation lives in `python/`. Run with `cd python && ./nit`.

## Key Conventions

- Colors use ANSI 0-15 via `lipgloss.Color("N")` — inherits terminal theme
- Cursor background threading: pass `bg *lipgloss.Color` through render functions (vigil pattern)
- Unified cursor: single `lipgloss.NewStyle().Width(w).Background(Black).Render()` pass
- Comments persist to `.nit.json` at the git repo root (globally gitignored)
- Git operations use `g`-prefix chords dispatched in key handler
- Lazy rendering: only visible viewport lines are formatted, cached by toggle state
- Word diff results cached per file load
- Auto-refresh uses `git diff --stat` as cheap change fingerprint
- Side-by-side wraps each column independently within half width
- Claude reads comments via the `/nit` skill
