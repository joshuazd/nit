import json

from nit.comments import (
    comment_matches_line,
    export_comments_json,
    export_comments_markdown,
    load_comments,
    make_comment,
    save_comments,
)
from nit.models import DiffLine, ReviewComment


def test_load_missing_file(tmp_path):
    assert load_comments(tmp_path) == []


def test_save_and_load_roundtrip(tmp_path):
    comment = ReviewComment(
        file_path="src/app.py",
        new_line_no=42,
        old_line_no=None,
        line_content="return result",
        comment="Handle empty case",
        hunk_context=["line1", "return result", "line3"],
        timestamp="2025-01-15T10:00:00+00:00",
        diff_mode="branch",
    )
    save_comments(tmp_path, [comment], branch="feature", base="main")

    loaded = load_comments(tmp_path)
    assert len(loaded) == 1
    c = loaded[0]
    assert c.file_path == "src/app.py"
    assert c.new_line_no == 42
    assert c.old_line_no is None
    assert c.comment == "Handle empty case"
    assert c.diff_mode == "branch"


def test_save_creates_valid_json(tmp_path):
    save_comments(tmp_path, [], branch="main", base="main")
    data = json.loads((tmp_path / ".nit.json").read_text())
    assert data["version"] == 1
    assert data["branch"] == "main"
    assert data["comments"] == []


def test_comment_matches_line_new_line():
    comment = ReviewComment(
        file_path="f.py",
        new_line_no=10,
        old_line_no=None,
        line_content="x",
        comment="note",
    )
    line = DiffLine(content="x", line_type="add", new_line_no=10)
    assert comment_matches_line(comment, line) is True


def test_comment_matches_line_old_line_only():
    comment = ReviewComment(
        file_path="f.py",
        new_line_no=None,
        old_line_no=5,
        line_content="x",
        comment="note",
    )
    line = DiffLine(content="x", line_type="remove", old_line_no=5, new_line_no=None)
    assert comment_matches_line(comment, line) is True


def test_comment_no_match():
    comment = ReviewComment(
        file_path="f.py",
        new_line_no=10,
        old_line_no=None,
        line_content="x",
        comment="note",
    )
    line = DiffLine(content="x", line_type="add", new_line_no=99)
    assert comment_matches_line(comment, line) is False


def test_make_comment():
    line = DiffLine(content="code", line_type="add", new_line_no=5, old_line_no=None)
    c = make_comment("file.py", line, "looks wrong", ["ctx1", "code", "ctx2"], diff_mode="unstaged")
    assert c.file_path == "file.py"
    assert c.new_line_no == 5
    assert c.comment == "looks wrong"
    assert c.diff_mode == "unstaged"
    assert c.timestamp  # non-empty


def test_atomic_write_no_leftover_temp(tmp_path):
    save_comments(tmp_path, [], branch="main", base="main")
    files = list(tmp_path.iterdir())
    names = [f.name for f in files]
    assert ".nit.json" in names
    assert not any(n.endswith(".nit.tmp") for n in names)


def test_load_corrupted_json(tmp_path):
    (tmp_path / ".nit.json").write_text("not valid json{{{")
    assert load_comments(tmp_path) == []


def test_load_wrong_type(tmp_path):
    (tmp_path / ".nit.json").write_text('"just a string"')
    assert load_comments(tmp_path) == []


def test_load_missing_comment_fields(tmp_path):
    data = {"comments": [{"file": "x.py", "comment": "ok"}]}
    (tmp_path / ".nit.json").write_text(json.dumps(data))
    loaded = load_comments(tmp_path)
    assert len(loaded) == 1
    assert loaded[0].file_path == "x.py"


def _make_review_comment(**overrides):
    defaults = dict(
        file_path="src/app.py",
        new_line_no=10,
        old_line_no=None,
        line_content="return result",
        comment="Fix this",
        hunk_context=["line1", "return result"],
        timestamp="2025-01-15T10:00:00+00:00",
        diff_mode="branch",
    )
    defaults.update(overrides)
    return ReviewComment(**defaults)


def test_export_comments_markdown_empty():
    assert export_comments_markdown([]) == ""


def test_export_comments_markdown():
    comments = [
        _make_review_comment(file_path="b.py", new_line_no=5, comment="second file"),
        _make_review_comment(file_path="a.py", new_line_no=10, comment="note here"),
        _make_review_comment(
            file_path="a.py", new_line_no=None, old_line_no=3, comment="old line comment"
        ),
    ]
    md = export_comments_markdown(comments)
    assert md.startswith("# Code Review Comments")
    # Files sorted alphabetically
    assert md.index("## a.py") < md.index("## b.py")
    assert "**L10**: note here" in md
    assert "**old L3**: old line comment" in md
    assert "**L5**: second file" in md
    assert "```" in md  # code fence for line_content


def test_export_comments_json():
    comments = [
        _make_review_comment(comment="test comment"),
    ]
    result = export_comments_json(comments)
    parsed = json.loads(result)
    assert isinstance(parsed, list)
    assert len(parsed) == 1
    assert parsed[0]["comment"] == "test comment"
    assert parsed[0]["file"] == "src/app.py"
    assert parsed[0]["line"] == 10
