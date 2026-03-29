from __future__ import annotations

import argparse
import importlib.metadata
from dataclasses import dataclass


@dataclass
class CLIArgs:
    mode: str | None = None
    commit_range: str | None = None
    path_filter: str | None = None
    verbose: bool = False


def build_parser() -> argparse.ArgumentParser:
    try:
        version = importlib.metadata.version("git-nit")
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
        "commit_range",
        nargs="?",
        default=None,
        help="Git commit range, e.g. HEAD~3..HEAD or main..feature",
    )
    return parser


def parse_args(argv: list[str] | None = None) -> CLIArgs:
    parser = build_parser()
    ns = parser.parse_args(argv)
    return CLIArgs(
        mode=ns.mode,
        commit_range=ns.commit_range,
        path_filter=ns.path,
        verbose=ns.verbose,
    )
