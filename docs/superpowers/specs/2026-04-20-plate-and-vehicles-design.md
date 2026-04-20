# Plate & Vehicles — Design

Date: 2026-04-20
Status: Approved — ready for implementation planning

## Goals

Add two top-level commands to chli:

1. `chli plate` — a per-canton dispatcher that, given a Swiss number plate, explains
   where and how to request holder information (Halterauskunft) for the correct
   cantonal authority. It never submits the form, never pays, never retrieves
   holder data. It is a researched, machine-readable index of the 26 cantonal
   processes.
2. `chli vehicles` — aggregate vehicle statistics pulled from the BFS MOFIS
   dataset on opendata.swiss: current stock and new registrations, filtered and
   grouped by canton, fuel, make, type, and period.

Plus a GitHub Action that verifies the cantonal endpoints on every tagged
release and weekly.

## Non-Goals

- Automating any cantonal web form. Every canton that publishes a
  Halterauskunft service gates it behind payment, captcha, and a stated reason
  under SVG Art. 104 / VZV Art. 126. We never bypass these.
- Returning individual holder names, addresses, or any personal data. The
  dispatcher response is purely procedural.
- CO₂/emissions data or per-model breakdowns beyond what BFS publishes in
  MOFIS. Out of scope for v1.
- LINDAS-backed vehicle statistics. The v1 data source is opendata.swiss CKAN.
  LINDAS can be added later if the existing client proves limiting.

## Architecture

chli uses a flat `api/*.go` layout (one file per data source plus a matching
`_types.go`) and a single external dependency (Cobra). This feature follows
that pattern — no subpackages, no YAML library.

New files:

```text
cmd/
  plate.go                Cobra root for `chli plate` + `lookup` behaviour.
  plate_verify.go         `chli plate verify [--all|--canton XX]`, used by CI.
  vehicles.go             Cobra root + `stock`, `registrations` subcommands.

api/
  plate.go                Plate parsing, canton detection, dispatcher.
  plate_types.go          CantonEntry, HalterauskunftEntry, enums.
  plate_cantons.go        JSON loader + go:embed + invariant checks.
  plate_cantons.json      go:embed'd — 26 canton entries.
  plate_verify.go         HTTP endpoint verifier (observational only).
  plate_test.go           Parser and loader tests.
  vehicles.go             Vehicles client (wraps existing opendata CKAN).
  vehicles_types.go       Row types, filter/group structs.
  vehicles_test.go        CSV filter / group tests against fixtures.

.github/workflows/
  plate-verify.yml        release:published + weekly cron + workflow_dispatch.
```

Cache TTL constants (`plateCacheTTL`, `vehiclesCacheTTL`) are added to
`api/cache.go`.

### Design choices

- **`plate` never automates the form.** The dispatcher emits a deep-linked URL
  (where the canton accepts plate via query string) and prints the rest of the
  procedure. ToS and StGB Art. 143bis make anything more aggressive
  inappropriate.
- **`vehicles` reuses `api/opendata`.** CKAN client, cache, formatting are
  already in place. MOFIS datasets are published there as CSV. Fewer moving
  parts than bringing LINDAS in on day one.
- **Canton data is JSON + go:embed.** chli's dependency policy is Cobra-only,
  so YAML is off the table. JSON is stdlib (`encoding/json` +
  `DisallowUnknownFields`) and diffs cleanly. Research notes live in a
  dedicated `notes` field per canton rather than file comments.
- **`chli plate verify` is a subcommand, not a second binary.** Same loader,
  same types, one implementation. CI calls it with `-o json`.

## `chli plate` — Detailed Behaviour

### Input parsing

Accepted forms:

| Input                                    | Interpretation                             |
| ---------------------------------------- | ------------------------------------------ |
| `chli plate ZH123456`                    | Canton `ZH`, plate digits `123456`         |
| `chli plate "ZH 123 456"`                | Whitespace normalised                      |
| `chli plate ZH-123-456`                  | Hyphens normalised                         |
| `chli plate zh123456`                    | Case-insensitive canton code               |
| `chli plate 120120 --canton AG,AI,FR`    | Fulltext mode, explicit canton list        |
| `chli plate 120120`                      | Error: no prefix and no `--canton`         |
| `chli plate XY123456`                    | Error: `XY` is not a valid canton          |
| `chli plate ZHABC`                       | Warning: non-digit body, still dispatches  |

The 26 valid codes are hardcoded as a set of `const` values. Canton detection
is a pure string operation — no API call.

### Dispatcher response

Per `(plate, canton)` combination, print a record like:

