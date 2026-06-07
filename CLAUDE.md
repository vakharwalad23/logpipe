# Logpipe

Small Go monorepo that mimics a Vector → ClickHouse logging stack: a service
emits logs, Vector ships and batches them, ClickHouse stores them, and a query
API (or Grafana) reads them back.

## Data flow

crud-api -> Vector agent -> Vector aggregator (batches) -> ClickHouse -> query-api / Grafana

The Vector agent ships to the aggregator; the aggregator batches inserts into
ClickHouse. The agent never writes to ClickHouse directly — batching is what
keeps ClickHouse healthy.

## Layout

- `cmd/` — binaries: `crud-api` (log producer), `query-api` (log reader / API),
  `loadgen` (traffic + synthetic log generator).
- `internal/` — private shared packages: `crud` (CRUD service + in-memory
  store), `logging` (slog JSON log streams), `clickhouse` (ClickHouse client),
  `query` (query-syntax-to-SQL), `loadgen` (load generation).
- `deploy/clickhouse/` — ClickHouse init SQL.
- `deploy/vector/` — Vector agent and aggregator config.
- `docker-compose.yml`, `Makefile`, `README.md` — local stack, common tasks, run
  instructions.

## Coding standards

- Idiomatic Go, formatted with gofmt.
- Lowercase package names, no underscores; idiomatic naming throughout.
- Doc comments on packages and exported identifiers.
- Wrap errors with `%w`.
- Table-driven tests.
- No decorative command output — no emojis, banners, colored echo, or ASCII art.
  Keep commands, Makefile targets, and scripts plain and standard.

## Working rules

- Implement strictly following PLAN.md. Never auto-advance to the
  next without being asked.
- Ask before adding any third-party dependency.
- The Vector agent must ship to the aggregator (which batches) and never directly
  to ClickHouse.
- Always parameterize ClickHouse queries — never build SQL by string
  concatenation.

## Commit conventions

- Use Conventional Commits for every commit: `type(scope): summary` with
  the summary in imperative mood (`add`, not `added`). Common types:
  `feat`, `fix`, `refactor`, `perf`, `docs`, `test`, `chore`, `build`, `ci`.
- Keep the subject ≤50 chars where practical, no trailing period.
- Add a body only for non-obvious *why*, breaking changes, or migration
  notes; mark breaking changes with `!` and a `BREAKING CHANGE:` footer.

## Source of truth

PLAN.md is the build sequence and the single source of truth for structure.