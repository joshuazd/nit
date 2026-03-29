# nit

Terminal diff viewer with inline review comments.

Navigate diffs with vim-style keybindings, leave comments on specific lines, and persist them as structured JSON. Useful for code review workflows, self-review before committing, and giving feedback to AI coding tools.

## Install

```bash
brew install joshuazd/tap/nit
```

Or via pip/pipx:

```bash
pipx install nit-cli
```

Or from source:

```bash
git clone https://github.com/joshuazd/nit.git
pipx install ./nit
```

## Usage

```bash
# Review current branch changes (vs main/master)
nit

# Review unstaged changes
nit --mode unstaged

# Review all uncommitted changes
nit --mode all

# Review a specific commit range
nit HEAD~3..HEAD
nit main..feature

# Filter to specific files or directories
nit --path src/
nit --path src/nit/app.py main..feature
```

## Keybindings

| Key | Action |
|-----|--------|
| `j` / `k` | Move cursor down / up |
| `J` / `K` | Jump to next / previous hunk |
| `n` / `p` | Next / previous file |
| `]` / `[` | Jump to next / previous comment |
| `gg` / `G` | Jump to top / bottom |
| `c` | Add comment on current line |
| `d` | Delete comment on current line |
| `m` | Cycle diff mode (branch / unstaged / staged / all) |
| `s` | Toggle side-by-side diff view |
| `w` | Toggle word-level diff highlighting |
| `r` | Refresh diff |
| `q` | Quit |

### Git operations (g-prefixed)

| Key | Action |
|-----|--------|
| `ga` | Stage hunk under cursor |
| `gu` | Unstage hunk (staged mode) |
| `gx` | Discard hunk |
| `gc` | Commit with message prompt |

## Comment Storage

Comments are saved to `.nit.json` at the root of your git repository. The format is structured JSON:

```json
{
  "version": 1,
  "branch": "my-feature",
  "base": "main",
  "comments": [
    {
      "file": "src/app.py",
      "line": 42,
      "line_content": "    return result",
      "comment": "Should handle the empty case here",
      "hunk_context": ["...", "..."],
      "timestamp": "2025-01-15T10:30:00+00:00",
      "diff_mode": "branch"
    }
  ]
}
```

Add `.nit.json` to your global gitignore to keep comments local:

```bash
echo '.nit.json' >> ~/.config/git/ignore
```

## Claude Code Integration

nit works as a review tool for [Claude Code](https://docs.anthropic.com/en/docs/claude-code). Leave comments on Claude's changes using nit, then have Claude read them with `/nit`. This creates a feedback loop where you can guide Claude's work through inline code review.

### Install the plugin

In Claude Code:

```
/plugin marketplace add joshuazd/nit
/plugin install nit
```

Then use `/nit` in any project to have Claude read and address your comments.

### Local development

If you cloned the repo, you can test the plugin directly:

```bash
claude --plugin-dir /path/to/nit
```

## Development

```bash
git clone https://github.com/joshuazd/nit.git
cd nit
pip install -e ".[dev]"
pytest
```

The `./nit` bootstrap script auto-creates a virtualenv at `~/.local/share/nit/venv` for quick local use without a manual install.

## License

GPL-3.0 - see [LICENSE](LICENSE) for details.
