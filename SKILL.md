---
name: chli
description: Swiss federal open data CLI — parliament, law, courts, gazette, datasets from one binary
metadata: {"openclaw": {"requires": {"bins": ["go"]}, "homepage": "https://github.com/matthiasak/chli"}}
---

# chli

Swiss federal open data CLI. One binary, five government APIs, consistent interface.

## What it does

Wraps five Swiss government data sources into a single CLI:

| Command           | Source              | Protocol      | Data                                   |
|-------------------|---------------------|---------------|----------------------------------------|
| `chli parl`       | parlament.ch        | OData v3      | Members, votes, business, committees   |
| `chli fedlex`     | fedlex.data.admin.ch| SPARQL        | Federal law (SR), gazette, treaties    |
| `chli entscheid`  | entscheidsuche.ch   | Elasticsearch | Court decisions, all 26 cantons        |
| `chli shab`       | shab.ch             | REST          | Official Gazette publications          |
| `chli opendata`   | opendata.swiss      | CKAN          | Public datasets and organizations      |

## Stack

- **Language:** Go 1.25+
- **CLI framework:** Cobra (only external dependency)
- **Build:** `make build` produces `./chli`
- **Test:** `make test` (go test ./... -v)
- **Lint:** `make lint` (go vet)

## Project layout

```
main.go              Entry point (delegates to cmd/)
cmd/                 CLI commands (Cobra). One file per data source + root.go
  root.go            Base command, global flags, cache management, shell completion
  parl.go            Parliament subcommands
  fedlex.go          Federal law subcommands
  entscheid.go       Court decisions subcommands
  shab.go            Official gazette subcommands
  opendata.go        Open data subcommands
  bookmark.go        Bookmark/watch functionality
  feed.go            RSS/feed mode
api/                 HTTP clients, types, caching
  client.go          Base HTTP client (TLS fingerprint, retry, timeout)
  cache.go           Filesystem cache (~/.cache/chli/, SHA256 keys, per-source TTLs)
  parl.go            Parliament OData client
  fedlex.go          Fedlex SPARQL client
  fedlex_queries.go  Pre-built SPARQL query templates
  entscheid.go       Court decisions Elasticsearch client
  shab.go            SHAB REST client
  opendata.go        CKAN client
  openparl.go        OpenParlData supplementary client
  *_types.go         Data structures per source
  *_test.go          Unit tests
output/              Dual-mode formatting (table when TTY, JSON when piped)
  output.go          Table/JSON/CSV/TSV rendering, TTY detection, paging
config/              JSON config loader (~/.config/chli/config.json)
```

## Conventions

- **One dependency.** Cobra only. Everything else uses Go stdlib.
- **One file per source.** Each data source has a matched pair: `cmd/<source>.go` (CLI) and `api/<source>.go` (client) + `api/<source>_types.go` (structs).
- **Output goes through `output/`.** Never print directly. Use the output layer for TTY-aware formatting.
- **Cache is per-source.** Each API client sets its own TTL. Cache keys are SHA256-hashed.
- **Global flags** (`--json`, `--lang`, `--no-cache`, `--refresh`, `--verbose`, `--debug`, `--quiet`, `--no-color`, `--columns`, `-o format`) are defined in `cmd/root.go`.
- **Multilingual.** Five languages (de/fr/it/en/rm). Language flows through API calls where the source supports it.
- **Error handling.** Structured errors on stderr (interactive) or JSON (piped). Retry with exponential backoff + jitter on 429/5xx.
- **TLS fingerprinting.** `api/client.go` mimics Chrome's TLS config to bypass WAF on parlament.ch. Do not simplify this.

## Adding a new data source

1. Create `api/<source>.go` with a client struct and methods. Use `api/client.go`'s `DoRequest` for HTTP. Add `api/<source>_types.go` for response structs.
2. Create `cmd/<source>.go` with Cobra subcommands. Register the top-level command in `cmd/root.go`'s `init()`.
3. Add a cache TTL constant in `api/cache.go`.
4. Route all output through `output.PrintTable` / `output.PrintJSON`.
5. Add tests in `api/<source>_test.go`.

## Adding a new subcommand to an existing source

1. Add the Cobra command in `cmd/<source>.go`.
2. Add any needed API methods in `api/<source>.go` and types in `api/<source>_types.go`.
3. Respect `--lang`, `--json`, `--no-cache`, `--columns` flags.

## Key design decisions

- **Escape hatches exist.** `parl query` (raw OData) and `fedlex sparql` (raw SPARQL, supports `@filename`) let power users bypass the structured CLI.
- **TTY detection drives output.** Piped output is always JSON by default. Terminal output is always tables. `--json` forces JSON in terminal.
- **Cache is aggressive.** Federal law caches 7 days, parliament 1 hour, others 1-24 hours. `--no-cache` skips reads, `--refresh` forces fresh fetch.
- **Build metadata.** Version, commit, and build date are injected via ldflags at build time.

## Distribution

- Homebrew formula in `Formula/chli.rb`
- Nix flake in `flake.nix`
- GoReleaser config in `.goreleaser.yml`
- GitHub Actions CI in `.github/workflows/`
- Cross-platform binaries: macOS (Intel + ARM), Linux (amd64 + arm64), Windows