```text
Plate:         ZH 123 456
Canton:        Zürich (ZH) — Strassenverkehrsamt Zürich
Service:       Halterauskunft online
URL:           https://halterauskunft.zh.ch/?plate=ZH123456
Cost:          CHF 13 per lookup
Payment:       Twint, Mastercard, Visa, PostFinance Card
Auth:          hCaptcha + stated reason (accident/damage/legal)
Processing:    Instant (online), PDF emailed
Legal basis:   SVG Art. 104 / VZV Art. 126
Data verified: 2026-04-20 (source: https://www.zh.ch/.../halterauskunft.html)
Note:          SMS/email authentication not required.

This is a reference tool. Holder data is released only via the cantonal
process above.
```

Postal-only cantons keep the same shape with `URL` pointing at the form PDF,
`Payment` showing `invoice / prepaid`, `Processing` showing `5–10 business
days`, and an additional `Postal:` block with the street address.

### Plate flags

- `--open` — open the URL in the default browser (`open` / `xdg-open` / `start`).
- `-o json|yaml|md|csv` — machine-readable; same per-canton record shape.
- `--lang de|fr|it|en|rm` — selects language-tagged string fields from the
  canton data file; falls back to `de` when a language variant is missing.
- `--no-privacy-notice` — suppress the trailing reminder (JSON never prints it
  anyway).

### Offline behaviour

The dispatcher makes no network calls in the happy path — all data is embedded.
`--open` invokes the OS opener; `plate verify` does make HTTP calls but is a
separate subcommand.

## `chli vehicles` — Detailed Behaviour

### Data source

The BFS MOFIS dataset on opendata.swiss, accessed via the existing
`api/opendata` CKAN client. Two CSV resources:

- Vehicle stock — quarterly snapshot by canton × vehicle-type × fuel × make.
- New registrations — monthly new entries, same dimensions.

Resource IDs are pinned as `const` at the top of `api/vehicles.go` so a BFS
URL change is a one-line edit.

### Subcommands

```bash
chli vehicles stock --canton ZH --fuel electric
chli vehicles stock --canton ZH,BE,GE --make Tesla --as-of 2026-Q1
chli vehicles stock --type motorcycle --fuel petrol

chli vehicles registrations --canton ZH --make Tesla --from 2026-01 --to 2026-03
chli vehicles registrations --fuel electric --from 2025-01
chli vehicles registrations --make Renault --model Zoe
```

### Vehicles flags

- `--canton XX[,YY,...]` — multi-canton filter; omit for national total.
- `--fuel petrol|diesel|electric|hybrid|gas|hydrogen|other` — mapped to BFS
  codes internally.
- `--type car|truck|motorcycle|bus|tractor|trailer` — Fahrzeugart axis.
- `--make <string>` — case-insensitive contains match.
- `--as-of YYYY-QN` (stock) / `--from YYYY-MM --to YYYY-MM` (registrations).
- `--group-by canton|fuel|make|type` — pivot dimension.
- `--top N` — keep top N rows after grouping (default 20).

### Output

Table in TTY, JSON when piped. Stock includes the snapshot date; registrations
include the period. A totals row closes each table.

### Caching

24 h TTL, matching the existing opendata source. BFS updates MOFIS monthly or
quarterly.

## Canton Data Schema

`api/plate_cantons.json` (go:embed'd; one top-level object keyed by canton
code):

```json
{
  "schema_version": 1,
  "cantons": {
    "ZH": {
      "names": {
        "de": "Zürich",
        "fr": "Zurich",
        "it": "Zurigo",
        "en": "Zurich"
      },
      "authority": {
        "name": "Strassenverkehrsamt des Kantons Zürich",
        "url": "https://www.zh.ch/de/sicherheit-justiz/strassenverkehrsamt.html",
        "email": "info@stva.zh.ch",
        "phone": "+41 58 811 30 00",
        "postal": {
          "street": "Uetlibergstrasse 301",
          "zip": "8036",
          "city": "Zürich"
        }
      },
      "halterauskunft": {
        "mode": "online",
        "url": "https://halterauskunft.zh.ch",
        "deeplink_template": "https://halterauskunft.zh.ch/?plate={{.PlateNormalized}}",
        "form_pdf": null,
        "cost_chf": 13,
        "payment_methods": ["twint", "mastercard", "visa", "postfinance_card"],
        "auth": {
          "captcha": "hcaptcha",
          "sms": false,
          "email_confirmation": true,
          "requires_stated_reason": true,
          "requires_identification": false
        },
        "processing": {
          "typical": "instant",
          "delivery": "pdf_email"
        },
        "legal_basis": "SVG Art. 104 / VZV Art. 126",
        "notes": {
          "de": "Begründung erforderlich.",
          "en": "Reason required."
        }
      },
      "verification": {
        "last_verified": "2026-04-20",
        "verified_by": "manual",
        "source_urls": [
          "https://halterauskunft.zh.ch"
        ]
      }
    }
  }
}
```

Enum fields:

