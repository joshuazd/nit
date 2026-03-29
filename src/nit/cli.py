from __future__ import annotations

import argparse
import importlib.metadata
from dataclasses import dataclass
from pathlib import Path


@dataclass
class CLIArgs:
    mode: str | None = None
    commit_range: str | None = None
    path_filter: str | None = None
    verbose: bool = False
    file_path: str | None = None
    export_comments: str | None = None
    export_format: str = "markdown"


def build_parser() -> argparse.ArgumentParser:
    try:
        version = importlib.metadata.version("nit-cli")
    except importlib.metadata.PackageNotFoundError:
        version = "0.0.0-dev"

    parser = argparse.ArgumentParser(
        prog="nit",
        description="Terminal diff viewer with inline review comments.",
    )
    parser.add_argument(
        "--version",
        action="version",
        version=f"nit {version}",
    )
    parser.add_argument(
        "--mode",
        choices=["branch", "unstaged", "all"],
        default=None,
        help="Diff mode (default: branch)",
    )
    parser.add_argument(
        "--path",
        default=None,
        help="Filter to specific file or directory path",
    )
    parser.add_argument(
        "-v",
        "--verbose",
        action="store_true",
        default=False,
        help="Enable verbose logging to stderr",
    )
    parser.add_argument(
        "--export-comments",
        nargs="?",
        const="-",
        default=None,
        metavar="FILE",
        help="Export comments on quit (- for stdout, or file path)",
    )
    parser.add_argument(
        "--export-format",
        choices=["markdown", "json"],
        default="markdown",
        help="Comment export format (default: markdown)",
    )
    parser.add_argument(
        "target",
        nargs="?",
        default=None,
        help="File path (review mode) or git commit range (e.g. HEAD~3..HEAD)",
    )
    return parser


def parse_args(argv: list[str] | None = None) -> CLIArgs:
    parser = build_parser()
    ns = parser.parse_args(argv)
    target = ns.target
    file_path = None
    commit_range = None
    if target:
        if Path(target).exists():
            file_path = target
        else:
            commit_range = target
    return CLIArgs(
        mode=ns.mode,
        commit_range=commit_range,
        path_filter=ns.path,
        verbose=ns.verbose,
        file_path=file_path,
        export_comments=ns.export_comments,
        export_format=ns.export_format,
    )
