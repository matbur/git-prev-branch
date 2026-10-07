#!/usr/bin/env python3
"""Verify that README.pl.md stays in sync with README.md.

The Polish README is a translation, so its prose differs by design. What must
NOT differ is the document skeleton: the sequence and nesting of headings, the
number of list items, table shapes, blockquotes, and -- most importantly -- the
exact contents of every fenced code block, since those are the parts readers
copy and paste verbatim.

This script compares that skeleton and also checks that every internal
`](#anchor)` link in both files resolves to a heading in the same file, using
GitHub's anchor rules (lowercase, punctuation dropped, spaces become hyphens,
duplicate headings get a -1, -2, ... suffix).

Implemented in Python rather than awk so that the result is identical on macOS
and on Linux CI runners, whose default awk implementations differ.

Usage:
    python3 scripts/check_readme_sync.py            # check both files
    python3 scripts/check_readme_sync.py --write    # refresh tmp/*.skeleton

Exit status: 0 in sync, 1 on drift, 2 on a usage/IO error.
"""

from __future__ import annotations

import re
import sys
import tempfile
from collections import Counter
from dataclasses import dataclass, field
from pathlib import Path

EN = "README.md"
PL = "README.pl.md"
SNAP_DIR = "tmp"

# A line opening or closing a fenced code block.
FENCE_RE = re.compile(r"^```")
HEADING_RE = re.compile(r"^(#{1,6}) ")
HR_RE = re.compile(r"^[ \t]*(?:-{3,}|\*{3,}|_{3,})[ \t]*$")
QUOTE_RE = re.compile(r"^>")
BULLET_RE = re.compile(r"^[ \t]*[-*+] ")
ORDERED_RE = re.compile(r"^[ \t]*[0-9]+\. ")
# A GitHub table separator, e.g. `|---|---|` or `| :--- | ---: |`
TABLE_SEP_RE = re.compile(r"^[ \t]*\|[ \t]*:?-+:?[ \t]*(?:\|[ \t]*:?-+:?[ \t]*)*\|[ \t]*$")
TABLE_ROW_RE = re.compile(r"^[ \t]*\|")
# A markdown link or image target: ](...)
LINK_RE = re.compile(r"\]\(([^)\s]*)\)")
# An internal anchor reference, e.g. ](#kody-wjścia)
ANCHOR_RE = re.compile(r"\]\(#([^\s)]+)\)")
EXTERNAL_SCHEME_RE = re.compile(r"^(?:https?|mailto|ftp):")


@dataclass
class Report:
    """Accumulates failures so every check runs before reporting.

    Each failure is a header line plus indented detail lines, so the report
    reads with the same `ok    ` / `FAIL  ` rhythm as the progress lines.
    """

    failures: list[tuple[str, list[str]]] = field(default_factory=list)

    def fail(self, header: str, *details: str) -> None:
        self.failures.append((header, list(details)))

    @property
    def ok(self) -> bool:
        return not self.failures

    def render(self) -> str:
        lines = []
        for header, details in self.failures:
            lines.append(f"FAIL  {header}")
            lines.extend(f"        {detail}" for detail in details)
        return "\n".join(lines)


def skeleton(text: str) -> list[str]:
    """Reduce a README to a list of structural tokens.

    Prose collapses to one PARA token per paragraph, so re-wrapping a paragraph
    does not register as drift -- but splitting or merging a paragraph does.
    Fenced code lines keep their full text: those are copy-paste payloads.
    """
    tokens: list[str] = []
    in_fence = False
    previous = ""

    for raw in text.splitlines():
        line = raw.rstrip()

        if in_fence:
            if FENCE_RE.match(line):
                in_fence = False
                tokens.append("FENCE-END")
            else:
                tokens.append(f"CODE {line}")
            continue

        if FENCE_RE.match(line):
            in_fence = True
            tokens.append(f"FENCE-BEGIN {line[3:].strip()}")
            previous = "fence"
            continue

        if not line.strip():
            # A blank line ends the current paragraph. Without this, a wrapped
            # paragraph and two paragraphs joined by a blank line would collapse
            # into one token and splitting a paragraph would go unnoticed.
            previous = ""
            continue

        if HEADING_RE.match(line):
            tokens.append(f"H{len(HEADING_RE.match(line).group(1))}")
            previous = "heading"
            continue

        if HR_RE.match(line):
            tokens.append("HR")
            previous = "hr"
            continue

        if QUOTE_RE.match(line):
            tokens.append("BQ")
            previous = "quote"
            continue

        if BULLET_RE.match(line):
            tokens.append("LI")
            previous = "li"
            continue

        if ORDERED_RE.match(line):
            tokens.append("OLI")
            previous = "oli"
            continue

        if TABLE_SEP_RE.match(line):
            tokens.append("TSEP")
            previous = "tsep"
            continue

        if TABLE_ROW_RE.match(line):
            tokens.append("TROW")
            previous = "trow"
            continue

        if previous != "para":
            tokens.append("PARA")
        previous = "para"

    return tokens


