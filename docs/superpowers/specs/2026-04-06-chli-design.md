# chli Design Spec

**Date**: 2026-04-06
**Status**: Draft

## Overview

`chli` (Swiss + CLI) is a Go CLI providing unified access to 5 Swiss federal open data sources in a single binary. It follows the klapp architecture blueprint: Cobra CLI, four packages (`cmd/`, `api/`, `config/`, `output/`), minimal dependencies, dual output (table for TTY, JSON for pipes).

Module path: `github.com/matthiasak/chli`

## Deviations from klapp

1. **No auth** — all APIs are public. No `login.go`, `logout.go`, `auth.go`, no token machinery.
2. **Multiple base URLs** — each `api/<source>.go` owns its own base URL constant.
3. **Filesystem cache** at `~/.cache/chli/` with per-source TTLs.
4. **Language flag** `--lang` (de/fr/it/en/rm), default `de`.

## Project Structure

```
chli/
├── main.go
├── cmd/
│   ├── root.go           # Root command + global flags
│   ├── fedlex.go         # Federal law (SPARQL)
│   ├── parl.go           # Parliament (OData)
│   ├── shab.go           # Official Gazette (REST)
│   ├── opendata.go       # opendata.swiss (CKAN)
│   └── entscheid.go      # Court decisions (Elasticsearch)
├── api/
│   ├── client.go         # Shared HTTP client, DoJSON, DoRaw
│   ├── cache.go          # Filesystem cache
│   ├── parl.go           # OData query builder + API calls
│   ├── parl_types.go
│   ├── fedlex.go         # SPARQL HTTP POST helper
│   ├── fedlex_queries.go # Canned SPARQL constants
│   ├── fedlex_types.go
│   ├── shab.go           # SHAB REST client
│   ├── shab_types.go
│   ├── opendata.go       # CKAN API calls
│   ├── opendata_types.go
│   ├── entscheid.go      # Elasticsearch POST helper
│   └── entscheid_types.go
├── config/
│   └── config.go         # User prefs only (lang, format, cache dir)
├── output/
│   └── output.go         # Table, JSON, Section, Error, PickLang
├── go.mod
├── Makefile
└── NOTES.md
```

## Dependencies

- `github.com/spf13/cobra` — CLI framework
- stdlib only for everything else (net/http, encoding/json, encoding/xml, text/tabwriter, crypto/sha256)

## Global Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--json` | bool | false | Force JSON output |
| `--no-color` | bool | false | Disable colored output |
| `--lang` | string | `de` | Language: de, fr, it, en, rm |
| `--no-cache` | bool | false | Skip cache reads |
| `--refresh` | bool | false | Force cache refresh |

## Package: config/

Stores user preferences at `~/.config/chli/config.json`. No credentials, no 0600 permissions needed.

```go
type Config struct {
    Language     string `json:"language"`      // de, fr, it, en, rm
    OutputFormat string `json:"output_format"` // table, json
    CacheDir     string `json:"cache_dir"`     // default ~/.cache/chli
}
```

## Package: output/

Follows klapp pattern exactly:

- `IsInteractive()` — checks `ForceJSON` flag, then TTY detection
- `Table(headers, rows)` — tabwriter, 3-space padding
- `JSON(v)` — indented JSON to stdout
- `Error(msg)` — stderr if interactive, JSON `{"error":...}` if piped
- `Section(title)` — `\n--- title ---\n`
- `PickLang(de, fr, it, en string, lang string) string` — returns the value matching the current language flag, falls back to `de`

## Package: api/

### client.go

Single `Client` struct, no auth:

```go
type Client struct {
    HTTP     *http.Client
    Config   *config.Config
    CacheDir string
    NoCache  bool
    Refresh  bool
}
```

Methods:
- `NewClient() (*Client, error)` — loads config, sets up HTTP client with `User-Agent: chli/<version>`
- `DoJSON(baseURL, path string, result any) error` — GET + JSON decode
- `DoRaw(baseURL, path string) ([]byte, error)` — GET raw bytes
- `DoPost(baseURL, path string, contentType string, body io.Reader, result any) error` — POST + decode (for SPARQL, Elasticsearch)
- `DoPostRaw(baseURL, path string, contentType string, body io.Reader) ([]byte, error)` — POST raw

