# db — MariaDB transport + streaming fetch + schema introspection

The read half of the MariaDB layer. Transport, columnar streaming, and schema
metadata. Writes live in the sibling `db/mutate` package.

## Owns

| File | Role |
|------|------|
| `conn.go` | `Conn` (pooled `*sql.DB`), DSN builder, `BeginTx` |
| `fetch.go` | `Columnar` (column-major), `Fetch` (batched streaming), `FetchOn` (dedicated conn + USE), `fetchRows` (shared pipeline) |
| `schema.go` | `Schema`, `Columns`, `PrimaryKey`, `QuoteIdentifier`, `Database`/`Table`/`Column` |

## Invariants (do not break)

- **`Columnar` is column-major** (`Cols[c][r]`, one slice per column). Every
  consumer (tui/results.go, ai/tools.go `Summarize`) depends on this layout. A
  plan review already caught row-major test data — do not repeat it.
- **Sort/filter never hit the server.** The grid re-sorts the columnar buffer
  client-side; `Fetch` streams once.
- **Bulk reads: plain `Rows` loop into `RawBytes`→`string`.** No sqlx convenience
  layer. `RawBytes` is invalid after `Next()` — copy immediately (fetch.go does).
- **Quote identifiers** with `QuoteIdentifier`; positional `?` args only.
- **`Fetch` appends into the caller's `Columnar`.** The caller may read values only
  after a `Batch` arrives on the channel (send/receive orders the writes).
- **`FetchOn` uses a dedicated `sql.Conn` + `USE`** so unqualified SQL resolves
  against a chosen database. Do NOT replace it with `USE` on the pool — a pooled
  connection doesn't keep the session between calls. `fetchRows` is the shared
  streaming body; both `Fetch` and `FetchOn` must flow through it.

## Change here

- New driver flag → DSN builder in `conn.go`.
- New schema query → add a method in `schema.go`.
- **Columnar layout change → update every consumer first** (tui/results.go,
  ai/tools.go `Summarize`, tui/model.go `wireAI` OnQuery).

## Tests

`go test ./internal/db/` — `Columnar`/`Fetch` are exercised via `cmd/smoke` and the
mutate package's live-DB test. Schema methods are verified through the app.

## Doc check

Did this change touch an invariant above? If yes, update it here.
