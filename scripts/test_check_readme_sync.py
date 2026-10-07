#!/usr/bin/env python3
"""Self-test for check_readme_sync.py.

A checker that never fails is worse than no checker: it manufactures false
confidence that the two language versions are in sync. These cases mutate a
throwaway copy of the READMEs and assert that the checker notices -- and, just
as importantly, that it stays quiet about prose that legitimately differs.

Run from anywhere:
    python3 scripts/test_check_readme_sync.py
"""

from __future__ import annotations

import shutil
import subprocess
import sys
import tempfile
from pathlib import Path

REPO = Path(__file__).resolve().parent.parent
CHECKER = "scripts/check_readme_sync.py"
DOCS = ("README.md", "README.pl.md")

# (name, file, old, new, expect_drift, expected_substrings)
CASES = [
    ("identical copies pass", "README.md", None, None, False, ["is in sync"]),
    (
        "dropped list item in PL",
        "README.pl.md",
        "- [x] Dodać testy jednostkowe i integracyjne\n",
        "",
        True,
        ["structure drift", "-LI"],
    ),
    (
        "altered line inside a PL code block",
        "README.pl.md",
        "git-prev-branch -p /path",
        "git-prev-branch --path /path",
        True,
        ["structure drift", "-CODE git-prev-branch -p /path"],
    ),
    (
        "inserted line inside a PL code block",
        "README.pl.md",
        "cd git-prev-branch\n",
        "cd git-prev-branch\ngo vet ./...\n",
        True,
        ["structure drift", "+CODE go vet ./..."],
    ),
    (
        "broken PL anchor (diacritic dropped)",
        "README.pl.md",
        "](#kody-wyjścia)",
        "](#kody-wyjscia)",
        True,
        ["broken internal anchors in README.pl.md", "unresolved internal link #kody-wyjscia"],
    ),
    (
        "section added to EN only",
        "README.md",
        "## License",
        "## FAQ\n\n## License",
        True,
        ["structure drift", "-H2"],
    ),
    (
        "heading level changed in PL",
        "README.pl.md",
        "### 1. Instalacja przez Go",
        "## 1. Instalacja przez Go",
        True,
        ["structure drift"],
    ),
    (
        "dropped table row in PL",
        "README.pl.md",
        "| `2` | 2 kroki wstecz |\n",
        "",
        True,
        ["structure drift", "-TROW"],
    ),
    (
        "renamed asset referenced in PL only",
        "README.pl.md",
        "[LICENSE](LICENSE)",
        "[LICENSE](LICENSE.md)",
        True,
        ["in-repo link targets differ", "only in README.pl.md: LICENSE.md"],
    ),
    # Regression: the original awk implementation leaked its "previous token"
    # state across code fences, so a paragraph directly after ``` was never
    # counted and edits there slipped through unnoticed.
    (
        "paragraph inserted after a code fence in PL",
        "README.pl.md",
        "Powstały plik binarny będzie dostępny",
        "Dodatkowa nota.\n\nPowstały plik binarny będzie dostępny",
        True,
        ["structure drift", "+PARA"],
    ),
    # Negative controls: legitimate translation differences must NOT trip it.
    (
        "prose-only heading change tolerated",
        "README.pl.md",
        "## Funkcje",
        "## Funkcje oraz możliwości",
        False,
        ["is in sync"],
    ),
    (
        "reworded table cell tolerated",
        "README.pl.md",
        "| `0` | Sukces | nazwa gałęzi |",
        "| `0` | Sukces | nazwagałęzi |",
        False,
        ["is in sync"],
    ),
]


def run_checker(workdir: Path) -> tuple[int, str]:
    proc = subprocess.run(
        [sys.executable, CHECKER],
        cwd=workdir,
        capture_output=True,
        text=True,
    )
    return proc.returncode, proc.stdout + proc.stderr


def main() -> int:
    failures: list[str] = []
    passed = 0

    with tempfile.TemporaryDirectory() as tmp:
        work = Path(tmp)
        (work / "scripts").mkdir()
        shutil.copy(REPO / CHECKER, work / CHECKER)
        shutil.copy(REPO / DOCS[0], work / DOCS[0])
        shutil.copy(REPO / DOCS[1], work / DOCS[1])

        for name, target, old, new, expect_drift, expected in CASES:
            shutil.copy(REPO / DOCS[0], work / DOCS[0])
            shutil.copy(REPO / DOCS[1], work / DOCS[1])

            if old is not None and new is not None:
                path = work / target
                text = path.read_text(encoding="utf-8")
                if old not in text:
                    failures.append(f"{name}: mutation target not found in {target}")
                    continue
                path.write_text(text.replace(old, new, 1), encoding="utf-8")

            code, output = run_checker(work)
            drifted = code != 0

            if drifted != expect_drift:
                failures.append(
                    f"{name}: expected {'FAIL' if expect_drift else 'PASS'}, got exit {code}"
                )
                continue

            missing = [needle for needle in expected if needle not in output]
            if missing:
                failures.append(f"{name}: output missing {missing}")
                continue

            passed += 1
            print(f"  PASS  {name}")

    print()
    print(f"passed={passed} failed={len(failures)}")
    for failure in failures:
        print(f"  {failure}")
    return 1 if failures else 0


if __name__ == "__main__":
    raise SystemExit(main())