All methods check cache before making requests (unless `NoCache`), store responses in cache after.

### cache.go

Filesystem cache at `~/.cache/chli/`:

- Key: SHA256 of `method + url + body` → hex filename
- Value: JSON file `{"data": <base64>, "timestamp": <unix>, "ttl": <seconds>}`
- TTLs per source:
  - Parliament: 24h (data changes weekly)
  - Fedlex: 168h / 7 days (law changes slowly)
  - SHAB: 1h (daily publications)
  - opendata.swiss: 24h
  - entscheidsuche: 24h
- `Get(key) ([]byte, bool)` — returns data if cached and not expired
- `Set(key string, data []byte, ttl time.Duration)` — writes cache file
- `Clear()` — removes all cache files

## API: Parliament OData (api/parl.go)

**Base URL**: `https://ws.parlament.ch/odata.svc/`

### Probed Response Shape

OData v3 format. JSON wrapped in `d` array:
```json
{"d": [{"__metadata": {...}, "ID": 1, "Language": "DE", "LastName": "Aguet", ...}]}
```

Composite key: `(ID, Language)`. Language is part of the record — to get German data, filter `Language eq 'DE'`.

### OData Query Builder

```go
type ODataQuery struct {
    table   string
    filter  []string
    selects []string
    top     int
    skip    int
    orderBy string
    expand  []string
}
```

Chainable methods: `Filter()`, `Select()`, `Top()`, `Skip()`, `OrderBy()`, `Expand()` → `Build() string` produces URL path with query params. Always appends `$format=json`.

### Entity Types (48 total, key ones)

| Table | Key Fields | Notes |
|-------|-----------|-------|
| Person | ID, Language, LastName, FirstName, PersonNumber, GenderAsString, DateOfBirth, PlaceOfBirthCanton | |
| Business | ID, Language, BusinessShortNumber, BusinessType, BusinessTypeName, Title, SubmittedBy, BusinessStatus, BusinessStatusText | |
| Vote | ID, Language | Links to Business via navigation |
| Voting | ID, Language | Individual council member votes |
| Session | ID, Language | |
| Committee | ID, Language | |
| Party | ID, Language | |
| MemberCouncil | ID, Language | |
| Canton | ID, Language | |
| Transcript | ID, Language | |
| Resolution | ID, Language | |
| PersonInterest | ID, Language | |

### Types

Flat structs with language as a field (not multilingual struct — Parliament returns one language per record):

```go
type ParlPerson struct {
    ID              int    `json:"ID"`
    Language        string `json:"Language"`
    LastName        string `json:"LastName"`
    FirstName       string `json:"FirstName"`
    GenderAsString  string `json:"GenderAsString"`
    DateOfBirth     string `json:"DateOfBirth"`
    PlaceOfBirthCity   string `json:"PlaceOfBirthCity"`
    PlaceOfBirthCanton string `json:"PlaceOfBirthCanton"`
    // ... more fields
}
```

### Commands

| Command | Description |
|---------|-------------|
| `chli parl tables` | List all OData entity types |
| `chli parl schema <table>` | Show columns (from $metadata XML) |
| `chli parl query <table> [flags]` | Generic query with --filter, --select, --top, --skip, --orderby |
| `chli parl person [--name ...] [--party ...]` | Convenience wrapper |
| `chli parl business [--title ...] [--type ...]` | Parliamentary businesses |
| `chli parl vote <business-id>` | Voting results |
| `chli parl session [--current]` | Sessions |
| `chli parl committee` | Committees |

## API: Fedlex SPARQL (api/fedlex.go)

**Base URL**: `https://fedlex.data.admin.ch/sparqlendpoint`

### Probed Response Shape

Standard SPARQL JSON results:
```json
{"head": {"vars": [...]}, "results": {"bindings": [{"var": {"type": "...", "value": "..."}}]}}
```

POST with `Content-Type: application/x-www-form-urlencoded`, body `query=<sparql>`, header `Accept: application/sparql-results+json`.

### Key Ontology Findings

