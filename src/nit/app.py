from __future__ import annotations

import logging
import os
import shutil
import subprocess
from collections import Counter
from pathlib import Path

from rich.style import Style
from rich.text import Text
from textual import events
from textual.app import App, ComposeResult
from textual.binding import Binding
from textual.containers import Horizontal, Vertical, VerticalScroll
from textual.reactive import reactive
from textual.widgets import Footer, Input, Label, Static, Tree
from textual.widgets._tree import TreeNode

from . import comments as comments_mod
from . import git
from .cli import CLIArgs
from .diff_parser import align_hunk_lines, build_patch, file_to_diff, parse_diff, word_diff_segments
from .models import DiffHunk, FileDiff, ReviewComment, SideBySideRow
from .models import DiffLine as DiffLineModel
from .syntax import detect_language, highlight_line

logger = logging.getLogger(__name__)

STATUS_COLORS = {
    "added": ("A", "green"),
    "deleted": ("D", "red"),
    "renamed": ("R", "cyan"),
    "modified": ("M", "yellow"),
}


def _file_label(fd: FileDiff, comment_count: int) -> Text:
    letter, color = STATUS_COLORS.get(fd.status, ("M", "yellow"))
    label = Text()
    label.append(letter, style=f"bold {color}")
    label.append(f" {os.path.basename(fd.path)}")
    if comment_count:
        label.append(f" ({comment_count})", style="dim")
    return label


# --- Diff mode enum ---

DIFF_MODES = ["branch", "unstaged", "staged", "all"]
DIFF_MODE_LABELS = {
    "branch": "branch diff",
    "unstaged": "unstaged",
    "staged": "staged",
    "all": "unpushed",
}


# --- Widgets ---


class FileTree(Tree):
    """Sidebar file tree with visible cursor."""

    SCOPED_CSS = False
    DEFAULT_CSS = """
    FileTree {
        background: $background;
    }
    FileTree:focus {
        background: $background;
        background-tint: initial;
    }
    FileTree > .tree--cursor {
        background: ansi_bright_black !important;
        text-style: bold;
    }
    FileTree:focus > .tree--cursor {
        background: ansi_bright_black !important;
        text-style: bold;
    }
    FileTree > .tree--highlight {
        background: ansi_bright_black !important;
    }
    FileTree > .tree--highlight-line {
        background: ansi_bright_black !important;
    }
    """

    ICON_NODE = ""
    ICON_NODE_EXPANDED = ""

    def render_label(self, node: TreeNode, base_style: Style, style: Style) -> Text:
        # Strip only foreground color to preserve label colors, keep bgcolor
        style = Style(
            bgcolor=style.bgcolor,
            bold=style.bold,
            italic=style.italic,
            underline=style.underline,
            link=style.link,
            overline=style.overline,
            strike=style.strike,
        )
        return super().render_label(node, base_style, style)


class DiffLineWidget(Static):
    """A single line in the diff view."""

    pass


class CommentBlock(Static):
    """An inline comment block rendered between diff lines."""

    DEFAULT_CSS = """
    CommentBlock {
        background: $surface;
        color: $text;
        margin: 0 2 0 6;
        padding: 0 1;
        border: round $warning;
        width: 1fr;
    }
    """


class InlineCommentInput(Input):
    """Inline input that appears below a diff line for entering comments."""

    BINDINGS = [
        Binding("escape", "cancel", "Cancel", priority=True),
    ]

    DEFAULT_CSS = """
    InlineCommentInput {
        margin: 0 2 0 6;
        border: round $accent;
        width: 1fr;
    }
    """

    def action_cancel(self) -> None:
        diff_view = next(a for a in self.ancestors if isinstance(a, DiffView))
        diff_view.hide_comment_input()


class CommitInput(Input):
    """Input for entering commit messages."""

    BINDINGS = [
        Binding("escape", "cancel", "Cancel", priority=True),
    ]

    DEFAULT_CSS = """
    CommitInput {
        dock: bottom;
        margin: 0 2;
        border: round $accent;
        width: 1fr;
    }
    """

    def action_cancel(self) -> None:
        app: NitApp = self.app  # type: ignore[assignment]
        app.hide_commit_input()


