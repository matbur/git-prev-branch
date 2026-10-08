#!/usr/bin/env python3
"""Self-tests for update_homebrew_formula.py.

Run from anywhere:
    python3 scripts/test_update_homebrew_formula.py
"""

from __future__ import annotations

import tempfile
import unittest
from pathlib import Path

from update_homebrew_formula import (
    TARGETS,
    Formula,
    FormulaError,
    asset_url,
    class_name,
    formula_version,
    parse_checksums,
)

ASSETS = {
    "git-prev-branch-darwin-amd64.tar.gz": "a" * 64,
    "git-prev-branch-darwin-arm64.tar.gz": "b" * 64,
    "git-prev-branch-linux-amd64.tar.gz": "c" * 64,
    "git-prev-branch-linux-arm64.tar.gz": "d" * 64,
}


def sample_formula(**overrides: str) -> Formula:
    fields: dict[str, object] = {
        "name": "git-prev-branch",
        "repo": "matbur/git-prev-branch",
        "desc": "Determine the previous git branch you switched from",
        "homepage": "https://github.com/matbur/git-prev-branch",
        "tag": "v1.2.3",
        "license": "MIT",
        "checksums": dict(ASSETS),
    }
    fields.update(overrides)
    return Formula(**fields)  # type: ignore[arg-type]


class ParseChecksumsTest(unittest.TestCase):
    def test_two_space_separator(self) -> None:
        text = f"{'a' * 64}  git-prev-branch-linux-amd64.tar.gz\n"
        self.assertEqual(
            parse_checksums(text),
            {"git-prev-branch-linux-amd64.tar.gz": "a" * 64},
        )

    def test_binary_mode_separator_and_uppercase(self) -> None:
        text = f"{'AB' * 32} *git-prev-branch-darwin-arm64.tar.gz\n"
        self.assertEqual(
            parse_checksums(text),
            {"git-prev-branch-darwin-arm64.tar.gz": "ab" * 32},
        )

    def test_skips_blank_lines(self) -> None:
        self.assertEqual(parse_checksums("\n  \n"), {})

    def test_rejects_garbage(self) -> None:
        with self.assertRaises(FormulaError):
            parse_checksums("not-a-checksum file.tar.gz")


class NameTest(unittest.TestCase):
    def test_class_name(self) -> None:
        self.assertEqual(class_name("git-prev-branch"), "GitPrevBranch")

    def test_class_name_single_word(self) -> None:
        self.assertEqual(class_name("ripgrep"), "Ripgrep")

    def test_class_name_rejects_separators(self) -> None:
        with self.assertRaises(FormulaError):
            class_name("git prev")


class VersionTest(unittest.TestCase):
    def test_strips_v(self) -> None:
        self.assertEqual(formula_version("v1.2.3"), "1.2.3")

    def test_rejects_other_tags(self) -> None:
        for tag in ("1.2.3", "v1.2", "latest", "v1.2.3-rc1"):
            with self.assertRaises(FormulaError):
                formula_version(tag)


class AssetUrlTest(unittest.TestCase):
    def test_release_download_url(self) -> None:
        self.assertEqual(
            asset_url("matbur/git-prev-branch", "v1.2.3", "git-prev-branch", "linux", "arm64"),
            "https://github.com/matbur/git-prev-branch/releases/download/"
            "v1.2.3/git-prev-branch-linux-arm64.tar.gz",
        )


class RenderTest(unittest.TestCase):
    def test_header(self) -> None:
        text = sample_formula().render()
        self.assertIn("class GitPrevBranch < Formula\n", text)
        self.assertIn('  desc "Determine the previous git branch you switched from"\n', text)
        self.assertIn('  homepage "https://github.com/matbur/git-prev-branch"\n', text)
        self.assertIn('  version "1.2.3"\n', text)
        self.assertIn('  license "MIT"\n', text)

    def test_every_target_block(self) -> None:
        text = sample_formula().render()
        for os_block, cpu_block in (
            ("on_macos", "on_intel"),
            ("on_macos", "on_arm"),
            ("on_linux", "on_intel"),
            ("on_linux", "on_arm"),
        ):
            self.assertIn(f"    {cpu_block} do\n", text)
        self.assertEqual(text.count("    on_intel do"), 2)
        self.assertEqual(text.count("    on_arm do"), 2)
        self.assertEqual(text.count("      url "), 4)
        self.assertEqual(text.count("      sha256 "), 4)
        self.assertIn("  end\n\n  on_linux do\n", text)

    def test_urls_and_checksums(self) -> None:
        # Exact ordered sequence, so a tarball swapped between two blocks of
        # the same OS (assertIn would not notice) fails here.
        text = sample_formula().render()
        pairs = [
            (
                f'      url "https://github.com/matbur/git-prev-branch/releases/download/'
                f'v1.2.3/{asset}"',
                f'      sha256 "{ASSETS[asset]}"',
            )
            for _, _, goos, goarch in TARGETS
            for asset in (f"git-prev-branch-{goos}-{goarch}.tar.gz",)
        ]
        cursor = 0
        for url, sha in pairs:
            url_at = text.find(url, cursor)
            sha_at = text.find(sha, cursor)
            self.assertNotEqual(url_at, -1, f"missing or out of order: {url}")
            self.assertNotEqual(sha_at, -1, f"missing or out of order: {sha}")
            self.assertLess(url_at, sha_at, f"sha256 before url for {url}")
            cursor = sha_at

    def test_install_and_test(self) -> None:
        text = sample_formula().render()
        self.assertIn('    bin.install "git-prev-branch"\n', text)
        self.assertIn(
            '    assert_match version.to_s, shell_output("#{bin}/git-prev-branch --version")\n',
            text,
        )
        self.assertIn(
            '  head "https://github.com/matbur/git-prev-branch.git", branch: "main"\n', text
        )
        # FormulaAudit/ComponentsOrder: head before the first on_* block.
        self.assertLess(text.index('  head "'), text.index("  on_macos do"))

    def test_deterministic(self) -> None:
        self.assertEqual(sample_formula().render(), sample_formula().render())

    def test_trailing_newline_and_no_v_prefix(self) -> None:
        text = sample_formula().render()
        self.assertTrue(text.endswith("end\n"))
        self.assertNotIn('version "v1.2.3"', text)

    def test_missing_checksum_raises(self) -> None:
        partial = {k: v for k, v in ASSETS.items() if "linux-arm64" not in k}
        with self.assertRaises(FormulaError) as ctx:
            sample_formula(checksums=partial).render()
        self.assertIn("git-prev-branch-linux-arm64.tar.gz", str(ctx.exception))


class WriteTest(unittest.TestCase):
    def test_writes_into_formula_dir(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            path = sample_formula().write(Path(tmp))
            self.assertEqual(path, Path(tmp) / "Formula" / "git-prev-branch.rb")
            self.assertEqual(path.read_text(encoding="utf-8"), sample_formula().render())

    def test_overwrites_previous_release(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            sample_formula(tag="v1.2.2").write(Path(tmp))
            path = sample_formula(tag="v1.2.3").write(Path(tmp))
            self.assertIn('version "1.2.3"', path.read_text(encoding="utf-8"))
            self.assertNotIn("1.2.2", path.read_text(encoding="utf-8"))


if __name__ == "__main__":
    unittest.main()
