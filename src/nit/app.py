from __future__ import annotations

import logging
import os
import subprocess
from collections import Counter

from rich.text import Text
from textual.app import App, ComposeResult
from textual.binding import Binding
from textual.containers import Horizontal, Vertical, VerticalScroll
from textual.reactive import reactive
from textual.widgets import Footer, Input, Label, Static, Tree

from . import comments as comments_mod
from . import git
from .cli import CLIArgs
from .diff_parser import parse_diff
from .models import DiffLine as DiffLineModel
from .models import FileDiff, ReviewComment

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

DIFF_MODES = ["branch", "unstaged", "all"]
DIFF_MODE_LABELS = {
    "branch": "branch diff",
    "unstaged": "unstaged",
    "all": "all uncommitted",
}


# --- Widgets ---


class FileTree(Tree):
    """Sidebar file tree with visible cursor."""

    SCOPED_CSS = False
    DEFAULT_CSS = """
    FileTree {
        background: $background;
    }
    FileTree > .tree--cursor {
        background: $panel;
        text-style: bold;
    }
    FileTree > .tree--highlight {
        background: $accent 50%;
    }
    FileTree > .tree--highlight-line {
        background: $accent 20%;
    }
    """

    ICON_NODE = ""
    ICON_NODE_EXPANDED = ""


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
        color: $accent;
        text-style: bold;
    }
    DiffLineWidget.cursor {
        background: $panel;
    }
    DiffLineWidget.add.cursor {
        background: $panel;
        color: $success;
    }
    DiffLineWidget.remove.cursor {
        background: $panel;
        color: $error;
    }
    """

    cursor_index: reactive[int] = reactive(0)

    def __init__(self, *args, **kwargs) -> None:
        super().__init__(*args, **kwargs)
        self.diff_lines: list[DiffLineModel] = []
        self._line_widgets: list[DiffLineWidget] = []
        self._active_input: InlineCommentInput | None = None

    def clear_diff(self) -> None:
        self.diff_lines = []
        self._line_widgets = []
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

        # Build a lookup of comments by line number
        comment_map: dict[int | None, list[ReviewComment]] = {}
        for c in file_comments:
            key = c.new_line_no if c.new_line_no is not None else c.old_line_no
            comment_map.setdefault(key, []).append(c)

        if file_diff.is_binary:
            w = DiffLineWidget("Binary file")
            self.mount(w)
            return

        for hunk in file_diff.hunks:
            for dl in hunk.lines:
                self.diff_lines.append(dl)
                line_no_str = self._format_line_no(dl)
                prefix = self._format_prefix(dl)
                text = f"{line_no_str}{prefix}{dl.content}"
                w = DiffLineWidget(text)
                w.add_class(dl.line_type.replace("_", "-"))
                self._line_widgets.append(w)
                self.mount(w)

                # Render inline comments for this line
                line_key = dl.new_line_no if dl.new_line_no is not None else dl.old_line_no
                for c in comment_map.get(line_key, []):
                    if comments_mod.comment_matches_line(c, dl):
                        block = CommentBlock(f"-- {c.comment}")
                        self.mount(block)

        start = min(restore_cursor, len(self._line_widgets) - 1) if self._line_widgets else 0
        self.cursor_index = start
        if self._line_widgets:
            self._line_widgets[start].add_class("cursor")

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
    theme = "nord"
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
    }
    #seg-branch {
        background: $accent;
        color: $text;
    }
    #seg-mode {
        background: $secondary;
        color: $text;
    }
    #seg-files {
        background: $primary;
        color: $text;
    }
    #seg-comments {
        background: $warning;
        color: $text;
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
    DiffView {
        border: round $border;
    }
    """

    BINDINGS = [
        Binding("q", "quit", "Quit"),
        Binding("j", "cursor_down", "Down", show=False),
        Binding("k", "cursor_up", "Up", show=False),
        Binding("J", "next_hunk", "Next hunk", show=False),
        Binding("K", "prev_hunk", "Prev hunk", show=False),
        Binding("n", "next_file", "Next file"),
        Binding("p", "prev_file", "Prev file"),
        Binding("c", "comment", "Comment"),
        Binding("d", "delete_comment", "Delete comment"),
        Binding("m", "cycle_mode", "Mode"),
        Binding("r", "refresh", "Refresh"),
        Binding("right_square_bracket", "next_comment", "]", show=False),
        Binding("left_square_bracket", "prev_comment", "[", show=False),
    ]

    diff_mode: reactive[str] = reactive("branch")

    def __init__(self, cli_args: CLIArgs | None = None, *args, **kwargs) -> None:
        super().__init__(*args, **kwargs)
        self._cli_args = cli_args or CLIArgs()
        self.current_file: FileDiff | None = None
        self.file_diffs: list[FileDiff] = []
        self.comments: list[ReviewComment] = []
        self.repo_root = None
        self.branch = ""
        self.base = ""
        self._file_index: int = 0

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
        try:
            self.repo_root = git.get_repo_root()
        except Exception:
            self.notify("Not a git repository", severity="error")
            self.exit()
            return
        try:
            self.branch = git.get_current_branch(self.repo_root)
        except Exception:
            self.branch = ""
        self.base = git.get_main_branch(self.repo_root)
        if self._cli_args.mode:
            self.diff_mode = self._cli_args.mode
        try:
            self.comments = comments_mod.load_comments(self.repo_root)
        except Exception:
            logger.warning("Failed to load comments, starting with empty list")
            self.notify("Could not load .nit.json — starting with no comments", severity="warning")
            self.comments = []
        self._load_diff()

    def _load_diff(self) -> None:
        if self.repo_root is None:
            return
        cwd = self.repo_root
        path_filter = self._cli_args.path_filter

        try:
            if self._cli_args.commit_range:
                raw = git.get_commit_range_diff(self._cli_args.commit_range, cwd, path_filter)
            elif self.diff_mode == "branch":
                raw = git.get_branch_diff(cwd, path_filter)
            elif self.diff_mode == "unstaged":
                raw = git.get_unstaged_diff(cwd, path_filter)
            else:
                raw = git.get_all_uncommitted_diff(cwd, path_filter)
        except subprocess.CalledProcessError as e:
            msg = (e.stderr or "").strip() or "Failed to load diff"
            logger.warning("Git diff failed: %s", msg)
            self.notify(msg, severity="error")
            raw = ""

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
        self.query_one("#seg-branch", Label).update(f"⎇ {self.branch}")
        self.query_one("#seg-mode", Label).update(f"⇄  {mode_label}")
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

    def action_next_hunk(self) -> None:
        self.query_one("#diff-view", DiffView).jump_to_next_hunk(forward=True)

    def action_prev_hunk(self) -> None:
        self.query_one("#diff-view", DiffView).jump_to_next_hunk(forward=False)

    def action_next_file(self) -> None:
        if self._file_index < len(self.file_diffs) - 1:
            self._file_index += 1
            self._select_file(self._file_index)

    def action_prev_file(self) -> None:
        if self._file_index > 0:
            self._file_index -= 1
            self._select_file(self._file_index)

    def action_next_comment(self) -> None:
        self.query_one("#diff-view", DiffView).jump_to_next_comment(forward=True)

    def action_prev_comment(self) -> None:
        self.query_one("#diff-view", DiffView).jump_to_next_comment(forward=False)

    def action_cycle_mode(self) -> None:
        if self._cli_args.commit_range:
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
                comments_mod.save_comments(self.repo_root, self.comments, self.branch, self.base)
        saved_cursor = diff_view.cursor_index
        diff_view.hide_comment_input()
        if self.current_file:
            file_comments = [c for c in self.comments if c.file_path == self.current_file.path]
            diff_view.load_file_diff(self.current_file, file_comments, restore_cursor=saved_cursor)
            self._update_status()
            self._refresh_file_labels()

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


if __name__ == "__main__":
    main()
