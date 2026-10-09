# git-prev-branch

[![Go Reference](https://pkg.go.dev/badge/github.com/matbur/git-prev-branch.svg)](https://pkg.go.dev/github.com/matbur/git-prev-branch)
[![Release](https://img.shields.io/github/v/release/matbur/git-prev-branch?sort=semver)](https://github.com/matbur/git-prev-branch/releases)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Docs Check](https://github.com/matbur/git-prev-branch/actions/workflows/readme-sync.yaml/badge.svg)](https://github.com/matbur/git-prev-branch/actions/workflows/readme-sync.yaml)
[![Go Build](https://github.com/matbur/git-prev-branch/actions/workflows/ci.yaml/badge.svg)](https://github.com/matbur/git-prev-branch/actions/workflows/ci.yaml)

<p align="center">
  <img src="assets/icon-anim.gif" alt="git-prev-branch icon" width="120">
</p>

[🇬🇧 English](README.md) | **[🇵🇱 Polski](README.pl.md)**

Proste narzędzie CLI napisane w Go, które pokazuje, z której gałęzi przełączyłeś się na bieżącą. Przydatne przy ustawianiu gałęzi bazowej podczas tworzenia pull requestów.

## Funkcje

- **Śledzi historię gałęzi Gita**: znajduje poprzednio aktywną gałąź na podstawie historii checkout/switch.
- **Przyjazny dla skryptów**: tylko nazwa gałęzi trafia na `stdout`, monity na `stderr`. Użyj `--yes` (`-y`), by pominąć monity. Zob. [Wyjście i monity](#wyjście-i-monity).
- **Działa z GitHub CLI**: można używać z `gh pr create --base $(git-prev-branch)`. Zob. [Użycie z GitHub CLI](#użycie-z-github-cli).
- **Cofanie w historii**: użyj indeksu pozycyjnego (`0`, `1`, `2`, ...), by cofnąć się o n kroków.
- **Działa z dowolnego miejsca**: wskaż inne repozytorium Git flagą `--path` (`-p`).
- **Konfigurowalny**: zachowanie można ustawić w pliku YAML.
- **Można używać jako biblioteki Go**: zaimportuj `git-prev-branch` do własnych narzędzi Go. Zob. [Użycie jako biblioteka Go](#użycie-jako-biblioteka-go).

## Instalacja

Możesz zainstalować `git-prev-branch` na jeden z poniższych sposobów:

### 1. Docker

Gotowy obraz jest publikowany na [Docker Hub](https://hub.docker.com/r/matbur/git-prev-branch). Zawiera wyłącznie Gita i binarkę `git-prev-branch`. Zamontuj bieżące repozytorium w `/repo` i utwórz alias:

```bash
alias git-prev-branch='docker run --rm -it -v "$PWD":/repo matbur/git-prev-branch'
```

Do skryptów opuść `-it` (np. `docker run --rm -v "$PWD":/repo matbur/git-prev-branch`). Nie są wymagane ani Git, ani Go. Dla repozytorium poza bieżącym katalogiem zamontuj je i przekaż `--path`. Plik konfiguracyjny można zamontować w `/home/uid1000/.config/git-prev-branch`.

### 2. Homebrew

```bash
brew install matbur/tap/git-prev-branch
```

Instaluje gotowe binarki dla macOS i Linuksa (amd64 i arm64) z [`matbur/homebrew-tap`](https://github.com/matbur/homebrew-tap). Nie wymaga toolchainu Go; `brew upgrade` pobiera nowe wydania.

### 3. Instalacja przez Go

Wymaga zainstalowanego w systemie [Go](https://go.dev/dl/).

```bash
go install github.com/matbur/git-prev-branch/cmd/git-prev-branch@latest
```

Binarka trafi do `$GOPATH/bin` (lub `$GOBIN`). Upewnij się, że ten katalog jest w `PATH`.

### 4. Kompilacja ze źródeł

```bash
git clone https://github.com/matbur/git-prev-branch.git
cd git-prev-branch
make build
```

Powstała binarka będzie dostępna w bieżącym katalogu. Możesz ją przenieść do lokalizacji w `PATH`.

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
| `--debug` | `-d` | Wypisuje na `stderr` decyzje: który plik konfiguracji został znaleziony, a jeśli żaden — że działają wartości domyślne, oraz które polecenia `git` zostały uruchomione i czy się powiodły. `stdout`, monity i kody wyjścia pozostają bez zmian, więc skrypty mogą w całości zignorować ten flag. |
| `--version` | `-v` | Wypisuje `git-prev-branch <wersja>` na `stdout` i kończy z kodem `0`, nie czytając repozytorium ani konfiguracji. Wersja jest ustalana przy budowie: `make build` i `make build-dist` wklejają `git describe` (najnowszy tag `vX.Y.Z` plus dystans, sam commit gdy nie znano taga); w przeciwnym razie wersja pochodzi z informacji o module — binarka zainstalowana przez `go install ...@vX.Y.Z` albo `go get` + `go build` zgłasza wersję modułu, zwykły `go build` w repozytorium git zgłasza tag lub pseudo-wersję wyprowadzoną z repozytorium, a budowa bez żadnych informacji (np. poza repozytorium) zwraca `dev`. |

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

> **Uwaga:** polecenie mieszka w pakiecie `cmd/git-prev-branch` jako `package main`, którego nie da się zaimportować, więc biblioteką jest podpakiet `github.com/matbur/git-prev-branch/gitprevbranch`. Błędy są zgłaszane jako wartości strażnicze — `ErrNotGitRepo`, `ErrNoPreviousBranch`, `ErrInvalidIndex`, `ErrGitCommand` — sprawdzalne przez `errors.Is`.

## Rozwój

### Wymagania wstępne

- [Go](https://go.dev/) 1.27 lub nowszy
- [Git](https://git-scm.com/)

### Lokalna konfiguracja rozwoju

```bash
git clone https://github.com/matbur/git-prev-branch.git
cd git-prev-branch
```

Polecenie znajduje się w `cmd/git-prev-branch/`, biblioteka w `gitprevbranch/`, a obsługa konfiguracji w `internal/config/`; testy każdego pakietu leżą obok niego.

> **Uwaga:** `make check` uruchamia każdą lokalną kontrolę — `gofmt`, `go vet`, testy Go oraz kontrole skryptów Pythona — a `make help` wypisuje wszystkie dostępne cele.

## Testowanie

Zestaw testów zapewnia poprawność i niezawodność narzędzia:

- **Testy jednostkowe** – weryfikują logikę rdzeniową w izolacji (wyznaczanie historii gałęzi, parsowanie argumentów, obsługa konfiguracji itd.).
- **Testy integracyjne** – weryfikują zachowanie na prawdziwych repozytoriach Git, obejmując realistyczne przepływy pracy.

> **Uwaga:** testy korzystają z pakietu `testing` z biblioteki standardowej oraz z `github.com/stretchr/testify` do asercji i uruchamiają się na prawdziwych, tymczasowych repozytoriach Git; uruchom je poleceniem `make test`.

## Jakość i automatyzacja

Aby utrzymać wysoką jakość kodu i usprawnić wydania, projekt korzysta z następującej automatyzacji:

| Obszar | Podejście | Korzyści |
|---|---|---|
| **CI dla kodu** | Workflowy GitHub Actions uruchamiające testy, linting i kompilację krzyżową przy każdym pushu i pull requeście. | Wczesne wykrywanie regresji i spójna kontrola jakości. |
| **Linting** | Analiza statyczna i kontrola stylu (np. `golangci-lint`) egzekwujące najlepsze praktyki Go. | Czystsza, łatwiejsza w utrzymaniu baza kodu. |
| **Buildy wieloplatformowe** | Automatyczna kompilacja krzyżowa dla Linuksa, macOS i Windows (amd64/arm64). | Szeroka zgodność dla użytkowników końcowych. |
| **Tagi wersji** | Nowy tag `vX.Y.Z` przy każdym mergu do `main`, podbijany labelką `major`/`minor`/`patch release` zmerged PR (`patch`, gdy braku brak). | Przewidywalne punkty wydania i proste, przyrostowe wersjonowanie. |
| **Publikowanie wydań** | Zautomatyzowane GitHub Releases (wraz z changelogami i gotowymi plikami binarnymi). | Prosta i przewidywalna dystrybucja. |
| **Publikowanie Dockera** | `Dockerfile` jest budowany i publikowany na Docker Hub przy każdym mergu do `main` oraz przy każdym tagu `vX.Y.Z`. | Gotowy do uruchomienia obraz; instalacja Gita ani Go nie jest potrzebna. |
| **Publikowanie Homebrew** | Job `homebrew` regeneruje `Formula/git-prev-branch.rb` w `matbur/homebrew-tap` i wypycha go przy każdym opublikowanym wydaniu `vX.Y.Z`. | `brew upgrade` podąża za każdym wydaniem; ręczne bumpowanie formuły niepotrzebne. |

> **Uwaga:** joby CI kodu, lintingu, kompilacji krzyżowej i wydań znajdują się w pliku `.github/workflows/ci.yaml` (wraz z jobem `homebrew`), obraz Docker w `.github/workflows/docker.yaml`.



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

`make check` uruchamia `gofmt`, `go vet`, testy Go oraz kontrole skryptów Pythona — `scripts/check_readme_sync.py`, `scripts/test_check_readme_sync.py`, `scripts/test_next_tag.py` i `scripts/test_update_homebrew_formula.py`. Jobów CI, których nie uruchamia, to `lint` (`make lint`, jeśli masz zainstalowany golangci-lint v2), krzyżowo kompilujący job `build`, publikujący wydanie job `tag` oraz wypychający formułę job `homebrew`. Skrypty README nie są równoważne: testy samego checkera mutują dosłowne wiersze skopiowane z obu README, więc edycja jednego z nich powoduje błąd testu, podczas gdy kontrola synchronizacji nadal przechodzi.

## Licencja

Ten projekt jest licencjonowany na licencji MIT. Szczegóły znajdziesz w pliku [LICENSE](LICENSE).
