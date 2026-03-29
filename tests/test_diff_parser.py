from nit.diff_parser import parse_diff


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
