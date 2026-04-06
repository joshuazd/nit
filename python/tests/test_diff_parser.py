from nit.diff_parser import align_hunk_lines, build_patch, parse_diff, word_diff_segments
from nit.models import DiffHunk, DiffLine, FileDiff


def test_empty_input():
    assert parse_diff("") == []
    assert parse_diff("   \n  ") == []


def test_single_modified_file():
    diff = """\
diff --git a/foo.py b/foo.py
index abc1234..def5678 100644
--- a/foo.py
+++ b/foo.py
@@ -1,3 +1,4 @@
 line1
-old line
+new line
+added line
 line3
"""
    files = parse_diff(diff)
    assert len(files) == 1
    f = files[0]
    assert f.path == "foo.py"
    assert f.status == "modified"
    assert len(f.hunks) == 1

    lines = f.hunks[0].lines
    # hunk_header, context, remove, add, add, context
    assert lines[0].line_type == "hunk_header"
    assert lines[1].line_type == "context"
    assert lines[1].content == "line1"
    assert lines[1].old_line_no == 1
    assert lines[1].new_line_no == 1
    assert lines[2].line_type == "remove"
    assert lines[2].content == "old line"
    assert lines[2].old_line_no == 2
    assert lines[2].new_line_no is None
    assert lines[3].line_type == "add"
    assert lines[3].content == "new line"
    assert lines[3].new_line_no == 2
    assert lines[3].old_line_no is None
    assert lines[4].line_type == "add"
    assert lines[4].content == "added line"
    assert lines[4].new_line_no == 3
    assert lines[5].line_type == "context"
    assert lines[5].old_line_no == 3
    assert lines[5].new_line_no == 4


def test_added_file():
    diff = """\
diff --git a/new.py b/new.py
new file mode 100644
index 0000000..abc1234
--- /dev/null
+++ b/new.py
@@ -0,0 +1,2 @@
+line1
+line2
"""
    files = parse_diff(diff)
    assert len(files) == 1
    assert files[0].status == "added"
    assert files[0].path == "new.py"


def test_deleted_file():
    diff = """\
diff --git a/old.py b/old.py
deleted file mode 100644
index abc1234..0000000
--- a/old.py
+++ /dev/null
@@ -1,2 +0,0 @@
-line1
-line2
"""
    files = parse_diff(diff)
    assert len(files) == 1
    assert files[0].status == "deleted"


def test_renamed_file():
    diff = """\
diff --git a/old_name.py b/new_name.py
similarity index 100%
rename from old_name.py
rename to new_name.py
"""
    files = parse_diff(diff)
    assert len(files) == 1
    assert files[0].status == "renamed"
    assert files[0].path == "new_name.py"
    assert files[0].old_path == "old_name.py"


def test_binary_file():
    diff = """\
diff --git a/image.png b/image.png
Binary files a/image.png and b/image.png differ
"""
    files = parse_diff(diff)
    assert len(files) == 1
    assert files[0].is_binary is True


def test_multiple_hunks():
    diff = """\
diff --git a/foo.py b/foo.py
index abc..def 100644
--- a/foo.py
+++ b/foo.py
@@ -1,3 +1,3 @@
 a
-b
+B
 c
@@ -10,3 +10,3 @@
 x
-y
+Y
 z
"""
    files = parse_diff(diff)
    assert len(files) == 1
    assert len(files[0].hunks) == 2
    assert files[0].hunks[0].old_start == 1
    assert files[0].hunks[1].old_start == 10


def test_multiple_files():
    diff = """\
diff --git a/a.py b/a.py
index abc..def 100644
--- a/a.py
+++ b/a.py
@@ -1,1 +1,1 @@
-old
+new
diff --git a/b.py b/b.py
new file mode 100644
index 0000000..abc1234
--- /dev/null
+++ b/b.py
@@ -0,0 +1 @@
+content
"""
    files = parse_diff(diff)
    assert len(files) == 2
    assert files[0].path == "a.py"
    assert files[1].path == "b.py"
    assert files[1].status == "added"


def test_no_newline_at_eof_skipped():
    diff = """\
diff --git a/foo.py b/foo.py
index abc..def 100644
--- a/foo.py
+++ b/foo.py
@@ -1,1 +1,1 @@
-old
\\ No newline at end of file
+new
\\ No newline at end of file
"""
    files = parse_diff(diff)
    lines = files[0].hunks[0].lines
    # Should have hunk_header, remove, add — no backslash lines
    types = [dl.line_type for dl in lines]
    assert "context" not in types or all(
        "\\" not in dl.content for dl in lines if dl.line_type == "context"
    )


# --- align_hunk_lines tests ---


def _dl(content, line_type, old_no=None, new_no=None):
    return DiffLine(content=content, line_type=line_type, old_line_no=old_no, new_line_no=new_no)


def test_align_pure_adds():
    lines = [
        _dl("", "hunk_header"),
        _dl("a", "add", new_no=1),
        _dl("b", "add", new_no=2),
    ]
    rows = align_hunk_lines(lines)
    assert rows[0].row_type == "hunk_header"
    assert rows[1].left is None
    assert rows[1].right.content == "a"
    assert rows[2].left is None
    assert rows[2].right.content == "b"