class DiffView(VerticalScroll):
    """Scrollable diff view with cursor tracking."""

    DEFAULT_CSS = """
    DiffView {
        width: 1fr;
    }
    DiffLineWidget {
        width: 1fr;
        padding: 0 1;
    }
    DiffLineWidget.add {
        background-tint: $success 25%;
        color: $success;
    }
    DiffLineWidget.remove {
        background-tint: $error 25%;
        color: $error;
    }
    DiffLineWidget.context {
        color: $text-muted;
    }
    DiffLineWidget.hunk-header {
        color: ansi_magenta;
        background: ansi_black;
        text-style: bold;
        margin: 1 0 0 0;
    }
    DiffLineWidget.hunk-header.cursor {
        background: ansi_bright_black;
    }
    DiffLineWidget.cursor {
        background: ansi_black;
    }
    DiffLineWidget.sbs {
        max-height: 1;
        overflow-x: hidden;
    }
    DiffLineWidget.add.cursor {
        background: ansi_black;
        color: $success;
    }
    DiffLineWidget.remove.cursor {
        background: ansi_black;
        color: $error;
    }
    """

    cursor_index: reactive[int] = reactive(0)
    side_by_side: reactive[bool] = reactive(False)
    word_diff: reactive[bool] = reactive(False)
    syntax_highlight: reactive[bool] = reactive(False)

    def __init__(self, *args, **kwargs) -> None:
        super().__init__(*args, **kwargs)
        self.diff_lines: list[DiffLineModel] = []
        self._line_widgets: list[DiffLineWidget] = []
        self._active_input: InlineCommentInput | None = None
        self._line_hunk_map: list[DiffHunk] = []
        self._sbs_rows: list[SideBySideRow] = []
        self._current_file_diff: FileDiff | None = None

    def clear_diff(self) -> None:
        self.diff_lines = []
        self._line_widgets = []
        self._line_hunk_map = []
        self._sbs_rows = []
        self._current_file_diff = None
        self.cursor_index = 0
        self.remove_children()

    def load_file_diff(
        self,
        file_diff: FileDiff,
        file_comments: list[ReviewComment],
        restore_cursor: int = 0,
    ) -> None:
        self.remove_children()
        self.diff_lines = []
        self._line_widgets = []
        self._line_hunk_map = []
        self._sbs_rows = []
        self._current_file_diff = file_diff

        # Build a lookup of comments by line number
        comment_map: dict[int | None, list[ReviewComment]] = {}
        for c in file_comments:
            key = c.new_line_no if c.new_line_no is not None else c.old_line_no
            comment_map.setdefault(key, []).append(c)

        if file_diff.is_binary:
            w = DiffLineWidget("Binary file")
            self.mount(w)
            return

        if self.side_by_side:
            self._load_side_by_side(file_diff, comment_map)
        else:
            self._load_unified(file_diff, comment_map)

        start = min(restore_cursor, len(self._line_widgets) - 1) if self._line_widgets else 0
        self.cursor_index = start
        if self._line_widgets:
            self._line_widgets[start].add_class("cursor")

    def _load_unified(
        self,
        file_diff: FileDiff,
        comment_map: dict[int | None, list[ReviewComment]],
    ) -> None:
        # Pre-compute word diff pairs: map line index → paired line for remove/add
        word_pairs: dict[int, DiffLineModel] = {}
        if self.word_diff:
            all_lines = [dl for hunk in file_diff.hunks for dl in hunk.lines]
            i = 0
            while i < len(all_lines):
                if all_lines[i].line_type == "remove":
                    # Collect consecutive removes then adds
                    removes = []
                    while i < len(all_lines) and all_lines[i].line_type == "remove":
                        removes.append(i)
                        i += 1
                    adds = []
                    while i < len(all_lines) and all_lines[i].line_type == "add":
                        adds.append(i)
                        i += 1
                    # Pair them up
                    for r_idx, a_idx in zip(removes, adds):
                        word_pairs[r_idx] = all_lines[a_idx]
                        word_pairs[a_idx] = all_lines[r_idx]
                else:
                    i += 1

        lang = detect_language(file_diff.path) if self.syntax_highlight else None

        flat_idx = 0
        for hunk in file_diff.hunks:
            for dl in hunk.lines:
                self.diff_lines.append(dl)
                self._line_hunk_map.append(hunk)
                line_no_str = self._format_line_no(dl)
                prefix = self._format_prefix(dl)

                if self.word_diff and flat_idx in word_pairs:
                    text = self._format_word_diff_line(
                        dl, word_pairs[flat_idx], line_no_str, prefix
                    )
                elif lang and dl.line_type == "context":
                    text = self._format_syntax_line(dl, line_no_str, prefix, lang)
                elif lang and dl.line_type in ("add", "remove"):
                    style = "reverse green" if dl.line_type == "add" else "reverse red"
                    text = Text(f"{line_no_str}{prefix}{dl.content}", style=style)
                else:
                    text = Text()
                    if dl.line_type == "context":
                        text.append(line_no_str, style="dim")
                    else:
                        text.append(line_no_str)
                    text.append(f"{prefix}{dl.content}")

                w = DiffLineWidget(text)
                if not (lang and dl.line_type in ("add", "remove", "context")):
                    w.add_class(dl.line_type.replace("_", "-"))
                self._line_widgets.append(w)
                self.mount(w)

                # Render inline comments for this line
                line_key = dl.new_line_no if dl.new_line_no is not None else dl.old_line_no
                for c in comment_map.get(line_key, []):
                    if comments_mod.comment_matches_line(c, dl):
                        block = CommentBlock(f"-- {c.comment}")
                        self.mount(block)

                flat_idx += 1

    def _load_side_by_side(
        self,
        file_diff: FileDiff,
        comment_map: dict[int | None, list[ReviewComment]],
    ) -> None:
        half = max(40, (self.size.width - 15) // 2)  # each side's content width
        lang = detect_language(file_diff.path) if self.syntax_highlight else None

        for hunk in file_diff.hunks:
            rows = align_hunk_lines(hunk.lines)
            for row in rows:
                self._sbs_rows.append(row)
                self._line_hunk_map.append(hunk)

                # Primary DiffLine for cursor/comment matching
                primary = row.right or row.left
                assert primary is not None
                self.diff_lines.append(primary)

                # Build Rich Text for the row
                text = self._format_sbs_row(row, half, lang if row.row_type == "context" else None)
                w = DiffLineWidget(text)

                w.add_class("sbs")
                if row.row_type == "hunk_header":
                    w.add_class("hunk-header")
                elif row.row_type == "context":
                    w.add_class("context")

                self._line_widgets.append(w)
                self.mount(w)

                # Render inline comments
                line_key = (
                    primary.new_line_no if primary.new_line_no is not None else primary.old_line_no
                )
                for c in comment_map.get(line_key, []):
                    if comments_mod.comment_matches_line(c, primary):
                        block = CommentBlock(f"-- {c.comment}")
                        self.mount(block)

    def _format_sbs_row(self, row: SideBySideRow, half: int, language: str | None = None) -> Text:
        """Format a side-by-side row as Rich Text with per-side coloring."""
        if row.row_type == "hunk_header":
            left = row.left
            assert left is not None
            return Text(left.content)

        sep = " │ "
        text = Text()

        # Compute word diff segments if applicable
        do_word_diff = (
            self.word_diff
            and row.row_type == "change"
            and row.left is not None
            and row.right is not None
        )
        if do_word_diff:
            old_segs, new_segs = word_diff_segments(row.left.content, row.right.content)

        # Left side
        if row.left:
            old_no = str(row.left.old_line_no) if row.left.old_line_no is not None else ""
            prefix = "-" if row.left.line_type == "remove" else " "
            left_style = "red" if row.left.line_type == "remove" else ""
            text.append(f"{old_no:>4} {prefix}", style=left_style)
            if do_word_diff:
                left_len = 0
                for seg_text, changed in old_segs:
                    chunk = seg_text[: half - left_len]
                    text.append(chunk, style="bold reverse red" if changed else "red")
                    left_len += len(chunk)
                    if left_len >= half:
                        break
                text.append(" " * max(0, half - left_len))
            elif language:
                highlighted = highlight_line(row.left.content[:half], language)
                text.append_text(highlighted)
                text.append(" " * max(0, half - highlighted.cell_len))
            else:
                left_content = row.left.content[:half]
                style = "red" if row.left.line_type == "remove" else ""
                text.append(f"{left_content:{half}}", style=style)
        else:
            text.append(f"{'':>4} {'':>{half + 1}}")

        text.append(sep, style="dim")

        # Right side
        if row.right:
            new_no = str(row.right.new_line_no) if row.right.new_line_no is not None else ""
            prefix = "+" if row.right.line_type == "add" else " "
            right_style = "green" if row.right.line_type == "add" else ""
            text.append(f"{new_no:>4} {prefix}", style=right_style)
            if do_word_diff:
                for seg_text, changed in new_segs:
                    text.append(seg_text[:half], style="bold reverse green" if changed else "green")
            elif language:
                highlighted = highlight_line(row.right.content[:half], language)
                text.append_text(highlighted)
            else:
                right_content = row.right.content[:half]
                style = "green" if row.right.line_type == "add" else ""
                text.append(right_content, style=style)
        else:
            text.append(f"{'':>4} ")

        return text

    def _format_line_no(self, dl: DiffLineModel) -> str:
        old = str(dl.old_line_no) if dl.old_line_no is not None else ""
        new = str(dl.new_line_no) if dl.new_line_no is not None else ""
        if dl.line_type == "hunk_header":
            return ""
        return f"{old:>4} {new:>4} \u2502 "

    def _format_prefix(self, dl: DiffLineModel) -> str:
        if dl.line_type == "add":
            return "+"
        elif dl.line_type == "remove":
            return "-"
        elif dl.line_type == "hunk_header":
            return ""
        return " "

    def _format_syntax_line(
        self,
        dl: DiffLineModel,
        line_no_str: str,
        prefix: str,
        language: str,
    ) -> Text:
        """Render a context line with syntax highlighting."""
        text = Text()
        text.append(f"{line_no_str}{prefix}", style="dim")
        highlighted = highlight_line(dl.content, language)
        text.append_text(highlighted)
        return text

    def _format_word_diff_line(
        self,
        dl: DiffLineModel,
        pair: DiffLineModel,
        line_no_str: str,
        prefix: str,
    ) -> Text:
        """Render a line with word-level diff highlighting."""
        if dl.line_type == "remove":
            old_segs, _ = word_diff_segments(dl.content, pair.content)
            base_style = "red"
            highlight_style = "bold reverse red"
            segs = old_segs
        else:
            _, new_segs = word_diff_segments(pair.content, dl.content)
            base_style = "green"
            highlight_style = "bold reverse green"
            segs = new_segs

        text = Text()
        text.append(f"{line_no_str}{prefix}", style=base_style)
        for seg_text, changed in segs:
            text.append(seg_text, style=highlight_style if changed else base_style)
        return text

    def get_current_hunk(self) -> tuple[FileDiff, DiffHunk] | None:
        if not self._line_hunk_map or not self._current_file_diff:
            return None
        if 0 <= self.cursor_index < len(self._line_hunk_map):
            return (self._current_file_diff, self._line_hunk_map[self.cursor_index])
        return None

    def watch_cursor_index(self, old: int, new: int) -> None:
        if not self._line_widgets:
            return
        if 0 <= old < len(self._line_widgets):
            self._line_widgets[old].remove_class("cursor")
        if 0 <= new < len(self._line_widgets):
            self._line_widgets[new].add_class("cursor")
            self._line_widgets[new].scroll_visible()

    def move_cursor(self, delta: int) -> None:
        if not self._line_widgets:
            return
        new_idx = max(0, min(len(self._line_widgets) - 1, self.cursor_index + delta))
        self.cursor_index = new_idx

    def jump_to_next_hunk(self, forward: bool = True) -> None:
        if not self.diff_lines:
            return
        start = self.cursor_index
        step = 1 if forward else -1
        idx = start + step
        while 0 <= idx < len(self.diff_lines):
            if self.diff_lines[idx].line_type == "hunk_header":
                self.cursor_index = idx
                return
            idx += step

    def jump_to_next_comment(self, forward: bool = True) -> None:
        """Jump to the next/previous line that has a comment."""
        app: NitApp = self.app  # type: ignore[assignment]
        if not self.diff_lines or not app.current_file:
            return
        file_comments = [c for c in app.comments if c.file_path == app.current_file.path]
        if not file_comments:
            return

        commented_lines: set[tuple[int | None, int | None]] = set()
        for c in file_comments:
            commented_lines.add((c.new_line_no, c.old_line_no))

        start = self.cursor_index
        step = 1 if forward else -1
        idx = start + step
        while 0 <= idx < len(self.diff_lines):
            dl = self.diff_lines[idx]
            if (dl.new_line_no, dl.old_line_no) in commented_lines:
                self.cursor_index = idx
                return
            idx += step

    def show_comment_input(self) -> None:
        if self._active_input is not None:
            return
        if not self._line_widgets or self.cursor_index >= len(self._line_widgets):
            return
        widget = self._line_widgets[self.cursor_index]
        self._active_input = InlineCommentInput(
            placeholder="comment (enter to submit, esc to cancel)"
        )
        self.mount(self._active_input, after=widget)
        self._active_input.focus()

    def hide_comment_input(self) -> None:
        if self._active_input is not None:
            self._active_input.remove()
            self._active_input = None
            self.focus()

    def get_current_line(self) -> DiffLineModel | None:
        if 0 <= self.cursor_index < len(self.diff_lines):
            return self.diff_lines[self.cursor_index]
        return None

    def get_hunk_context(self, center_idx: int, radius: int = 2) -> list[str]:
        start = max(0, center_idx - radius)
        end = min(len(self.diff_lines), center_idx + radius + 1)
        return [self.diff_lines[i].content for i in range(start, end)]


# --- App ---


class NitApp(App):
    TITLE = "nit"
    COMMANDS = set()
    ENABLE_COMMAND_PALETTE = False
    theme = "textual-ansi"
    CSS = """
    #frame {
        border: round $accent;
        height: 1fr;
        width: 1fr;
    }
    #status-bar {
        dock: top;
        height: 1;
    }
    .status-segment {
        padding: 0 1;
        text-style: bold;
        width: 1fr;
        overflow: hidden;
    }
    #seg-branch {
        background: ansi_magenta;
        color: ansi_black;
    }
    #seg-mode {
        background: $secondary;
        color: ansi_black;
    }
    #seg-files {
        background: $primary;
        color: ansi_black;
    }
    #seg-comments {
        background: $warning;
        color: ansi_black;
    }
    #layout {
        width: 100%;
        height: 1fr;
    }
    #sidebar {
        width: 40;
        dock: left;
        border: round $border;
        overflow-x: hidden;
    }
    #sidebar:focus {
        border: round $border;
    }
    DiffView {
        border: round $border;
    }
    Footer {
        background: ansi_bright_black;
    }
    Footer .footer-key--key {
        background: ansi_bright_black;
    }
    Footer .footer-key--description {
        background: ansi_bright_black;
    }
    """

    BINDINGS = [
        Binding("q", "quit", "Quit", show=False),
        Binding("j", "cursor_down", "Down", show=False),
        Binding("k", "cursor_up", "Up", show=False),
        Binding("J", "next_hunk", "Next hunk", show=False),
        Binding("K", "prev_hunk", "Prev hunk", show=False),
        Binding("n", "next_file", "Next file"),
        Binding("p", "prev_file", "Prev file"),
        Binding("c", "comment", "Comment"),
        Binding("d", "delete_comment", "Delete"),
        Binding("m", "cycle_mode", "Mode"),
        Binding("r", "refresh", "Refresh", show=False),
        Binding("right_square_bracket", "next_comment", "]", show=False),
        Binding("left_square_bracket", "prev_comment", "[", show=False),
        Binding("G", "cursor_end", "End", show=False),
        Binding("s", "toggle_side_by_side", "Split"),
        Binding("w", "toggle_word_diff", "Word"),
        Binding("W", "toggle_whitespace", "Whitespace"),
        Binding("e", "export_comments", "Export", show=False),
        Binding("h", "toggle_syntax", "Highlight", show=False),
    ]

    diff_mode: reactive[str] = reactive("branch")
    ignore_whitespace: reactive[bool] = reactive(False)

    def __init__(self, cli_args: CLIArgs | None = None, *args, **kwargs) -> None:
        kwargs.setdefault("ansi_color", True)
        super().__init__(*args, **kwargs)
        self._cli_args = cli_args or CLIArgs()
        self._pending_g = False
        self._commit_input: CommitInput | None = None
        self.current_file: FileDiff | None = None
        self.file_diffs: list[FileDiff] = []
        self.comments: list[ReviewComment] = []
        self.repo_root = None
        self.branch = ""
        self.base = ""
        self._file_index: int = 0
        self._last_raw_diff: str = ""
        self._file_review_mode: bool = False

    def compose(self) -> ComposeResult:
        with Vertical(id="frame"):
            with Horizontal(id="status-bar"):
                yield Label("", id="seg-branch", classes="status-segment")
                yield Label("", id="seg-mode", classes="status-segment")
                yield Label("", id="seg-files", classes="status-segment")
                yield Label("", id="seg-comments", classes="status-segment")
            with Horizontal(id="layout"):
                sidebar_tree = FileTree("Files", id="sidebar")
                sidebar_tree.guide_depth = 2
                yield sidebar_tree
                yield DiffView(id="diff-view")
        yield Footer()

    def on_mount(self) -> None:
        if self._cli_args.file_path:
            self._mount_file_review()
            return
        try:
            self.repo_root = git.get_repo_root()
        except Exception:
            self.notify("Not a git repository", severity="error")
            self.exit()
            return
        try:
            self.branch = git.get_current_branch(self.repo_root)
        except Exception:
            self.branch = "(detached HEAD)"
        self.base = git.get_main_branch(self.repo_root)
        if self._cli_args.mode:
            self.diff_mode = self._cli_args.mode
        elif self.branch and self.branch == self.base:
            self.diff_mode = "unstaged"
        try:
            self.comments = comments_mod.load_comments(self.repo_root)
        except Exception:
            logger.warning("Failed to load comments, starting with empty list")
            self.notify("Could not load .nit.json — starting with no comments", severity="warning")
            self.comments = []
        self._load_diff()
        self.set_interval(5.0, self._auto_refresh_poll)

    def _mount_file_review(self) -> None:
        from pathlib import Path

        file_path = self._cli_args.file_path
        p = Path(file_path)
        if not p.exists():
            self.notify(f"File not found: {file_path}", severity="error")
            self.exit()
            return
        self._file_review_mode = True
        self.branch = str(p.name)
        self.diff_mode = "file"
        try:
            self.repo_root = git.get_repo_root()
        except Exception:
            self.repo_root = p.parent
        try:
            self.comments = comments_mod.load_comments(self.repo_root)
        except Exception:
            self.comments = []
        content = p.read_text()
        self.file_diffs = file_to_diff(str(p), content)
        self._update_file_list()
        self._update_status()

    def _auto_refresh_poll(self) -> None:
        if self._file_review_mode:
            return
        if self._commit_input is not None:
            return
        dv = self.query_one("#diff-view", DiffView)
        if dv._active_input is not None:
            return
        try:
            raw = self._get_raw_diff()
        except Exception:
            return
        if raw != self._last_raw_diff:
            self._last_raw_diff = raw
            self.file_diffs = parse_diff(raw)
            self._update_file_list()
            self._update_status()

    def _get_raw_diff(self) -> str:
        if self.repo_root is None:
            return ""
        cwd = self.repo_root
        path_filter = self._cli_args.path_filter
        try:
            ws = self.ignore_whitespace
            if self._cli_args.commit_range:
                return git.get_commit_range_diff(
                    self._cli_args.commit_range, cwd, path_filter, ignore_whitespace=ws
                )
            elif self.diff_mode == "branch":
                return git.get_branch_diff(cwd, path_filter, ignore_whitespace=ws)
            elif self.diff_mode == "unstaged":
                return git.get_unstaged_diff(cwd, path_filter, ignore_whitespace=ws)
            elif self.diff_mode == "staged":
                return git.get_staged_diff(cwd, path_filter, ignore_whitespace=ws)
            else:
                return git.get_unpushed_diff(cwd, path_filter, ignore_whitespace=ws)
        except subprocess.CalledProcessError as e:
            msg = (e.stderr or "").strip() or "Failed to load diff"
            logger.warning("Git diff failed: %s", msg)
            self.notify(msg, severity="error")
            return ""

    def _load_diff(self) -> None:
        raw = self._get_raw_diff()
        self._last_raw_diff = raw
        self.file_diffs = parse_diff(raw)
        self._update_file_list()
        self._update_status()

    def _build_file_tree(self) -> None:
        """Populate the sidebar tree with files grouped by directory."""
        tree = self.query_one("#sidebar", FileTree)
        tree.clear()
        tree.show_root = False

        # Group files by directory
        dirs: dict[str, list[tuple[int, FileDiff]]] = {}
        for i, fd in enumerate(self.file_diffs):
            dirname = os.path.dirname(fd.path) or "."
            dirs.setdefault(dirname, []).append((i, fd))

        comment_counts = Counter(c.file_path for c in self.comments)

        dir_nodes: dict[str, object] = {}
        tree_order: list[int] = []
        for dirname in sorted(dirs):
            parts = dirname.split("/")
            current = tree.root
            for depth, part in enumerate(parts):
                key = "/".join(parts[: depth + 1])
                if key not in dir_nodes:
                    label = Text(f"{part}/", style="dim")
                    node = current.add(label, expand=True)
                    node.data = None
                    dir_nodes[key] = node
                current = dir_nodes[key]

            for i, fd in dirs[dirname]:
                label = _file_label(fd, comment_counts[fd.path])
                leaf = current.add_leaf(label)
                leaf.data = i
                tree_order.append(i)

        self._tree_order = tree_order

    def _update_file_list(self) -> None:
        self._build_file_tree()
        if self.file_diffs:
            restore_idx = 0
            if self.current_file:
                for i, fd in enumerate(self.file_diffs):
                    if fd.path == self.current_file.path:
                        restore_idx = i
                        break
            self._file_index = restore_idx
            self._select_file(restore_idx)
        else:
            self.current_file = None
            self.query_one("#diff-view", DiffView).clear_diff()

    def _refresh_file_labels(self) -> None:
        """Rebuild tree labels without changing selection."""
        self._build_file_tree()

    def _update_status(self) -> None:
        if self._cli_args.commit_range:
            mode_label = self._cli_args.commit_range
        else:
            mode_label = DIFF_MODE_LABELS.get(self.diff_mode, self.diff_mode)
        n_comments = len(self.comments)
        n_files = len(self.file_diffs)
        branch_display = self.branch
        if len(branch_display) > 30:
            branch_display = branch_display[:29] + "…"
        self.query_one("#seg-branch", Label).update(f"⎇ {branch_display}")
        ws_indicator = " [no-ws]" if self.ignore_whitespace else ""
        self.query_one("#seg-mode", Label).update(f"⇄  {mode_label}{ws_indicator}")
        self.query_one("#seg-files", Label).update(f"▤ {n_files} files")
        self.query_one("#seg-comments", Label).update(f"✎ {n_comments} comments")

    def _highlight_tree_node(self, file_index: int) -> None:
        """Move tree cursor to the node matching the given file index."""
        tree = self.query_one("#sidebar", FileTree)
        for line in range(tree.last_line + 1):
            node = tree.get_node_at_line(line)
            if node and node.data == file_index:
                tree.cursor_line = line
                tree.scroll_to_line(line)
                break

    def _select_file(self, index: int) -> None:
        if 0 <= index < len(self.file_diffs):
            self.current_file = self.file_diffs[index]
            file_comments = [c for c in self.comments if c.file_path == self.current_file.path]
            self.query_one("#diff-view", DiffView).load_file_diff(self.current_file, file_comments)
            self._highlight_tree_node(index)
            self.query_one("#diff-view", DiffView).focus()

    def on_tree_node_selected(self, event: Tree.NodeSelected) -> None:
        if event.node.data is not None:
            self._file_index = event.node.data
            self._select_file(event.node.data)

    # --- Actions ---

    def action_cursor_down(self) -> None:
        self.query_one("#diff-view", DiffView).move_cursor(1)

    def action_cursor_up(self) -> None:
        self.query_one("#diff-view", DiffView).move_cursor(-1)

    def action_cursor_end(self) -> None:
        dv = self.query_one("#diff-view", DiffView)
        if dv._line_widgets:
            dv.cursor_index = len(dv._line_widgets) - 1

    def on_key(self, event: "events.Key") -> None:
        if self._pending_g:
            self._pending_g = False
            g_actions = {
                "g": self.action_cursor_start,
                "a": self._action_stage_hunk,
                "u": self._action_unstage_hunk,
                "x": self._action_discard_hunk,
                "c": self._action_commit_prompt,
            }
            action = g_actions.get(event.key)
            if action:
                action()
                event.prevent_default()
                event.stop()
                return
        if event.key == "g":
            self._pending_g = True
            event.prevent_default()
            event.stop()

    def action_cursor_start(self) -> None:
        dv = self.query_one("#diff-view", DiffView)
        if dv._line_widgets:
            dv.cursor_index = 0

    def action_next_hunk(self) -> None:
        self.query_one("#diff-view", DiffView).jump_to_next_hunk(forward=True)

    def action_prev_hunk(self) -> None:
        self.query_one("#diff-view", DiffView).jump_to_next_hunk(forward=False)

    def action_next_file(self) -> None:
        order = getattr(self, "_tree_order", [])
        if not order:
            return
        try:
            pos = order.index(self._file_index)
        except ValueError:
            pos = -1
        new_pos = (pos + 1) % len(order)
        self._file_index = order[new_pos]
        self._select_file(self._file_index)

    def action_prev_file(self) -> None:
        order = getattr(self, "_tree_order", [])
        if not order:
            return
        try:
            pos = order.index(self._file_index)
        except ValueError:
            pos = 0
        new_pos = (pos - 1) % len(order)
        self._file_index = order[new_pos]
        self._select_file(self._file_index)

    def action_next_comment(self) -> None:
        self.query_one("#diff-view", DiffView).jump_to_next_comment(forward=True)

    def action_prev_comment(self) -> None:
        self.query_one("#diff-view", DiffView).jump_to_next_comment(forward=False)

    def action_cycle_mode(self) -> None:
        if self._file_review_mode or self._cli_args.commit_range:
            return
        idx = DIFF_MODES.index(self.diff_mode)
        self.diff_mode = DIFF_MODES[(idx + 1) % len(DIFF_MODES)]
        self._load_diff()

    def action_refresh(self) -> None:
        if self.repo_root:
            self.comments = comments_mod.load_comments(self.repo_root)
        self._load_diff()

    def action_comment(self) -> None:
        diff_view = self.query_one("#diff-view", DiffView)
        line = diff_view.get_current_line()
        if line is None or line.line_type == "hunk_header":
            return
        if self.current_file is None:
            return
        diff_view.show_comment_input()

    def on_input_submitted(self, event: Input.Submitted) -> None:
        if isinstance(event.input, CommitInput):
            msg = event.value.strip()
            self.hide_commit_input()
            if msg and self.repo_root:
                try:
                    git.commit(msg, cwd=self.repo_root)
                    self.notify("Committed")
                except subprocess.CalledProcessError as e:
                    self.notify(f"Commit failed: {(e.stderr or '').strip()}", severity="error")
                self._load_diff()
            return
        if not isinstance(event.input, InlineCommentInput):
            return
        diff_view = self.query_one("#diff-view", DiffView)
        text = event.value.strip()
        if text and self.current_file and self.repo_root:
            line = diff_view.get_current_line()
            if line:
                context = diff_view.get_hunk_context(diff_view.cursor_index)
                comment = comments_mod.make_comment(
                    self.current_file.path,
                    line,
                    text,
                    context,
                    diff_mode=self.diff_mode,
                )
                self.comments.append(comment)
                try:
                    comments_mod.save_comments(
                        self.repo_root, self.comments, self.branch, self.base
                    )
                except Exception as e:
                    self.notify(f"Failed to save comment: {e}", severity="error")
        saved_cursor = diff_view.cursor_index
        diff_view.hide_comment_input()
        if self.current_file:
            file_comments = [c for c in self.comments if c.file_path == self.current_file.path]
            diff_view.load_file_diff(self.current_file, file_comments, restore_cursor=saved_cursor)
            self._update_status()
            self._refresh_file_labels()

    def action_toggle_whitespace(self) -> None:
        self.ignore_whitespace = not self.ignore_whitespace
        self._load_diff()

    def action_toggle_word_diff(self) -> None:
        dv = self.query_one("#diff-view", DiffView)
        dv.word_diff = not dv.word_diff
        if self.current_file:
            saved_cursor = dv.cursor_index
            file_comments = [c for c in self.comments if c.file_path == self.current_file.path]
            dv.load_file_diff(self.current_file, file_comments, restore_cursor=saved_cursor)

    def action_toggle_syntax(self) -> None:
        dv = self.query_one("#diff-view", DiffView)
        dv.syntax_highlight = not dv.syntax_highlight
        if self.current_file:
            saved_cursor = dv.cursor_index
            file_comments = [c for c in self.comments if c.file_path == self.current_file.path]
            dv.load_file_diff(self.current_file, file_comments, restore_cursor=saved_cursor)

    def action_toggle_side_by_side(self) -> None:
        dv = self.query_one("#diff-view", DiffView)
        dv.side_by_side = not dv.side_by_side
        if self.current_file:
            saved_cursor = dv.cursor_index
            file_comments = [c for c in self.comments if c.file_path == self.current_file.path]
            dv.load_file_diff(self.current_file, file_comments, restore_cursor=saved_cursor)

    # --- Git operations (g-prefixed chords) ---

    def _action_stage_hunk(self) -> None:
        if self._file_review_mode:
            return
        if self.diff_mode not in ("unstaged", "all"):
            self.notify("Stage: switch to unstaged/all mode", severity="warning")
            return
        dv = self.query_one("#diff-view", DiffView)
        result = dv.get_current_hunk()
        if result is None:
            return
        file_diff, hunk = result
        patch = build_patch(file_diff, hunk)
        try:
            git.apply_patch(patch, cwd=self.repo_root, cached=True)
            self.notify("Hunk staged")
        except subprocess.CalledProcessError as e:
            self.notify(f"Stage failed: {(e.stderr or '').strip()}", severity="error")
            return
        self._load_diff()

    def _action_unstage_hunk(self) -> None:
        if self._file_review_mode:
            return
        if self.diff_mode != "staged":
            self.notify("Unstage: switch to staged mode", severity="warning")
            return
        dv = self.query_one("#diff-view", DiffView)
        result = dv.get_current_hunk()
        if result is None:
            return
        file_diff, hunk = result
        patch = build_patch(file_diff, hunk)
        try:
            git.apply_patch(patch, cwd=self.repo_root, cached=True, reverse=True)
            self.notify("Hunk unstaged")
        except subprocess.CalledProcessError as e:
            self.notify(f"Unstage failed: {(e.stderr or '').strip()}", severity="error")
            return
        self._load_diff()

    def _action_discard_hunk(self) -> None:
        if self._file_review_mode:
            return
        if self.diff_mode not in ("unstaged", "all"):
            self.notify("Discard: switch to unstaged/all mode", severity="warning")
            return
        dv = self.query_one("#diff-view", DiffView)
        result = dv.get_current_hunk()
        if result is None:
            return
        file_diff, hunk = result
        patch = build_patch(file_diff, hunk)
        try:
            git.apply_patch(patch, cwd=self.repo_root, reverse=True)
            self.notify("Hunk discarded")
        except subprocess.CalledProcessError as e:
            self.notify(f"Discard failed: {(e.stderr or '').strip()}", severity="error")
            return
        self._load_diff()

    def _action_commit_prompt(self) -> None:
        if self._file_review_mode:
            return
        if self._commit_input is not None:
            return
        self._commit_input = CommitInput(
            placeholder="commit message (enter to commit, esc to cancel)"
        )
        self.mount(self._commit_input, before=self.query_one(Footer))
        self._commit_input.focus()

    def hide_commit_input(self) -> None:
        if self._commit_input is not None:
            self._commit_input.remove()
            self._commit_input = None
            self.query_one("#diff-view", DiffView).focus()

    def action_delete_comment(self) -> None:
        diff_view = self.query_one("#diff-view", DiffView)
        line = diff_view.get_current_line()
        if line is None or self.current_file is None or self.repo_root is None:
            return
        # Find and remove comments matching this line
        before = len(self.comments)
        self.comments = [
            c
            for c in self.comments
            if not (
                c.file_path == self.current_file.path and comments_mod.comment_matches_line(c, line)
            )
        ]
        if len(self.comments) < before:
            saved_cursor = diff_view.cursor_index
            comments_mod.save_comments(self.repo_root, self.comments, self.branch, self.base)
            file_comments = [c for c in self.comments if c.file_path == self.current_file.path]
            diff_view.load_file_diff(self.current_file, file_comments, restore_cursor=saved_cursor)
            self._update_status()
            self._refresh_file_labels()

    def action_export_comments(self) -> None:
        if not self.comments:
            self.notify("No comments to export", severity="warning")
            return
        text = comments_mod.export_comments_markdown(self.comments)
        clip_cmd = None
        for cmd in ("pbcopy", "xclip", "xsel"):
            if shutil.which(cmd):
                clip_cmd = [cmd] if cmd == "pbcopy" else [cmd, "-selection", "clipboard"]
                break
        if clip_cmd is None:
            self.notify("No clipboard command found (pbcopy/xclip/xsel)", severity="error")
            return
        try:
            subprocess.run(clip_cmd, input=text, text=True, check=True, timeout=5)
            self.notify(f"Copied {len(self.comments)} comments to clipboard")
        except (subprocess.CalledProcessError, FileNotFoundError, subprocess.TimeoutExpired):
            self.notify("Clipboard copy failed", severity="error")


def _export_on_quit(args: CLIArgs, comments: list[ReviewComment]) -> None:
    if not args.export_comments or not comments:
        return
    if args.export_format == "json":
        text = comments_mod.export_comments_json(comments)
    else:
        text = comments_mod.export_comments_markdown(comments)
    if args.export_comments == "-":
        print(text)
    else:
        Path(args.export_comments).write_text(text)


def main() -> None:
    from .cli import parse_args

    args = parse_args()
    logging.basicConfig(
        level=logging.DEBUG if args.verbose else logging.WARNING,
        format="%(name)s: %(message)s",
        stream=__import__("sys").stderr,
    )
    app = NitApp(cli_args=args)
    app.run()
    _export_on_quit(args, app.comments)


if __name__ == "__main__":
    main()
