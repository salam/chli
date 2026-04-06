# Release Notes

## v0.1.0 — Initial Release

First public release of chli, a unified CLI for Swiss federal open data.

### What's Included

**Five data sources in one binary:**
- **Swiss Parliament** — Members, votes, business items, committees via OData v3
- **Federal Law (Fedlex)** — SR lookups, full-text search, Federal Gazette, treaties, consultations via SPARQL
- **Official Gazette (SHAB)** — Publication search with rubric filtering via REST
- **opendata.swiss** — Dataset search across Switzerland's open data portal via CKAN
- **Court Decisions (entscheidsuche.ch)** — Full-text search across all 26 cantons via Elasticsearch

**Core features:**
- Dual output mode: human-readable tables in terminal, JSON when piped
- Filesystem caching with source-specific TTLs
- Multilingual support (de, fr, it, en, rm)
- Escape hatches for raw OData and SPARQL queries
- Cross-platform builds (macOS, Linux, Windows)
- Single dependency (Cobra)

### Known Limitations

- Parliament API requires curl fallback due to TLS fingerprint detection
- SHAB publication detail returns raw XML (no structured parsing yet)
- No shell completions bundled yet
- Version defaults to "dev" when built without git tags
