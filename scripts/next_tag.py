#!/usr/bin/env python3
"""Print the next release tag (vX.Y.Z) for the current git state.

The tag is derived from the newest existing vX.Y.Z tag by bumping it
(patch by default, or minor/major when passed as the first argument); with no
such tag the initial v0.1.0 is printed. In CI the bump type comes from the
merged PR labels (see the tag job in .github/workflows/ci.yaml). Creating and
pushing the printed tag is left to the caller.
"""

from __future__ import annotations

import argparse
import re
import subprocess
import sys
from dataclasses import dataclass

TAG_RE = re.compile(r"^v(\d+)\.(\d+)\.(\d+)$")


@dataclass(frozen=True, order=True)
class Version:
    """A semantic version parsed from a vX.Y.Z tag."""

    major: int
    minor: int
    patch: int

    def bumped(self, bump: str) -> Version:
        """Return this version bumped by the given type (patch, minor or major)."""
        if bump == "major":
            return Version(self.major + 1, 0, 0)
        if bump == "minor":
            return Version(self.major, self.minor + 1, 0)
        return Version(self.major, self.minor, self.patch + 1)

    def __str__(self) -> str:
        return f"v{self.major}.{self.minor}.{self.patch}"


def parse_tag(tag: str) -> Version | None:
    """Parse a raw tag name into a Version, or None when it is not vX.Y.Z."""
    match = TAG_RE.fullmatch(tag)
    if match is None:
        return None
    return Version(*(int(part) for part in match.groups()))


def read_tags(repo_path: str = ".") -> list[str]:
    """Return the tags of repo_path that start with a digit after the v."""
    out = subprocess.run(
        ["git", "-C", repo_path, "tag", "--list", "v[0-9]*"],
        check=True,
        capture_output=True,
        text=True,
    ).stdout
    return out.splitlines()


def next_tag(tags: list[str], bump: str = "patch") -> str:
    """Compute the tag to create next, given the tags that already exist.

    The bump type is one of "patch", "minor" or "major"; "minor"/"major"
    reset the trailing numbers to zero.
    """
    versions = [version for tag in tags if (version := parse_tag(tag)) is not None]
    if not versions:
        return "v0.1.0"
    return str(max(versions).bumped(bump))


def bump_from_labels(labels: list[str], default: str = "patch") -> str:
    """Map PR labels to a bump type, highest-priority first.

    A label is recognized when its words contain the bump type and "release",
    so "major release", "release minor" and "release: patch" all work.
    """
    for bump in ("major", "minor", "patch"):
        for label in labels:
            words = label.lower().replace(":", " ").split()
            if "release" in words and bump in words:
                return bump
    return default


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Print the vX.Y.Z tag to create next for the current git state."
    )
    parser.add_argument(
        "bump",
        nargs="?",
        choices=("patch", "minor", "major"),
        default="patch",
        help="bump type when --labels is not given (default: %(default)s)",
    )
    parser.add_argument(
        "--labels",
        metavar="LABELS",
        help="comma-separated PR labels; a release label (major/minor/patch release) picks the bump type",
    )
    args = parser.parse_args()

    if args.labels:
        bump = bump_from_labels([label for label in args.labels.split(",") if label])
    else:
        bump = args.bump
    print(next_tag(read_tags(), bump))
    return 0


if __name__ == "__main__":
    sys.exit(main())