- `halterauskunft.mode`: `online | postal | mixed | unavailable`
- `halterauskunft.auth.captcha`: `hcaptcha | recaptcha | none`
- `halterauskunft.processing.typical`: `instant | hours | 1-3_days | 5-10_days`
- `halterauskunft.processing.delivery`: `pdf_email | postal | online_portal`
- `verification.verified_by`: `manual | ci`

### Go types (sketch)

```go
type HalterauskunftMode string

const (
    ModeOnline      HalterauskunftMode = "online"
    ModePostal      HalterauskunftMode = "postal"
    ModeMixed       HalterauskunftMode = "mixed"
    ModeUnavailable HalterauskunftMode = "unavailable"
)

type CantonEntry struct {
    Code           string
    Names          map[string]string
    Authority      Authority
    Halterauskunft HalterauskunftEntry
    Verification   Verification
}
```

Plus `Authority`, `Postal`, `Auth`, `Processing`, `Verification` — plain
carriers, no methods beyond `String()`.

### Invariants at load time

Enforced in a `sync.Once`-guarded loader:

- Exactly 26 entries, keys match the canton-code const set.
- `mode in {online, mixed}` → `url` is non-empty.
- `mode in {postal, mixed}` → `authority.postal` is populated.
- `cost_chf >= 0`.
- `verification.last_verified` parses as RFC 3339 date.
- Unknown JSON keys rejected (`json.Decoder.DisallowUnknownFields`).

Malformed JSON aborts the process with a clear error. This is embedded data,
so a malformed file is a build-time bug.

## GitHub Action: Cantonal Endpoint Verification

### Subcommand

```bash
chli plate verify --all                # all 26 cantons
chli plate verify --canton ZH,BE       # targeted
chli plate verify --all -o json        # machine-readable for CI
chli plate verify --all --fail-on-warn
```

Per canton:

1. `GET` the `halterauskunft.url` (and `form_pdf` if set). 10 s timeout, follows
   redirects, sends `User-Agent: chli-plate-verify/<version> (+<repo>)`.
2. Records final URL, status code, `Last-Modified`, content length.
3. Body heuristics: canton name appears; at least one of
   `["halterauskunft", "halter", "détenteur", "detentore"]` appears; not a
   placeholder page.
4. Emits one record per canton with `status: ok | warn | error` + reasons.
5. Never submits the form, never solves captcha, only observes the landing
   page.

### Exit codes

- `0` — all ok.
- `1` — any error (HTTP ≥ 500, timeout, hostname changed).
- `2` — any warn, only if `--fail-on-warn`. Default: warns don't fail.

### Workflow

`.github/workflows/plate-verify.yml`:

```yaml
name: plate-verify
on:
  release:
    types: [published]
  schedule:
    - cron: "0 6 * * 1"
  workflow_dispatch:

jobs:
  verify:
    runs-on: ubuntu-latest
    timeout-minutes: 5
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - run: go build -o chli .
      - run: ./chli plate verify --all -o json | tee plate-verify.json
      - uses: actions/upload-artifact@v4
        with:
          name: plate-verify-${{ github.event.release.tag_name || github.run_id }}
          path: plate-verify.json
      - if: failure()
        uses: actions/github-script@v7
        with:
          script: |
            // Read plate-verify.json, open one issue per errored canton,
            // label: plate-verify,data-rot,auto. Dedup against open issues.
```

On `release: published` the workflow runs against the freshly-built binary,
surfacing stale embedded data at release time. The weekly cron catches rot
between releases. `workflow_dispatch` allows ad-hoc runs.

## Testing

- `api/plate_test.go`:
  - Plate parsing: table test covering every accepted and rejected form.
  - Canton loader: embed-driven test asserts all 26 entries, invariant checks.
  - Deeplink template rendering for each canton with `deeplink_template` set.
  - Language fallback: requested `fr` falls back to `de` when `notes.fr`
    absent.
  - Verifier: unit test with `httptest.Server` covering ok / redirect /
    keyword-miss / timeout / 500.
- `api/vehicles_test.go`:
  - CSV parsing with golden fixtures in `api/testdata/vehicles-*.csv`.
  - Filter + group logic on synthetic rows.
  - Integration-style test gated by env var, hits opendata.swiss.
- `cmd/` smoke tests:
  - Cobra command wiring: flag parsing, error paths, output format selection.

## Build & CI

- `make build` compiles normally; `go:embed` pulls `plate_cantons.json` into
  the binary.
- `make test` covers the above.
- New workflow `plate-verify.yml`. Existing CI is unchanged.

## Open Items Deferred to v2

- Per-model registration breakdown if BFS ever exposes it.
- CO₂/emissions dataset wiring (separate BFS/ASTRA source).
- LINDAS-based alternative to opendata CKAN.
- Interactive `--confirm` flow that summarises costs before `--open`.
