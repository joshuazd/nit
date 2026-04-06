from rich.text import Text

from nit.syntax import detect_language, highlight_line


def test_detect_language_python():
    assert detect_language("app.py") == "python"


def test_detect_language_javascript():
    assert detect_language("index.js") == "javascript"


def test_detect_language_unknown():
    assert detect_language("file.xyzunknown") is None


def test_highlight_line_returns_text():
    result = highlight_line("x = 1", "python")
    assert isinstance(result, Text)
    assert str(result).strip() == "x = 1"


def test_highlight_line_empty():
    result = highlight_line("", "python")
    assert isinstance(result, Text)
