# git-prev-branch

[![Go Reference](https://pkg.go.dev/badge/github.com/matbur/git-prev-branch.svg)](https://pkg.go.dev/github.com/matbur/git-prev-branch)
[![Release](https://img.shields.io/github/v/release/matbur/git-prev-branch?sort=semver)](https://github.com/matbur/git-prev-branch/releases)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Docs Check](https://github.com/matbur/git-prev-branch/actions/workflows/readme-sync.yml/badge.svg)](https://github.com/matbur/git-prev-branch/actions/workflows/readme-sync.yml)
[![Go Build](https://github.com/matbur/git-prev-branch/actions/workflows/ci.yml/badge.svg)](https://github.com/matbur/git-prev-branch/actions/workflows/ci.yml)

[🇬🇧 English](README.md) | **[🇵🇱 Polski](README.pl.md)**

Lekkie narzędzie CLI dla użytkowników Gita, napisane w Go. `git-prev-branch` pozwala szybko ustalić, z której gałęzi przełączyłeś się, aby dotrzeć do bieżącej gałęzi — idealne do automatycznego ustawiania gałęzi bazowej przy tworzeniu pull requestów.

## Funkcje

- **Śledzenie historii gałęzi Gita**: wykrywa poprzednio aktywną gałąź na podstawie Twojej historii checkout/switch.
- **Domyślnie przyjazne dla skryptów**: zapisuje wyłącznie nazwę gałęzi na `stdout`, a wszystkie monity na `stderr`, więc `$(git-prev-branch)` i potoki przechwytują wyłącznie nazwę gałęzi. Użyj `--yes` (`-y`), aby całkowicie pominąć monit potwierdzenia — bez tej flagi terminal interaktywny nadal zostanie zapytany. Zob. [Wyjście i monity](#wyjście-i-monity).
- **Przystosowane do GitHub CLI**: działa z `gh pr create --base` i bez dodatkowych flag — zob. [Użycie z GitHub CLI](#użycie-z-github-cli).
- **Elastyczna nawigacja**: cofnij się w historii przełączeń gałęzi, podając indeks pozycyjny (`0`, `1`, `2`, ...).
- **Działa z dowolnego miejsca**: wskaż repozytorium Git znajdujące się w innym katalogu za pomocą flagi `--path` (`-p`).
- **Konfigurowalne zachowanie**: konfiguruj monity interaktywne i wartości domyślne w pliku konfiguracyjnym YAML.
- **Możliwość użycia jako biblioteka Go**: zaimportuj `git-prev-branch` do własnych narzędzi w Go — zob. [Użycie jako biblioteka Go](#użycie-jako-biblioteka-go).

## Instalacja

Możesz zainstalować `git-prev-branch` jedną z poniższych metod:

> **Uwaga:** metoda 3 działa już dziś; metody 1 i 2 wymagają czegoś, czego jeszcze nie ma — opublikowanego wydania. `brew install matbur/homebrew-tap/git-prev-branch` nie działa do czasu utworzenia tapu, a `go install github.com/matbur/git-prev-branch@latest` do czasu pierwszego tagu kończy się błędem o niewłaściwej wersji. Dystrybucja przez Homebrew jest planowana za pośrednictwem osobnego tapu (`matbur/homebrew-tap`); dokładna formuła, konfiguracja tapu i automatyzacja wydania zostaną ustalone w ramach procesu wydania.

### 1. Homebrew (zalecany sposób)

```bash
brew install matbur/homebrew-tap/git-prev-branch
```

### 2. Instalacja przez Go

Wymaga zainstalowanego w systemie [Go](https://go.dev/dl/).

```bash
go install github.com/matbur/git-prev-branch@latest
```

Zainstaluje to plik binarny do katalogu `$GOPATH/bin` (lub `$GOBIN`). Upewnij się, że katalog ten znajduje się w `PATH`.

### 3. Ręczna kompilacja ze źródeł

```bash
git clone https://github.com/matbur/git-prev-branch.git
cd git-prev-branch
go build
```

Powstały plik binarny będzie dostępny w bieżącym katalogu. Możesz go przenieść do lokalizacji na swojej ścieżce `PATH`.

## Użytkowanie

### Tryb interaktywny

Uruchom bez argumentów wewnątrz repozytorium Git:

```bash
git-prev-branch
```

Spowoduje to:

1. Wykrycie bieżącej gałęzi.
2. Wykrycie poprzednio aktywnej gałęzi (domyślnie: 1 krok wstecz w historii przełączeń gałęzi).
3. Wyświetlenie informacji o poprzedniej gałęzi.
4. Monit z prośbą o kontynuację albo przerwanie operacji. Przerwanie kończy się kodem `2` i nie wypisuje nic na `stdout` — zob. [Kody wyjścia](#kody-wyjścia).

### Wyjście i monity

Każde wywołanie podlega dwóm regułom:

- Na `stdout` trafia **zawsze wyłącznie nazwa gałęzi**.
- Monity i inne komunikaty dla użytkownika trafiają na `stderr`. Monit potwierdzenia jest wyświetlany **wtedy i tylko wtedy, gdy `stdin` jest interaktywnym terminalem i nie podano `--yes` (`-y`)**.

Gdy `stdin` nie jest terminalem — jest przekierowany z pliku lub z innego polecenia (`< /dev/null`, `echo | git-prev-branch`, CI) — monit jest automatycznie pomijany, a nazwa gałęzi wypisywana od razu. Przekierowanie ani potok na `stdout` (`> out.txt`, `| cat`) nie zmienia `stdin`, więc żadne z nich nie wyłącza monitu.

Zwróć uwagę, że podstawienie wyniku polecenia i potok są dla narzędzia nierozróżnialne: w obu przypadkach `stdout` jest potokiem, a `stdin` jest dziedziczony z powłoki wywołującej. Tak więc `$(git-prev-branch)` uruchomione z terminala interaktywnego **nadal zadaje pytanie** — na `stderr` — i czeka na Twoją odpowiedź, zanim wypisze nazwę gałęzi. `git-prev-branch | cat` zachowuje się identycznie. Zamiast tego przekaż `--yes` (`-y`), aby odpowiedzieć na monit automatycznie.

#### Kody wyjścia

| Kod | Znaczenie | `stdout` |
|---|---|---|
| `0` | Sukces | nazwa gałęzi |
| `1` | Błąd — nie repozytorium Git, brak poprzedniej gałęzi, nieprawidłowy argument, brakujący lub uszkodzony plik konfiguracyjny | pusty |
| `2` | Przerwano na monicie potwierdzenia | pusty |

Najważniejsza dla skryptów jest ta niezmienna zasada: **niezerowe zakończenie zawsze oznacza, że `stdout` jest pusty.** Nic częściowego nigdy nie trafia na wyjście, zanim monit zostanie rozstrzygnięty.

Kod `2` powstaje wyłącznie w trybie interaktywnym. Gdy `stdin` nie jest terminalem, monit jest pomijany, więc skrypt może zaobserwować jedynie `0` albo `1`. Przekazanie `-y` również ogranicza możliwe wyniki do `0` i `1`.

#### Bezpieczne użycie w skryptach

Podstawienie wyniku polecenia w powłoce odrzuca kod zakończenia. `$(...)` rozwija się do tego, co polecenie wypisało, więc błąd po cichu zamienia się w pusty ciąg znaków, zamiast zatrzymać skrypt:

```bash
base=$(git-prev-branch)      # a failure here is invisible: the status is discarded
gh pr create --base "$base"  # ...and this still runs, with --base set to ""
```

Przechwyć kod zakończenia, zanim użyjesz wartości:

```bash
base=$(git-prev-branch) && gh pr create --base "$base"
```

lub przerwij szybko:

```bash
base=$(git-prev-branch) || exit 1
gh pr create --base "$base"
```

Kod `2` pozwala odróżnić przerwanie od rzeczywistego błędu:

```bash
if base=$(git-prev-branch); then
  gh pr create --base "$base"
else
  case $? in
    2) echo "aborted at the confirmation prompt" ;;
    *) echo "could not determine the base branch" >&2; exit 1 ;;
  esac
fi
```

Możesz też użyć jawnej flagi `--yes` (`-y`), aby automatycznie zaakceptować wykryty wynik i pominąć monit, niezależnie od wykrywania terminala:

```bash
git-prev-branch --yes
# or
git-prev-branch -y
```

Ponieważ `-y` wyłącza monit, kod `2` nie może wystąpić, a strażnik `&&` staje się opcjonalny.

### Cofanie się dalej w historii

Możesz podać liczbę pozycyjną, aby cofnąć się o `n` kroków w historii przełączeń gałęzi.

| Argument | Znaczenie |
|---|---|
| `0` | Bieżąca gałąź |
| `1` | Poprzednio aktywna gałąź (domyślnie) |
| `2` | 2 kroki wstecz |
| `3` | 3 kroki wstecz |
| `...` | `n` kroków wstecz |

```bash
git-prev-branch 0
git-prev-branch 1
git-prev-branch 2
```

Możesz to także połączyć z `--yes`:

```bash
git-prev-branch -y 2
git-prev-branch -y 0
```

### Wskazanie ścieżki repozytorium

Uruchom dla repozytorium Git znajdującego się w innym katalogu:

```bash
git-prev-branch -p /path/to/repository
git-prev-branch --path /path/to/repository
```

### Użycie własnego pliku konfiguracyjnego

Nadpisz domyślne lokalizacje konfiguracji:

```bash
git-prev-branch -c /path/to/config.yaml
git-prev-branch --config /path/to/config.yaml
```

## Użycie z GitHub CLI

`git-prev-branch` współpracuje z [GitHub CLI (`gh`)](https://cli.github.com/), aby ustawić gałąź bazową nowego pull requesta.

```bash
gh pr create --base $(git-prev-branch)
```

Nie są wymagane żadne dodatkowe flagi. Uruchomione z terminala interaktywnego poprosi o potwierdzenie wykrytej gałęzi przed startem `gh` — zob. [Wyjście i monity](#wyjście-i-monity). Podstawienie wyniku polecenia jest tu nieodróżnialne od potoku, więc monit pojawia się w obu przypadkach. Przekaż `--yes` (`-y`), jeśli wolisz go pominąć.

> **Uwaga:** cudzysłowy są celowo pominięte. Nazwa gałęzi Gita nie może zawierać spacji, więc rozwinięcie nie wymaga cudzysłowów — ich pominięcie sprawia też, że forma ta zawodzi głośno: jeśli polecenie nic nie wypisze, `--base` nie dostanie żadnej wartości, a `gh` przerwie z `flag needs an argument: --base`, zamiast po cichu utworzyć pull request z pustą gałęzią bazową.

## Flagi

| Flaga | Krótka | Opis |
|---|---|---|
| `--config` | `-c` | Ścieżka do własnego pliku konfiguracyjnego. Nadpisuje obie domyślne lokalizacje. |
| `--path` | `-p` | Ścieżka do repozytorium Git do przeanalizowania. Domyślnie bieżący katalog roboczy. |
| `--yes` | `-y` | Przyjmuje wykrytą gałąź bez pytania, nawet w terminalu interaktywnym. `stdout` pozostaje bez zmian w obu przypadkach — zawsze trafia tam wyłącznie nazwa gałęzi — więc ta flaga wyłącza monit, a nie wyjście ani komunikaty diagnostyczne. Sprawia też, że kod wyjścia `2` jest nieosiągalny. |

> **Uwaga:** jeśli nie podano argumentu pozycyjnego, domyślną wartością kroku jest `1`. Zachowanie monitów i podział strumieni opisano w sekcji [Wyjście i monity](#wyjście-i-monity).

## Konfiguracja

`git-prev-branch` czyta konfigurację z pierwszej lokalizacji, która istnieje:

### Lokalizacje i kolejność

```text
--config <path>
<root>/.config/git-prev-branch/config.yaml
~/.config/git-prev-branch/config.yaml
```

Lokalizacje nie są scalane: wygrywa pierwsza istniejąca, a pozostałe są ignorowane. `<root>` to najwyższy poziom drzewa roboczego, wyznaczany przez przejście w górę od bieżącego katalogu — dokładnie tak, jak robi to Git; przy `--path` (`-p`) wyszukiwanie podąża za tą ścieżką zamiast za bieżącym katalogiem roboczym. `~` to katalog domowy bieżącego użytkownika, więc ostatnia lokalizacja jest identyczna na każdej platformie — nie sprawdzamy ani `os.UserConfigDir()` (które na macOS zwraca `~/Library/Application Support`), ani `XDG_CONFIG_HOME`.

Plik na poziomie repozytorium jest współdzielony ze wszystkimi, którzy klonują to repozytorium: zapisz go, jeśli chcesz tych ustawień dla wszystkich, albo dodaj go do `.gitignore` tego repozytorium, jeśli dotyczy tylko Ciebie.

Jeśli `--config` (`-c`) wskazuje na plik, którego nie ma, uruchomienie kończy się kodem wyjścia `1`. Dwie domyślne lokalizacje są sprawdzane oportunistycznie — gdy żadna nie istnieje, działają wbudowane wartości domyślne. Uszkodzony plik jest błędem niezależnie od tego, z której lokalizacji pochodzi.

### Przykładowa struktura

Poniższa struktura konfiguracji odpowiada temu, co implementacja czyta dzisiaj. `default_action` jest ustalony, ponieważ zależą od niego kody wyjścia opisane wyżej; dodatkowe klucze mogą pojawić się w przyszłych wersjach.

```yaml
interactive:
  # Behavior when the confirmation prompt is shown
  # Possible values: "accept", "reject"
  default_action: reject
```

- `default_action` — steruje domyślną odpowiedzią na interaktywny monit potwierdzenia (np. czy domyślnie akceptować, czy odrzucać). Domyślnie dostarczana wartość to `reject`, więc samo Enter na monicie przerywa działanie (kod wyjścia `2`). `--yes` (`-y`) całkowicie pomija monit, więc ma pierwszeństwo przed tym ustawieniem.
- Dodatkowe opcje konfiguracji mogą pojawić się w przyszłych wersjach, w zależności od potrzeb implementacji.

## Użycie jako biblioteka Go

`git-prev-branch` jest zaprojektowany tak, aby można go było zaimportować jako bibliotekę Go, co pozwala włączyć jego logikę wykrywania gałęzi do własnych narzędzi.

### Import

```go
import gpb "github.com/matbur/git-prev-branch/gitprevbranch"
```

### Użycie

Biblioteka udostępnia dwie funkcje w pakiecie `gitprevbranch`: `Previous` odwołuje się do bieżącego katalogu roboczego, `PreviousIn` do wskazanego katalogu. Obie zwracają gałąź o `n` kroków wstecz, przy czym `n = 0` oznacza gałąź bieżącą:

```go
prev, err := gpb.Previous(1) // Get previous branch (1 step back)
if err != nil {
    log.Fatal(err)
}
fmt.Println(prev)
```

> **Uwaga:** polecenie mieszka w katalogu głównym repozytorium jako `package main`, którego nie da się zaimportować, więc biblioteką jest podpakiet `github.com/matbur/git-prev-branch/gitprevbranch`. Błędy są zgłaszane jako wartości strażnicze — `ErrNotGitRepo`, `ErrNoPreviousBranch`, `ErrInvalidIndex`, `ErrGitCommand` — sprawdzalne przez `errors.Is`.

## Rozwój

### Wymagania wstępne

- [Go](https://go.dev/) 1.24 lub nowszy
- [Git](https://git-scm.com/)

### Lokalna konfiguracja rozwoju

```bash
git clone https://github.com/matbur/git-prev-branch.git
cd git-prev-branch
```

Polecenie znajduje się w katalogu głównym jako `main.go`, biblioteka w `gitprevbranch/`, a obsługa konfiguracji w `config/`; testy każdego pakietu leżą obok niego.

> **Uwaga:** `make check` uruchamia każdą lokalną kontrolę — `gofmt`, `go vet`, testy Go i obie kontrole README — a `make help` wypisuje wszystkie dostępne cele.

## Testowanie

Zestaw testów zapewnia poprawność i niezawodność narzędzia:

- **Testy jednostkowe** – weryfikują logikę rdzeniową w izolacji (wyznaczanie historii gałęzi, parsowanie argumentów, obsługa konfiguracji itd.).
- **Testy integracyjne** – weryfikują zachowanie na prawdziwych repozytoriach Git, obejmując realistyczne przepływy pracy.

> **Uwaga:** testy korzystają wyłącznie z pakietu `testing` z biblioteki standardowej i uruchamiają się na prawdziwych, tymczasowych repozytoriach Git; uruchom je poleceniem `go test ./...` albo `make test`.

## Jakość i automatyzacja

Aby utrzymać wysoką jakość kodu i usprawnić wydania, projekt korzysta z następującej automatyzacji:

| Obszar | Podejście | Korzyści |
|---|---|---|
| **CI dla kodu** | Workflowy GitHub Actions uruchamiające testy, linting i kompilację krzyżową przy każdym pushu i pull requeście. | Wczesne wykrywanie regresji i spójna kontrola jakości. |
| **Linting** | Analiza statyczna i kontrola stylu (np. `golangci-lint`) egzekwujące najlepsze praktyki Go. | Czystsza, łatwiejsza w utrzymaniu baza kodu. |
| **Buildy wieloplatformowe** | Automatyczna kompilacja krzyżowa dla Linuksa, macOS i Windows (amd64/arm64). | Szeroka zgodność dla użytkowników końcowych. |
| **Publikowanie wydań** | Zautomatyzowane GitHub Releases (wraz z changelogami i gotowymi plikami binarnymi). | Prosta i przewidywalna dystrybucja. |
| **Przygotowanie Homebrew** | Automatyczne aktualizacje formuły w tapie w ramach potoku wydania. | Bezproblemowe aktualizacje dla użytkowników Homebrew. |

> **Uwaga:** joby CI kodu, lintingu i kompilacji krzyżowej znajdują się w pliku `.github/workflows/ci.yml`; publikowanie wydań i tap Homebrew powstaną później.

## Plan rozwoju

- [x] Zaimplementować logikę rdzeniową ustalania poprzedniej gałęzi na podstawie historii Gita
- [x] Dodać parsowanie argumentów wiersza poleceń (indeks pozycyjny i flagi)
- [x] Zaimplementować wyjście przyjazne skryptom (rozdzielenie `stdout`/`stderr` i wykrywanie TTY, aby bezpiecznie działać w `$(...)` i potokach)
- [x] Zaimplementować `--yes`/`-y`, aby przyjmować wykryty wynik bez pytania
- [x] Dodać obsługę własnej ścieżki repozytorium (`--path`/`-p`)
- [x] Zaimplementować obsługę pliku konfiguracyjnego (lokalizacja w repozytorium i użytkownika, nadpisanie `--config`/`-c`) z domyślnymi wartościami dla trybu interaktywnego
- [x] Zdefiniować i ustabilizować publiczne API biblioteki Go (`github.com/matbur/git-prev-branch/gitprevbranch`)
- [x] Dodać testy jednostkowe i integracyjne
- [x] Skonfigurować CI (linting, testy, buildy wieloplatformowe)
- [ ] Przygotować i opublikować tap Homebrew wraz z formułą
- [ ] Wydać pierwszą stabilną wersję wraz z plikami binarnymi

## Współtworzenie

Wkład jest mile widziany! Jeśli chcesz zaproponować zmiany, zgłosić problem lub wskazać usprawnienia:

1. Załóż [issue](https://github.com/matbur/git-prev-branch/issues), aby omówić pomysł.
2. Zrób fork repozytorium i utwórz gałąź funkcji.
3. Wprowadź zmiany w jasnych, dobrze udokumentowanych commitach.
4. Wyślij pull request opisujący motywację i zakres swoich zmian.

Przestrzegaj standardowych konwencji Go i utrzymuj zmiany spójne z celami oraz zakresem projektu. Jeśli edytujesz `README.md`, odwzoruj zmianę w `README.pl.md` i przed wypchnięciem uruchom kontrole:

```bash
make check
```

`make check` uruchamia `gofmt`, `go vet`, testy Go i obie kontrole README — `scripts/check_readme_sync.py` oraz `scripts/test_check_readme_sync.py`. To wszystko, co robi CI, poza jobem `golangci-lint` (`make lint`, jeśli masz zainstalowany golangci-lint v2). Skrypty README nie są równoważne: testy samego checkera mutują dosłowne wiersze skopiowane z obu README, więc edycja jednego z nich powoduje błąd testu, podczas gdy kontrola synchronizacji nadal przechodzi.

## Licencja

Ten projekt jest licencjonowany na licencji MIT. Szczegóły znajdziesz w pliku [LICENSE](LICENSE).