def slugify(heading: str) -> str:
    """Reproduce GitHub's anchor slug for a heading.

    Lowercase, drop punctuation, spaces become hyphens. Letters are compared
    with str.isalnum, which is Unicode-aware, so Polish diacritics survive.
    """
    kept = [c for c in heading.lower() if c.isalnum() or c in " _-"]
    return "".join(kept).replace(" ", "-")


def heading_slugs(text: str) -> set[str]:
    """All anchors the headings in `text` define, including duplicate suffixes."""
    slugs: set[str] = set()
    counts: Counter[str] = Counter()
    in_fence = False

    for raw in text.splitlines():
        line = raw.rstrip()
        if FENCE_RE.match(line):
            in_fence = not in_fence
            continue
        if in_fence:
            continue
        match = HEADING_RE.match(line)
        if not match:
            continue

        base = slugify(HEADING_RE.sub("", line, count=1))
        n = counts[base]
        counts[base] += 1
        slugs.add(base if n == 0 else f"{base}-{n}")

    return slugs


def unresolved_anchors(path: Path, text: str) -> list[str]:
    """Internal `#anchor` links in `text` that no heading in `text` defines."""
    defined = heading_slugs(text)
    bad: list[str] = []
    in_fence = False

    for raw in text.splitlines():
        line = raw.rstrip()
        if FENCE_RE.match(line):
            in_fence = not in_fence
            continue
        if in_fence:
            continue
        for anchor in ANCHOR_RE.findall(line):
            if anchor not in defined:
                bad.append(f"{path.name}: unresolved internal link #{anchor}")

    return bad


def in_repo_targets(text: str) -> set[str]:
    """Relative link targets (no scheme, not an anchor): images, LICENSE, ..."""
    return {
        target
        for target in LINK_RE.findall(text)
        if target and not target.startswith("#") and not EXTERNAL_SCHEME_RE.match(target)
    }


def render_diff(en_tokens: list[str], pl_tokens: list[str]) -> str:
    """Unified diff of the two skeletons, as text."""
    import difflib

    return "".join(
        difflib.unified_diff(
            [f"{t}\n" for t in en_tokens],
            [f"{t}\n" for t in pl_tokens],
            fromfile=EN,
            tofile=PL,
            n=2,
        )
    )


def main(argv: list[str]) -> int:
    write = False
    for arg in argv[1:]:
        if arg == "--write":
            write = True
        elif arg in ("-h", "--help"):
            print(__doc__)
            return 0
        else:
            print(f"usage: {Path(argv[0]).name} [--write]", file=sys.stderr)
            return 2

    try:
        en_text = Path(EN).read_text(encoding="utf-8")
        pl_text = Path(PL).read_text(encoding="utf-8")
    except OSError as exc:
        print(f"error: {exc} (run from the repository root)", file=sys.stderr)
        return 2

    report = Report()
    en_tokens = skeleton(en_text)
    pl_tokens = skeleton(pl_text)

    # 1. In-repo link targets must match, so a renamed asset (LICENSE, an
    #    image) cannot be updated in one language only.
    en_assets = in_repo_targets(en_text)
    pl_assets = in_repo_targets(pl_text)
    if en_assets != pl_assets:
        report.fail(
            f"in-repo link targets differ between {EN} and {PL}",
            *(f"only in {EN}: {target}" for target in sorted(en_assets - pl_assets)),
            *(f"only in {PL}: {target}" for target in sorted(pl_assets - en_assets)),
        )
    else:
        print(f"ok    in-repo link targets match ({len(en_assets)} unique)")

    # 2. Every internal anchor must resolve within its own file.
    for name, text in ((EN, en_text), (PL, pl_text)):
        broken = unresolved_anchors(Path(name), text)
        if broken:
            report.fail(f"broken internal anchors in {name}", *broken)
        else:
            print(f"ok    all internal anchors resolve in {name}")

    # 3. The document skeletons must be identical.
    drift = render_diff(en_tokens, pl_tokens)
    if drift:
        report.fail(
            f"structure drift between {EN} and {PL}",
            f"(- is {EN}, + is {PL})",
            *drift.splitlines(),
        )
    else:
        print(f"ok    structure matches ({EN} ⇄ {PL}, {len(en_tokens)} blocks)")

    if write:
        snap = Path(SNAP_DIR)
        snap.mkdir(exist_ok=True)
        (snap / "readme.en.skeleton").write_text("\n".join(en_tokens) + "\n", encoding="utf-8")
        (snap / "readme.pl.skeleton").write_text("\n".join(pl_tokens) + "\n", encoding="utf-8")
        print(f"wrote {SNAP_DIR}/readme.en.skeleton, {SNAP_DIR}/readme.pl.skeleton")

    if not report.ok:
        print()
        print(report.render())
        print()
        print(f"{PL} has drifted from {EN} — resync the translation")
        return 1

    print(f"{PL} is in sync with {EN}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