- SR 101 (Constitution) = URI `https://fedlex.data.admin.ch/eli/cc/1999/404`
- Type: `jolux:ConsolidationAbstract`
- Properties: `dcterms:identifier`, `jolux:dateDocument`, `jolux:inForceStatus`, `jolux:isRealizedBy`, `jolux:classifiedByTaxonomyEntry`
- Expressions (language versions) linked via `jolux:isRealizedBy` → filter by `jolux:language`
- Language URIs: `http://publications.europa.eu/resource/authority/language/DEU` (FRA, ITA, ENG, ROH)
- SR number → ELI URI mapping: need a taxonomy lookup or pattern-based mapping

### Canned Queries (fedlex_queries.go)

String constants with `%s` placeholders:
- `QuerySRByNumber` — fetch SR entry by number
- `QuerySearchTitle` — search by title keyword
- `QueryBBLByYear` — Federal Gazette by year
- `QueryTreaties` — treaties with optional partner/year filter
- `QueryConsultations` — open/closed consultations

### Types

```go
type FedlexResult struct {
    Head    SPARQLHead    `json:"head"`
    Results SPARQLResults `json:"results"`
}

type SPARQLHead struct {
    Vars []string `json:"vars"`
}

type SPARQLResults struct {
    Bindings []map[string]SPARQLValue `json:"bindings"`
}

type SPARQLValue struct {
    Type     string `json:"type"`
    Value    string `json:"value"`
    Datatype string `json:"datatype,omitempty"`
}
```

For convenience commands, parse bindings into typed structs:

```go
type SREntry struct {
    URI        string
    Identifier string
    Title      string
    DateDoc    string
    InForce    string
}
```

### Commands

| Command | Description |
|---------|-------------|
| `chli fedlex sr <number>` | Fetch SR entry by number |
| `chli fedlex search <query> [--type ...]` | Search titles |
| `chli fedlex bbl [--year ...] [--week ...]` | Federal Gazette |
| `chli fedlex treaty [--partner ...] [--year ...]` | Treaties |
| `chli fedlex consultation [--status ...]` | Vernehmlassungen |
| `chli fedlex sparql <query-or-@file>` | Raw SPARQL escape hatch |
| `chli fedlex fetch <eli-uri> [--format ...]` | Download manifestation |

## API: SHAB (api/shab.go)

**Base URL**: `https://shab.ch/api/v1/`

**Important**: Requires `x-requested-with: XMLHttpRequest` header on all requests.

### Probed Response Shape

Search returns paginated JSON:
```json
{
  "content": [{"meta": {"id": "uuid", "rubric": "HR", "subRubric": "HR02", "language": "de",
    "publicationNumber": "...", "publicationState": "PUBLISHED", "publicationDate": "...",
    "title": {"de": "...", "fr": "...", "it": "...", "en": "..."}, "cantons": ["BS"],
    "registrationOffice": {...}}, "links": [], "attachments": [], "content": null}],
  "total": 552118,
  "pageRequest": {"page": 0, "size": 1}
}
```

Publication detail XML at `/publications/<uuid>/xml` — rich structured data with company info, purpose, capital, persons.

### Search Parameters

| Param | Description |
|-------|-------------|
| `keyword` | Search term |
| `rubrics` | Comma-separated: AB,AW,AZ,BB,BH,EK,ES,FM,HR,KK,LS,NA,SB,SR,UP,UV |
| `publicationStates` | PUBLISHED, CANCELLED |
| `pageRequest.page` | Page number (0-based) |
| `pageRequest.size` | Page size |
| `includeContent` | Include full content in search results |
| `allowRubricSelection` | Allow rubric facets |

### Rubric Codes

| Code | Description |
|------|-------------|
| HR | Handelsregister (Commercial Register) |
| SB | Schuldbetreibung und Konkurs (Debt enforcement and bankruptcy) |
| KK | Konkurse (Bankruptcies) |
| AB | Amtliche Bekanntmachungen |
| AW | Amtliche Warnungen |
| BB | Bundesblatt |
| EK | Eidg. Kommissionen |
| ES | Eidg. Steuerverwaltung |
| FM | Finanzmarktaufsicht |
| LS | Liegenschaftsschätzungen |
| NA | Nachlassverfahren |
| SR | Sozialversicherungsrecht |
| UP | Umweltschutz |
| UV | Urheberrecht und Verwertungsgesellschaften |
| AZ | Other |
| BH | Bundesamt für Gesundheit |

