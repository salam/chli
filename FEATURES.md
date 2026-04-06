# Features

## Data Sources

### Swiss Parliament (`chli parl`)
- Search members by name, party, or ID
- Full activity profiles: committees, interests, lobbying badges (via OpenParlData)
- Parliamentary business lookup by short number
- Business detail views with participants, status timeline, preconsultations, publications, and transcripts
- Browse all 48 OData entity sets with `parl tables`
- Inspect entity schemas with `parl schema <table>`
- Generic OData query builder with filter, select, top, skip, and orderby support

### Federal Law (`chli fedlex`)
- SR number lookup (e.g., `101` for the Federal Constitution)
- Full-text title search across consolidated legislation
- Federal Gazette (BBL) entries by year
- International treaties with partner and year filters
- Federal consultations (Vernehmlassungen) with status filters
- Raw SPARQL queries for advanced users (inline or from file with `@`)
- Download URL construction for law texts in HTML, PDF, or XML

### Official Gazette (`chli shab`)
- Keyword search across all publications
- Rubric-based filtering (HR, KK, SB, AB, and 12 more)
- Pagination support
- Full XML detail for individual publications
- Rubric directory listing

### Open Data (`chli opendata`)
- Dataset search across opendata.swiss
- Organization and format filters
- Full metadata and resource listings for individual datasets
- Organization directory

### Court Decisions (`chli entscheid`)
- Full-text search across all 26 cantons and federal courts
- Court and date range filters
- Individual decision metadata with PDF download URLs
- Court directory listing

## CLI Features

### Smart Output
- Automatic TTY detection: tables in terminal, JSON when piped
- `--json` flag to force JSON output from terminal
- Truncated columns for readable tables, full data in JSON mode
- Errors on stderr (interactive) or as JSON (piped)

### Intelligent Caching
- Filesystem-based with SHA256 key hashing
- Per-source TTLs tuned to data update frequency
- `--no-cache` to skip cache reads entirely
- `--refresh` to force fresh data while still writing to cache
- Configurable cache directory

### Multilingual Support
- Five languages: de, fr, it, en, rm
- Automatic language-aware OData filtering for Parliament
- Multilingual text picker for Fedlex, SHAB, and opendata.swiss
- Per-command language override with `--lang`

### Escape Hatches
- `parl query` — raw OData queries against any of the 48 Parliament entity sets
- `fedlex sparql` — raw SPARQL queries against the Fedlex endpoint
- File input for SPARQL with `@filename` syntax

### Cross-Platform
- Single static binary, no runtime dependencies
- Pre-built for macOS (Intel + Apple Silicon), Linux (amd64 + arm64), Windows
- Minimal external dependencies (only Cobra for CLI)

### Configuration
- JSON config at `~/.config/chli/config.json`
- Persistent language and cache directory preferences
- Sensible defaults that work without any configuration
