from __future__ import annotations

import os
from functools import lru_cache

from rich.syntax import Syntax
from rich.text import Text


@lru_cache(maxsize=32)
def _get_syntax(language: str) -> Syntax:
    """Get a cached Syntax object for a language."""
    theme = "ansi_dark"
    if os.environ.get("NO_COLOR"):
        theme = "default"
    return Syntax("", language, theme=theme, line_numbers=False)


def detect_language(file_path: str) -> str | None:
    """Detect language from file extension. Returns None if unknown."""
    try:
        lexer = Syntax.guess_lexer(file_path)
        return lexer if lexer not in ("text", "default") else None
    except Exception:
        return None


def highlight_line(code: str, language: str) -> Text:
    """Return a Rich Text with syntax highlighting for a single line."""
    syntax = _get_syntax(language)
    text = syntax.highlight(code)
    text.rstrip()
    return text
