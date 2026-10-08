#!/usr/bin/env python3
"""Generate a Homebrew formula for a release inside a tap repository.

The release assets are prebuilt tarballs (one per OS/architecture); this
script pairs them with the sha256 checksums from a sha256sum-format file and
writes Formula/<name>.rb into the tap checkout. Name, repository, description
and homepage are flags, so the same script serves any further app of the
general-purpose tap (see README, "Installation -> Homebrew").

The CI wiring lives in the homebrew job of .github/workflows/ci.yaml; the
commit and push into the tap are the caller's responsibility.
"""

from __future__ import annotations

import argparse
import re
import sys
from dataclasses import dataclass
from pathlib import Path

# OS/architecture blocks of the formula, in file order: the Homebrew DSL
# block, the CPU block, and the GOOS/GOARCH of the release tarball.
TARGETS: tuple[tuple[str, str, str, str], ...] = (
    ("on_macos", "on_intel", "darwin", "amd64"),
    ("on_macos", "on_arm", "darwin", "arm64"),
    ("on_linux", "on_intel", "linux", "amd64"),
    ("on_linux", "on_arm", "linux", "arm64"),
)

CHECKSUM_RE = re.compile(r"^([0-9a-fA-F]{64}) [ *](.+)$")
TAG_RE = re.compile(r"^v(\d+\.\d+\.\d+)$")


class FormulaError(ValueError):
    """An input the generator refuses to turn into a formula."""


def parse_checksums(text: str) -> dict[str, str]:
    """Parse `sha256sum`/`shasum -a 256` output into {filename: sha256}.

    Both separators are accepted: two spaces (text mode) and space-star
    (binary mode). Filenames come back lowercased-hex as written; duplicate
    filenames keep the last entry, like `sha256sum -c` would.
    """
    checksums: dict[str, str] = {}
    for line in text.splitlines():
        if not line.strip():
            continue
        match = CHECKSUM_RE.fullmatch(line)
        if match is None:
            raise FormulaError(f"not a sha256sum line: {line!r}")
        checksums[match.group(2)] = match.group(1).lower()
    return checksums


def formula_version(tag: str) -> str:
    """Strip the leading v from a vX.Y.Z tag for the formula's version."""
    match = TAG_RE.fullmatch(tag)
    if match is None:
        raise FormulaError(f"tag must look like vX.Y.Z, got {tag!r}")
    return match.group(1)


def class_name(name: str) -> str:
    """Turn a formula filename (git-prev-branch) into its class (GitPrevBranch)."""
    parts = name.split("-")
    if not name or any(not part.isalnum() for part in parts):
        raise FormulaError(f"name must be alphanumeric dash-separated, got {name!r}")
    return "".join(part.capitalize() for part in parts)


def asset_url(repo: str, tag: str, name: str, goos: str, goarch: str) -> str:
    return f"https://github.com/{repo}/releases/download/{tag}/{name}-{goos}-{goarch}.tar.gz"


@dataclass(frozen=True)
class Formula:
    """Everything the rendered file is built from."""

    name: str
    repo: str
    desc: str
    homepage: str
    tag: str
    license: str
    checksums: dict[str, str]

    @property
    def version(self) -> str:
        return formula_version(self.tag)

    def required_assets(self) -> list[str]:
        return [f"{self.name}-{goos}-{goarch}.tar.gz" for _, _, goos, goarch in TARGETS]

    def render(self) -> str:
        missing = [a for a in self.required_assets() if a not in self.checksums]
        if missing:
            raise FormulaError(f"no checksum for: {', '.join(missing)}")

        lines = [
            f"class {class_name(self.name)} < Formula",
            f'  desc "{self.desc}"',
            f'  homepage "{self.homepage}"',
            f'  version "{self.version}"',
            f'  license "{self.license}"',
            # Homebrew's FormulaAudit wants `head` before any `on_*` block.
            f'  head "https://github.com/{self.repo}.git", branch: "main"',
            "",
        ]
        # One `on_<os>` group per OS, one `on_<cpu>` block per architecture;
        # a blank line between CPU blocks keeps the diff of a new target small.
        for os_index, os_block in enumerate(("on_macos", "on_linux")):
            lines.append(f"  {os_block} do")
            first = True
            for os_b, cpu_block, goos, goarch in TARGETS:
                if os_b != os_block:
                    continue
                if not first:
                    lines.append("")
                first = False
                lines.append(f"    {cpu_block} do")
                lines.append(
                    f'      url "{asset_url(self.repo, self.tag, self.name, goos, goarch)}"'
                )
                asset = f"{self.name}-{goos}-{goarch}.tar.gz"
                lines.append(f'      sha256 "{self.checksums[asset]}"')
                lines.append("    end")
            lines.append("  end")
            if os_index == 0:
                lines.append("")
        lines += [
            "",
            "  def install",
            f'    bin.install "{self.name}"',
            "  end",
            "",
            "  test do",
            f'    assert_match version.to_s, shell_output("#{{bin}}/{self.name} --version")',
            "  end",
            "end",
            "",
        ]
        return "\n".join(lines)

    def write(self, tap_dir: Path) -> Path:
        path = tap_dir / "Formula" / f"{self.name}.rb"
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(self.render(), encoding="utf-8")
        return path


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Write Formula/<name>.rb for a release into a tap checkout."
    )
    parser.add_argument("--name", required=True, help="formula name, e.g. git-prev-branch")
    parser.add_argument("--repo", required=True, help="GitHub repository, e.g. matbur/git-prev-branch")
    parser.add_argument("--desc", required=True, help="one-line formula description")
    parser.add_argument("--homepage", required=True, help="formula homepage URL")
    parser.add_argument("--tag", required=True, help="release tag the assets belong to (vX.Y.Z)")
    parser.add_argument("--license", default="MIT", help="SPDX license id (default: %(default)s)")
    parser.add_argument("--shas", required=True, metavar="FILE", help="sha256sum output for the release assets")
    parser.add_argument("--tap-dir", required=True, metavar="DIR", help="checkout of the tap repository")
    args = parser.parse_args()

    try:
        formula = Formula(
            name=args.name,
            repo=args.repo,
            desc=args.desc,
            homepage=args.homepage,
            tag=args.tag,
            license=args.license,
            checksums=parse_checksums(Path(args.shas).read_text(encoding="utf-8")),
        )
        path = formula.write(Path(args.tap_dir))
    except FormulaError as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 1
    print(path)
    return 0


if __name__ == "__main__":
    sys.exit(main())
