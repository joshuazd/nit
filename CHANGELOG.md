# Changelog

## 0.2.1

- Switch to `textual-ansi` theme with `ansi_color=True` for native terminal colors
- Fix file tree focus background tint and cursor foreground color overrides
- Add `gg`/`G` vim keybindings to jump to start/end of diff
- Add visual hunk separators with prominent headers
- Style footer, cursors, and hunk headers with ANSI palette colors
- Fix bootstrap script reinstalling on every run

## 0.2.0

- Add error handling for corrupted `.nit.json`, git command failures, and subprocess timeouts
- Add `--verbose` / `-v` flag for debug logging
- Add PyPI trusted publishing workflow
- Pin textual dependency to `<1.0`

## 0.1.0

Initial release. Terminal diff viewer with inline review comments.
