# state — app-side SQLite (action log + AI transcript)

The application's own persistence, separate from the MariaDB connection. Holds
the undo action log and the AI tool-call transcript.

## Owns

| File | Role |
|------|------|
| `store.go` | `Store` (Open/Close/Record/SetStatus/Find/List), `Action`, `Verdict`, `Status`, schema migration |
| `turns.go` | `Turn`, `AddTurn`, `ListTurns` — AI transcript persistence |

## Invariants (do not break)

- **Schema migration lives in `store.go` `Open`.** New tables are added there with
  `IF NOT EXISTS`; parameterized SQL only.
- **The store is app-wide** — opened once in `tui/model.go` `ensureStore`, shared
  across connections. One writer.
- **`Action` records carry before/after images as JSON.** The undo executor
  (`tui/writer.go`) and AI registry (`ai/tools.go`) both record here.
- **`Verdict` distinguishes `Undoable` vs `LoggedOnly`** — the UI tags every action
  with it before and after execution (SPEC §6). Never let a write bypass recording.

## Change here

- New action kind → extend `Action.Kind` strings + the undo executors.
- New persisted state (e.g. sessions) → new table in `Open` + methods here.

## Tests

`go test ./internal/state/` — round-trip Record→List→SetStatus→Find against a
temp SQLite file; turns persistence.

## Doc check

Did this change touch an invariant above? If yes, update it here.
