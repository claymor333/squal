# db/mutate — RowEditor + undo contract + keyset pagination

The write half of the MariaDB layer. All mutations pass through the undo
contract defined in SPEC §6.

## Owns

| File | Role |
|------|------|
| `undo.go` | `Classify(sql)` → `UndoVerdict` — decides undo feasibility BEFORE execution |
| `editor.go` | `RowEditor` — load/update/delete rows with before/after images |
| `keyset.go` | `LoadNextSQL`, `keysetCondition` — keyset pagination (never `OFFSET`) |

## Invariants (do not break)

- **Never undo DDL or `TRUNCATE`.** `Classify` marks them infeasible; nothing here
  may widen that.
- **Undoable = single-table INSERT/UPDATE/DELETE, no joins/aliases/subqueries/LIMIT,
  no `INSERT...SELECT`.** The predicate must translate to a before-image SELECT.
- **Quote identifiers** with `db.QuoteIdentifier`. No string interpolation of
  identifiers anywhere in this package.
- **Before-image inside a transaction.** The executor (tui/writer.go) captures
  `SELECT ... FOR UPDATE` on the same tx as the write; this package provides the
  `BeforeSelect` the executor runs.
- **Undo restores row data only** — triggers, auto-increment counters, and
  statement-set `updated_at` never come back.

## Change here

- New verdict type → extend `UndoVerdict` + the `Classify` switch.
- New mutation → add a method on `RowEditor` (it already handles before/after).
- New pagination key shape → `keysetCondition`'s composite-key builder.

## Tests

`go test ./internal/db/mutate/` — unit tests for `Classify` and `keysetCondition`
(no DB); `TestRowEditorUpdateUndo` needs the live `squal_smoke` DB and skips when
unreachable.

## Doc check

Did this change touch an invariant above? If yes, update it here.
