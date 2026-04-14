# chli

A unified command-line interface for Swiss federal open data. Access parliament records, federal law, court decisions, the official gazette, public datasets, the IP register, the commercial register, federal linked-data, and the geoportal — all from a single binary.

```
chli parl person --name "Sommaruga"
chli fedlex sr 101
chli entscheid search "Mietrecht"
chli shab search "Konkurs" --rubric KK
chli opendata search "Verkehr"
chli swissreg trademark '"Ovomaltine"'
chli zefix search "Migros" --canton ZH
chli geo search "Bundesplatz 3, Bern"
```

## Why chli?

Switzerland publishes a wealth of government data through various APIs — OData, SPARQL, Elasticsearch, CKAN, and REST. Each has its own query language, pagination, and authentication quirks. **chli** wraps them all into a consistent CLI with caching, multilingual support, and smart output formatting.

## Install

### From source

```bash
git clone https://github.com/salam/meteoswisscli.git
cd meteoswisscli
make build
./chli --help
```

Requires Go 1.25+.

### Cross-platform builds

```bash
make all
# Produces binaries in dist/ for:
#   darwin-amd64, darwin-arm64
#   linux-amd64, linux-arm64
#   windows-amd64
```

## Data Sources

| Command | Source | API | Description |
|---------|--------|-----|-------------|
| `chli parl` | [parlament.ch](https://www.parlament.ch) | OData v3 (+ legacy JSON for departments) | Members, votes, business items, committees, federal departments |
| `chli fedlex` | [fedlex.data.admin.ch](https://fedlex.data.admin.ch) | SPARQL | Federal law (SR), Federal Gazette, treaties, consultations |
| `chli shab` | [shab.ch](https://www.shab.ch) | REST | Official Gazette publications |
| `chli opendata` | [opendata.swiss](https://opendata.swiss) | CKAN | Public datasets and organizations |
| `chli entscheid` | [entscheidsuche.ch](https://entscheidsuche.ch) | Elasticsearch | Court decisions across all 26 cantons |
| `chli swissreg` | [swissreg.ch](https://www.swissreg.ch) | REST (public search backend) | Trademarks, patents, designs, and their publications |
| `chli zefix` | [zefix.admin.ch](https://www.zefix.admin.ch) | REST (HTTP Basic) | Swiss commercial register — search and UID/CHID lookup |
| `chli uid` | [zefix.admin.ch](https://www.zefix.admin.ch) | REST (HTTP Basic) | UID-centric entry point; shares credentials with `zefix` |
| `chli lindas` | [lindas.admin.ch](https://lindas.admin.ch) | SPARQL | Federal linked-data hub (IPI, BFS, procurement, energy, …) |
| `chli geo` | [api3.geo.admin.ch](https://api3.geo.admin.ch) | REST | Address/place search, layer listing, identify-by-coordinate |

## Quick Start

### Parliament

```bash
# Search for a member of parliament
chli parl person --name "Müller"

# Look up a parliamentary business item
chli parl business 24.3012

# Browse available OData entity sets
chli parl tables

# Run a custom OData query
chli parl query Person --filter "LastName eq 'Berset'" --select "PersonNumber,FirstName,LastName"

# List federal departments (uses legacy ws-old.parlament.ch — not exposed via OData)
chli parl department
chli parl department --historic   # include end-dated historic records

# Show current session (or past + next) + command help
chli parl

# Upcoming parliamentary events from the parlament.ch agenda
chli parl events                  # all upcoming events
chli parl events --sessions       # only upcoming sessions (through 2027)
chli parl events --lang fr        # localized titles and categories
```

The `parl session` list and `chli parl` root both merge in future sessions
scheduled on parlament.ch that are not yet published in the OData Session
entity (the OData feed usually lags by a few months).

### Federal Law

```bash
# Look up a law by SR number
chli fedlex sr 210          # ZGB
chli fedlex sr 814.01       # Umweltschutzgesetz

# Search laws by title
chli fedlex search "Datenschutz"

# Federal Gazette entries
chli fedlex bbl --year 2025

# Run raw SPARQL
chli fedlex sparql "SELECT ?s ?p ?o WHERE { ?s ?p ?o } LIMIT 10"
chli fedlex sparql @query.rq   # from file
```

### Court Decisions

```bash
# Search across all courts
chli entscheid search "Mietrecht"

# Filter by court and date range
chli entscheid search "Haftung" --court BGer --from 2024-01-01 --to 2024-12-31

# Results include an ID column — pass it straight to get/download
chli entscheid get CH_BGer_001_4A-123-2024_2024-06-15

# List available courts
chli entscheid courts
```

### Official Gazette (SHAB)

```bash
# Search publications (table includes a clickable shab.ch link per row)
chli shab search "Konkurs"

# Filter by rubric
chli shab search "AG" --rubric HR,KK

# Full publication detail — company, seat, address, auditor, changes
# (accepts the publication number or the internal UUID)
chli shab publication HR02-1006615899

# Field-level diff between prior and new state (HR mutations)
chli shab publication HR02-1006615899 --diff

# Timeline of prior FOSC entries for the same legal entity
chli shab history HR02-1006615899
chli shab history HR02-1006615899 --depth 5

# List available rubric codes
chli shab rubrics
```

### Swiss IP Register (Swissreg)

```bash
# Trademarks, patents, and designs — no authentication required.
chli swissreg trademark '"Ovomaltine"'       # quote for exact phrase
chli swissreg trademark Nestle --status aktiv --class 30 --max 50
chli swissreg trademark Nestle --filed-after 2024-01-01 --office DE
chli swissreg patent Widerstand
chli swissreg design Stuhl
chli swissreg patent-pub "Ottavio Sala"

# Download a trademark image (looks up the hash automatically).
chli swissreg image 570105 --out ovo.png
chli swissreg image 570105 --url          # just print the image URL

# Fetch a single record by internal URN id (discoverable in search results).
chli swissreg detail chmarke 1206422825
```

Filters: `--status aktiv|geloescht`, `--class 30` (comma-separated for
multiple), `--filed-after YYYY-MM-DD`, `--filed-before YYYY-MM-DD`,
`--office CH|DE|AT|...`.

Pagination beyond `--max 64` is not supported: the backend uses an opaque
Transit+JSON cursor that chli does not decode. Refine the query instead.

### Commercial Register (Zefix) & UID

Zefix and the UID lookup share the same HTTP Basic credentials (register at
[zefix.admin.ch](https://www.zefix.admin.ch), see the API section).

```bash
# Store credentials once (prompts for user + password, password echo-suppressed)
chli zefix login                       # or: chli uid login — same keystore entry
chli zefix status
chli zefix logout

# Alternatively, export env vars (take precedence over the keystore)
export ZEFIX_USER=... ZEFIX_PASS=...

# Search the commercial register by company name
chli zefix search "Migros" --canton ZH --max 50

# Look up by UID or CHID
chli zefix company CHE-105.817.537
chli uid lookup CHE-105.817.537

# Normalize / pretty-format a UID (no API call)
chli uid format che105817537           # → CHE-105.817.537
```

### LINDAS (linked open data)

```bash
# List datasets exposed by LINDAS
chli lindas datasets

# Raw SPARQL — inline or from a file (same pattern as `fedlex sparql`)
chli lindas sparql "SELECT ?s ?p ?o WHERE { ?s ?p ?o } LIMIT 10"
chli lindas sparql @my-query.rq
```

LINDAS aggregates many admin linked-data graphs into one SPARQL endpoint:
IPI trademarks/patents, BFS statistics, public procurement, energy data,
and more.

### Geoportal (geo.admin.ch)

```bash
# Address / place search
chli geo search "Bundesplatz 3, Bern"

# Feature search across a specific layer type
chli geo search "Rütli" --type featuresearch --limit 5

# List available map layers (useful to find layer IDs for `identify`)
chli geo layers

# Identify features at a WGS84 coordinate (lon,lat)
chli geo identify 7.4446,46.9479
chli geo identify 7.4446,46.9479 --layers ch.kantone.cantonal-boundaries
```

### Open Data

```bash
# Search datasets
chli opendata search "Bevölkerung"

# Filter by organization and format
chli opendata search "Klima" --org meteoschweiz --format CSV

# List organizations
chli opendata orgs
```

## Output

chli adapts its output automatically, with explicit override via `-o`:

- **Terminal (TTY):** Human-readable tables with aligned columns
- **Piped:** JSON by default
- **`-o <format>`:** Force `json`, `yaml`, `csv`, `tsv`, or `md` (GitHub-flavoured markdown table). `--json` is a shortcut for `-o json`.

```bash
# Table in terminal
chli parl person --party "SP"

# JSON (same data, scriptable)
chli parl person --party "SP" --json
chli parl person --party "SP" | jq '.[] | .FirstName'

# YAML
chli zefix search "Migros" -o yaml

# Markdown table (paste into docs or GitHub issues)
chli geo layers -o md

# CSV export
chli entscheid search "Mietrecht" -o csv > cases.csv
```

## Global Flags

| Flag | Description |
|------|-------------|
| `--json` | Force JSON output (shortcut for `-o json`) |
| `-o <fmt>` | Output format: `json`, `yaml`, `csv`, `tsv`, `md` |
| `--no-color` | Disable colored output |
| `--lang <code>` | Language: `de` (default), `fr`, `it`, `en`, `rm` |
| `--no-cache` | Skip reading from cache |
| `--refresh` | Force cache refresh (ignore TTL) |

## Caching

chli caches API responses to reduce latency and load on public APIs. Cache files are stored at `~/.cache/chli/` with source-specific TTLs:

| Source | TTL | Reason |
|--------|-----|--------|
| Federal Law | 7 days | Law changes infrequently |
| Parliament | 1 hour | Data updates weekly |
| SHAB | 1 hour | Daily publications |
| opendata.swiss | 24 hours | Metadata changes slowly |
| Court Decisions | 24 hours | Decisions published periodically |
| Swissreg | 24 hours | IP register updates slowly |
| Zefix / UID | 24 hours | Commercial register updates slowly |
| LINDAS | 24 hours | Aggregated linked-data graphs |
| Geoportal | 7 days | Layer metadata and place lookups are stable |

Use `--no-cache` to bypass or `--refresh` to force a fresh fetch.

## Configuration

Optional config file at `~/.config/chli/config.json`:

```json
{
  "language": "de",
  "cache_dir": "~/.cache/chli"
}
```

## Languages

All five official and national languages are supported where the underlying API provides translations:

- **de** - Deutsch (default)
- **fr** - Francais
- **it** - Italiano
- **en** - English
- **rm** - Rumantsch

```bash
chli --lang fr fedlex search "protection des donnees"
chli --lang it parl person --name "Cassis"
```

## Architecture

```
cmd/          CLI commands (Cobra)
api/          API clients, types, and caching
config/       User configuration
output/       Dual-mode output formatting (table/JSON)
```

**Dependencies:** Only [Cobra](https://github.com/spf13/cobra). Everything else uses the Go standard library.

## For AI Agents

This repository includes a [SKILL.md](SKILL.md) with project layout, conventions, and recipes for extending chli. Compatible with OpenClaw and other agent frameworks that discover skills via frontmatter-annotated SKILL.md files.

## License

MIT