### Types

```go
type SHABSearchResult struct {
    Content     []SHABPublication `json:"content"`
    Total       int               `json:"total"`
    PageRequest SHABPageRequest   `json:"pageRequest"`
}

type SHABPublication struct {
    Meta        SHABMeta       `json:"meta"`
    Links       []any          `json:"links"`
    Attachments []any          `json:"attachments"`
    Content     any            `json:"content"`
}

type SHABMeta struct {
    ID                 string            `json:"id"`
    Rubric             string            `json:"rubric"`
    SubRubric          string            `json:"subRubric"`
    Language           string            `json:"language"`
    PublicationNumber  string            `json:"publicationNumber"`
    PublicationState   string            `json:"publicationState"`
    PublicationDate    string            `json:"publicationDate"`
    ExpirationDate     string            `json:"expirationDate"`
    Title              MultilingualText  `json:"title"`
    Cantons            []string          `json:"cantons"`
    RegistrationOffice *SHABOffice       `json:"registrationOffice"`
}

type MultilingualText struct {
    DE string `json:"de"`
    FR string `json:"fr"`
    IT string `json:"it"`
    EN string `json:"en"`
}
```

### Commands

| Command | Description |
|---------|-------------|
| `chli shab search <query> [--rubric ...] [--from ...] [--to ...]` | Search publications |
| `chli shab publication <id>` | Fetch publication (XML detail) |
| `chli shab rubrics` | List rubric codes |

## API: opendata.swiss CKAN (api/opendata.go)

**Base URL**: `https://ckan.opendata.swiss/api/3/action/`

**Important**: Requires `User-Agent` header (returns 403 without it).

### Probed Response Shape

Standard CKAN:
```json
{"success": true, "result": {"count": 73, "results": [{...dataset...}]}}
```

Datasets have multilingual fields as objects: `{"de": "...", "fr": "...", "it": "...", "en": ""}`. Resources have `download_url`, `format`, `media_type`.

### Types

```go
type CKANResponse struct {
    Success bool        `json:"success"`
    Result  CKANResult  `json:"result"`
}

type CKANResult struct {
    Count   int            `json:"count"`
    Results []CKANDataset  `json:"results"`
}

type CKANDataset struct {
    ID           string           `json:"id"`
    Name         string           `json:"name"`
    Title        MultilingualText `json:"title"`
    Description  MultilingualText `json:"description"`
    Organization CKANOrg          `json:"organization"`
    Resources    []CKANResource   `json:"resources"`
    NumResources int              `json:"num_resources"`
    Issued       string           `json:"issued"`
    Modified     string           `json:"metadata_modified"`
}

type CKANResource struct {
    ID          string           `json:"id"`
    Name        MultilingualText `json:"name"`
    Format      string           `json:"format"`
    DownloadURL string           `json:"download_url"`
    MediaType   string           `json:"media_type"`
}

type CKANOrg struct {
    Name  string           `json:"name"`
    Title MultilingualText `json:"title"`
}
```

### Commands

| Command | Description |
|---------|-------------|
| `chli opendata search <query> [--org ...] [--format ...]` | Search datasets |
| `chli opendata dataset <id>` | Full metadata + resources |
| `chli opendata orgs` | List organizations |

## API: entscheidsuche.ch (api/entscheid.go)

**Base URL**: `https://entscheidsuche.ch/`

**Endpoint**: `POST /_search.php` with Elasticsearch Query DSL JSON body.

### Probed Response Shape

Standard Elasticsearch:
```json
{
  "hits": {
    "total": {"value": 10000, "relation": "gte"},
    "hits": [{"_index": "entscheidsuche.v2-ag_baugesetzgebung", "_id": "...", "_score": 1.0,
      "_source": {"date": "1994-02-21", "hierarchy": ["AG","AG_BG","AG_BG_001"],
        "abstract": {"de": "...", "fr": "...", "it": "..."},
        "title": {"de": "...", "fr": "...", "it": "..."},
        "reference": ["..."], "canton": "AG", "id": "...",
        "attachment": {"content_url": "https://entscheidsuche.ch/docs/....pdf", "content_type": "application/pdf"}
      }}]
  }
}
```

