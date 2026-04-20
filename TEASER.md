# chli

**A unified command-line interface for Swiss federal open data.**

One Go binary, ten Swiss government data sources, one consistent CLI.

## What it does

Switzerland publishes a wealth of government data through various APIs — OData, SPARQL, Elasticsearch, CKAN, REST — each with its own query language, pagination, and auth quirks. `chli` wraps them all into a single CLI with caching, multilingual support, and adaptive output formatting (table in TTY, JSON when piped, plus YAML/CSV/TSV/Markdown on demand).

```bash
chli parl person --name "Sommaruga"       # Parliament
chli fedlex sr 101                        # Federal law (constitution)
chli entscheid search "Mietrecht"         # Court decisions (all 26 cantons)
chli shab search "Konkurs" --rubric KK    # Official gazette
chli swissreg trademark '"Ovomaltine"'    # IP register
chli zefix search "Migros" --canton ZH    # Commercial register
chli lindas datasets                      # Federal linked-data hub
chli geo search "Bundesplatz 3, Bern"     # Geoportal
chli opendata search "Verkehr"            # opendata.swiss
```

## Why it's interesting

- **Agent-native.** Ships with a [SKILL.md](SKILL.md) so Claude Code / OpenClaw / other agent frameworks can use it as a discoverable tool for Swiss-data research (journalism, legal, compliance, due-diligence).
- **Zero-dependency Go.** Only Cobra plus the standard library. Cross-compiles to macOS/Linux/Windows (amd64 + arm64) via `make all`.
- **Caching by default.** Per-source TTLs keep public APIs happy and repeat queries fast.
- **Multilingual.** `de`/`fr`/`it`/`en`/`rm` wherever the upstream API supports it.

## Traction signals

- Active shipping cadence on `main`: recent work added `zefix`, `uid`, `lindas`, `geo`, and `swissreg` sources plus a credentials keystore and YAML/Markdown output modes.
- Source structure maps 1:1 to an extensible API-client layer under `api/` — adding a new Swiss federal data source is a well-worn path (see `docs/` and `SKILL.md`).

## Status

- **Stage:** Working product, single maintainer, used for Swiss open-data research and agent-driven workflows.
- **Repo:** https://github.com/salam/chli
- **License:** MIT