def test_align_pure_removes():
    lines = [
        _dl("", "hunk_header"),
        _dl("a", "remove", old_no=1),
        _dl("b", "remove", old_no=2),
    ]
    rows = align_hunk_lines(lines)
    assert rows[1].left.content == "a"
    assert rows[1].right is None
    assert rows[2].left.content == "b"
    assert rows[2].right is None


def test_align_matched_pairs():
    lines = [
        _dl("", "hunk_header"),
        _dl("old", "remove", old_no=1),
        _dl("new", "add", new_no=1),
    ]
    rows = align_hunk_lines(lines)
    assert len(rows) == 2  # header + 1 change row
    assert rows[1].row_type == "change"
    assert rows[1].left.content == "old"
    assert rows[1].right.content == "new"


def test_align_unequal_counts():
    lines = [
        _dl("", "hunk_header"),
        _dl("a", "remove", old_no=1),
        _dl("b", "remove", old_no=2),
        _dl("c", "remove", old_no=3),
        _dl("x", "add", new_no=1),
        _dl("y", "add", new_no=2),
    ]
    rows = align_hunk_lines(lines)
    # header + 3 change rows (2 paired + 1 left-only)
    assert len(rows) == 4
    assert rows[1].left.content == "a" and rows[1].right.content == "x"
    assert rows[2].left.content == "b" and rows[2].right.content == "y"
    assert rows[3].left.content == "c" and rows[3].right is None


def test_align_context_lines():
    lines = [
        _dl("", "hunk_header"),
        _dl("ctx", "context", old_no=1, new_no=1),
    ]
    rows = align_hunk_lines(lines)
    assert rows[1].row_type == "context"
    assert rows[1].left.content == "ctx"
    assert rows[1].right.content == "ctx"


def test_align_mixed():
    """Context, then remove+add pair, then context."""
    lines = [
        _dl("", "hunk_header"),
        _dl("before", "context", old_no=1, new_no=1),
        _dl("old", "remove", old_no=2),
        _dl("new", "add", new_no=2),
        _dl("after", "context", old_no=3, new_no=3),
    ]
    rows = align_hunk_lines(lines)
    assert len(rows) == 4  # header, context, change, context
    assert rows[1].row_type == "context"
    assert rows[2].row_type == "change"
    assert rows[3].row_type == "context"


# --- build_patch tests ---


def test_build_patch_modified():
    hunk = DiffHunk(
        header="@@ -1,3 +1,3 @@",
        old_start=1,
        new_start=1,
        lines=[
            DiffLine("", "hunk_header", raw="@@ -1,3 +1,3 @@"),
            DiffLine("a", "context", old_line_no=1, new_line_no=1, raw=" a"),
            DiffLine("old", "remove", old_line_no=2, raw="-old"),
            DiffLine("new", "add", new_line_no=2, raw="+new"),
            DiffLine("c", "context", old_line_no=3, new_line_no=3, raw=" c"),
        ],
    )
    fd = FileDiff(path="foo.py", status="modified")
    patch = build_patch(fd, hunk)
    assert patch.startswith("diff --git a/foo.py b/foo.py\n")
    assert "--- a/foo.py\n" in patch
    assert "+++ b/foo.py\n" in patch
    assert "@@ -1,3 +1,3 @@\n" in patch
    assert " a\n" in patch
    assert "-old\n" in patch
    assert "+new\n" in patch


def test_build_patch_added_file():
    hunk = DiffHunk(
        header="@@ -0,0 +1,1 @@",
        lines=[
            DiffLine("", "hunk_header", raw="@@ -0,0 +1,1 @@"),
            DiffLine("hello", "add", new_line_no=1, raw="+hello"),
        ],
    )
    fd = FileDiff(path="new.py", status="added")
    patch = build_patch(fd, hunk)
    assert "--- /dev/null\n" in patch
    assert "+++ b/new.py\n" in patch


def test_build_patch_deleted_file():
    hunk = DiffHunk(
        header="@@ -1,1 +0,0 @@",
        lines=[
            DiffLine("", "hunk_header", raw="@@ -1,1 +0,0 @@"),
            DiffLine("bye", "remove", old_line_no=1, raw="-bye"),
        ],
    )
    fd = FileDiff(path="old.py", status="deleted")
    patch = build_patch(fd, hunk)
    assert "--- a/old.py\n" in patch
    assert "+++ /dev/null\n" in patch


# --- word_diff_segments tests ---


def test_word_diff_identical():
    old_segs, new_segs = word_diff_segments("hello world", "hello world")
    assert old_segs == [("hello world", False)]
    assert new_segs == [("hello world", False)]


def test_word_diff_single_word_change():
    old_segs, new_segs = word_diff_segments("hello world", "hello earth")
    assert old_segs == [("hello ", False), ("world", True)]
    assert new_segs == [("hello ", False), ("earth", True)]


def test_word_diff_insertion():
    old_segs, new_segs = word_diff_segments("a b", "a x b")
    assert old_segs == [("a ", False), ("b", False)]
    assert new_segs == [("a ", False), ("x ", True), ("b", False)]


def test_word_diff_deletion():
    old_segs, new_segs = word_diff_segments("a x b", "a b")
    assert old_segs == [("a ", False), ("x ", True), ("b", False)]
    assert new_segs == [("a ", False), ("b", False)]


def test_word_diff_completely_different():
    old_segs, new_segs = word_diff_segments("aaa", "bbb")
    assert old_segs == [("aaa", True)]
    assert new_segs == [("bbb", True)]
