#!/usr/bin/env python3
"""Self-tests for next_tag.py.

Run from anywhere:
    python3 scripts/test_next_tag.py
"""

from __future__ import annotations

import unittest

from next_tag import bump_from_labels, next_tag, parse_tag


class ParseTagTest(unittest.TestCase):
    def test_plain(self) -> None:
        self.assertEqual(str(parse_tag("v1.2.3")), "v1.2.3")

    def test_rejects_non_semver(self) -> None:
        self.assertIsNone(parse_tag("latest"))
        self.assertIsNone(parse_tag("v1.2"))
        self.assertIsNone(parse_tag("1.2.3"))


class NextTagTest(unittest.TestCase):
    def test_no_tags_starts_at_v0_1_0(self) -> None:
        self.assertEqual(next_tag([]), "v0.1.0")

    def test_bumps_patch(self) -> None:
        self.assertEqual(next_tag(["v1.2.3"]), "v1.2.4")

    def test_ignores_non_semver_refs(self) -> None:
        self.assertEqual(next_tag(["latest", "v1.0.0"]), "v1.0.1")

    def test_picks_newest_numerically(self) -> None:
        self.assertEqual(next_tag(["v10.0.0", "v9.9.9"]), "v10.0.1")

    def test_bumps_patch_into_next_major_within_v0(self) -> None:
        self.assertEqual(next_tag(["v0.9.9"]), "v0.9.10")

    def test_bumps_minor(self) -> None:
        self.assertEqual(next_tag(["v1.2.3"], "minor"), "v1.3.0")

    def test_bumps_major(self) -> None:
        self.assertEqual(next_tag(["v1.2.3"], "major"), "v2.0.0")

    def test_major_resets_from_v0(self) -> None:
        self.assertEqual(next_tag(["v0.9.9"], "major"), "v1.0.0")


class BumpFromLabelsTest(unittest.TestCase):
    def test_no_labels_defaults_to_patch(self) -> None:
        self.assertEqual(bump_from_labels([]), "patch")

    def test_patch_label(self) -> None:
        self.assertEqual(bump_from_labels(["bugfix", "patch release"]), "patch")

    def test_minor_label(self) -> None:
        self.assertEqual(bump_from_labels(["minor release"]), "minor")

    def test_major_label(self) -> None:
        self.assertEqual(bump_from_labels(["major release"]), "major")

    def test_major_wins_over_patch(self) -> None:
        self.assertEqual(bump_from_labels(["patch release", "major release"]), "major")

    def test_handles_non_matching_labels(self) -> None:
        self.assertEqual(bump_from_labels(["bugfix", "no-release"]), "patch")

    def test_case_insensitive_and_colon_forms(self) -> None:
        self.assertEqual(bump_from_labels(["RELEASE: minor"]), "minor")


if __name__ == "__main__":
    unittest.main()
