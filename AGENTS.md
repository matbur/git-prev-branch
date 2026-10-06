# AGENTS.md

## Reviews

- Zawsze zapisuj review do `tmp/review.md`, nadpisując plik od zera.
- Nigdy nie czytaj poprzedniej zawartości `tmp/review.md` — review ma opisywać tylko bieżący stan.
- Enumeruj znaleziska: ponumerowana lista, jeden problem na pozycję, z `plik:linia` i konkretną poprawką. Bez akapitów podsumowujących, bez kilku findeów w jednym punkcie — każdy musi dać się podnieść osobno.

## Stan repo

Kod Go jest już w repo (`main.go`, `config/`, `gitprevbranch/` + testy obok pakietów), ale na razie **leży w working tree i nie jest commitowany**. README jest jednocześnie specyfikacją i opisem implementacji — poprawiaj je razem. Wykonywalne źródła prawdy to `Makefile`, `scripts/`, `.github/workflows/`.

## README.md ↔ README.pl.md

Dwa README muszą pozostać strukturalnie identyczne, bo pilnuje tego CI (`.github/workflows/readme-sync.yml`):

- ta sama sekwencja i poziomy nagłówków, ta sama liczba elementów list, wierszy tabel i bloków cytatów,
- **bajtowo identyczne bliki kodu** (obejmuje `gh pr create --base $(git-prev-branch)` bez cudzysłowów),
- ta sama liczba akapitów — złączenie lub rozbicie akapitu to drift; ponowne zawijanie linii to nie drift,
- każdy link `](#anchor)` musi rozwiązywać się w swoim własnym pliku (anchor PL ma polskie diakrytyki),
- tekst prozy może się różnić — to tłumaczenie.

Zawsze po edycji README uruchom:

```bash
make check   # fmt-check + vet + test + check-readme + test-scripts; CI dodatkowo golangci-lint
```

`check_readme_sync.py` czyta ścieżki względem CWD — uruchamiaj z katalogu repozytorium. Wyjście: `0` zgodne, `1` drift, `2` błąd użycia/IO.

## Pułapka: self-test mutuje prawdziwe README

`scripts/test_check_readme_sync.py` robi `str.replace` na kopiach README, używając **dosłownych linii z obecnych plików**. Zmiana dowolnej z tych linii psuje test z komunikatem `mutation target not found`, nawet jeśli struktura jest w porządku. Chodzi m.in. o:

`- [x] Dodać testy jednostkowe i integracyjne`, `git-prev-branch -p /path`, `cd git-prev-branch`, `### 1. Homebrew (zalecany sposób)`, `| \`2\` | 2 kroki wstecz |`, `Powstały plik binarny będzie dostępny`, `## Funkcje`, `| \`0\` | Sukces | nazwa gałęzi |`, `## License`, `](LICENSE)`, `](#kody-wyjscia)`.

Przy zmianie któregokolwiek z tych fragmentów zaktualizuj odpowiedni wpis w `CASES`.

## Inne

- Python stdlib tylko, bez zależności; skrypty z `from __future__ import annotations` i typami w `dataclass`.
- `tmp/` to katalog roboczy (`--write` zapisuje tam `readme.*.skeleton`), ale **nie jest w `.gitignore`** — nie commituj zawartości `tmp/`.
- `make help` wypisuje listę celów.