### Field Names

Document `_source` fields (German PascalCase in some indices, lowercase in others):

| Field | Description |
|-------|-------------|
| `id` | Document ID |
| `date` | Decision date |
| `canton` | Canton abbreviation |
| `hierarchy` | Court hierarchy path |
| `title` | Multilingual title object |
| `abstract` | Multilingual summary object |
| `reference` | Case references array |
| `attachment.content_url` | PDF download URL |
| `attachment.content_type` | MIME type |
| `scrapedate` | When the decision was scraped |
| `meta` | Multilingual court metadata |

### Court Index Pattern

Index names follow `entscheidsuche.v2-<canton>_<court>` or `entscheidsuche-<canton>_<court>`. The hierarchy field `["AG", "AG_BG", "AG_BG_001"]` encodes canton → court → sub-court.

### Types

```go
type ESResponse struct {
    Hits ESHits `json:"hits"`
}

type ESHits struct {
    Total ESTotal  `json:"total"`
    Hits  []ESHit  `json:"hits"`
}

type ESTotal struct {
    Value    int    `json:"value"`
    Relation string `json:"relation"`
}

type ESHit struct {
    Index  string     `json:"_index"`
    ID     string     `json:"_id"`
    Score  float64    `json:"_score"`
    Source ESDecision `json:"_source"`
}

type ESDecision struct {
    ID         string           `json:"id"`
    Date       string           `json:"date"`
    Canton     string           `json:"canton"`
    Hierarchy  []string         `json:"hierarchy"`
    Title      MultilingualText `json:"title"`
    Abstract   MultilingualText `json:"abstract"`
    Reference  []string         `json:"reference"`
    Attachment ESAttachment     `json:"attachment"`
    Meta       MultilingualText `json:"meta"`
    ScrapeDate string           `json:"scrapedate"`
}

type ESAttachment struct {
    ContentURL  string `json:"content_url"`
    ContentType string `json:"content_type"`
}
```

### Commands

| Command | Description |
|---------|-------------|
| `chli entscheid search <query> [--court ...] [--from ...] [--to ...]` | Search decisions |
| `chli entscheid get <id>` | Get decision metadata + PDF URL |
| `chli entscheid courts` | List known courts (derived from index names or hierarchy) |

## Multilingual Strategy

Two patterns based on how APIs return data:

1. **Parliament OData**: Language is part of the composite key. Filter with `Language eq 'DE'`. Structs have single-language string fields.
2. **All other APIs**: Return multilingual objects `{de, fr, it, en}`. Use shared `MultilingualText` struct with `PickLang(lang)` method.

```go
type MultilingualText struct {
    DE string `json:"de"`
    FR string `json:"fr"`
    IT string `json:"it"`
    EN string `json:"en"`
    RM string `json:"rm,omitempty"`
}

func (t MultilingualText) Pick(lang string) string {
    switch lang {
    case "fr": return t.FR
    case "it": return t.IT
    case "en": return t.EN
    case "rm": return t.RM
    default:   return t.DE
    }
}
```

## Output Rules

- Table output: 4-6 columns max, truncate long titles to ~60 chars
- JSON output: full struct, untruncated
- Escape-hatch commands (`parl query`, `fedlex sparql`) always output JSON
- Every command branches on `output.IsInteractive()`

## Cache TTLs

| Source | TTL | Rationale |
|--------|-----|-----------|
| Fedlex | 7 days | Law changes slowly |
| Parliament | 24h | Data updates weekly |
| SHAB | 1h | Daily publications |
| opendata.swiss | 24h | Moderate update frequency |
| entscheidsuche | 24h | Moderate update frequency |

## Implementation Order

1. `config/`, `output/`, `api/client.go`, `api/cache.go`, `cmd/root.go` with global flags
2. `chli fedlex` — proves SPARQL pattern. Start with `sr` and `search`
3. `chli parl` — proves OData pattern. Start with `tables`, `schema`, then `business`, `person`
4. `chli shab` — REST with XHR header
5. `chli opendata` — CKAN, well-documented
6. `chli entscheid` — Elasticsearch POST
7. (stretch) `chli openparl` — OpenParlData.ch
