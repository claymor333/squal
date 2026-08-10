# Squal — Project Brain & Conventions

This file is the project's operational memory. It is structured so a session can load
what it needs without reading everything, and so edits stay local (change one section,
don't rewrite the file).

**Read order for a new session:**
1. **How to think about this codebase** (below) — the mental model in 30 seconds.
2. **Architecture map** — package roles + the message flow.
3. The section that matches your task (tui / db / ai / state / config).
4. **Conventions** before writing any code.

**This project runs on superpowers.** Load `superpowers-using-superpowers` at session
start and follow it: brainstorm → write plan → subagent-driven implementation →
review → finish branch. See `.agents/skills/`.

---

## How to think about this codebase

Squal is a **keyboard-first TUI for browsing and editing MariaDB**, with an
OpenAI-compatible AI agent that can run queries and write data. Three hard rules
define its shape:

1. **The grid loads fast and stays local.** Results stream off a goroutine into
   columnar storage (`[][]string`, one slice per column) and are sorted/filtered
   client-side. The server is never re-queried for sort/filter. Large tables page via
   **keyset pagination, never `OFFSET`**.
2. **No undo lies.** Every write action is either *undoable* or *logged-only*, decided
   BEFORE execution by a parser, and the UI says which. Structured edits always capture
   a before-image; raw SQL is undoable only when `sqlparser` can prove it's a
   single-table statement with a translatable predicate.
3. **AI is a verb, not a chat box.** The AI runs real queries and writes through the
   same undo contract as a human. Writes confirm in the TUI before executing — enforced
   centrally in the registry, not by the UI, so no caller can bypass it.

**The product is a browser first.** Read paths are polished; the interactive write
path (row panel → editor → undo) is partially wired — see "Known gaps" below.

---

## Architecture map

```
main.go
  └─ internal/
     ├─ config/   profiles + AI endpoint. JSON config, 0600, env overrides.
     ├─ db/       the MariaDB read layer: conn, streaming fetch, schema.
     │  └─ mutate/  the write layer: undo classifier, RowEditor, keyset. (db/mutate)
     ├─ state/    app-side SQLite: action log (undo store) + AI transcript.
     ├─ ai/       OpenAI-compatible client + tool-calling agent. No TUI deps.
     └─ tui/      bubbletea app: tabs, panes, keymap, batch pump, AI panel.
cmd/smoke        integration check against a real MariaDB (the exception to
                 "no verification scripts" — tests cover unit behavior).
```

### Per-module docs — the brainstore discipline

Every package has a `DOC.md` sitting next to its code: what it owns, its
invariants, how to change it, its test command, and a **doc-check** line ("did
this change touch an invariant above?"). Update the module's `DOC.md` in the same
change as the code — a change that touches an invariant without updating its doc
is incomplete. The docs are the contract; the invariants in them are the rules a
review enforces.

### Data flow (the one diagram worth remembering)

```
query ─→ db.Fetch ──goroutine──► Columnar (column-major, [][]string)
                                    │
                     batchMsg ◄─────┘ (channel, ~500 rows each)
                                    │
                     tui.handleBatch ─► resultsView.rebuildOrder (sort+filter local)
                                    │
                                    └─ visibleRows → rendered grid
                                    └─ keyset LoadNextSQL → next 100k
```

### Message flow (bubbletea)

All panes communicate by `tea.Msg` types declared in `internal/tui/messages.go`:
`browseRequestMsg`, `runQueryMsg`, `batchMsg`, `queryDoneMsg`, `loadNextMsg`,
`saveRowMsg`, `deleteRowMsg`, `undoActionMsg`, plus AI's `aiEventMsg`/`aiConfirmMsg`.
`model.Update` dispatches by type. **Add new message types to `messages.go`, never
inline in a pane.** Panes are pure-ish: they mutate `connData` state, emit msgs, and
return `tea.Cmd`s; they don't talk to the DB directly.

### The undo contract (SPEC §6, enforced in `internal/db/undo.go` + `internal/tui/writer.go`)

- `db.Classify(sql)` → `UndoVerdict{Feasible, Kind, Table, ExecSQL, BeforeSelect, Reason}`.
  Feasible = single-table INSERT/UPDATE/DELETE, no joins/aliases/subqueries/LIMIT, no
  `INSERT...SELECT`. DDL and `TRUNCATE` are never undoable.
- `writer.runTypedSQL` runs feasible SQL in a **transaction**: capture before-image
  with `SELECT ... FOR UPDATE`, execute, commit, record the action. Infeasible SQL runs
  logged-only.
- Every action lands in `state.Store` with a before-image; undo restores it.
  **Undo restores row data only** — triggers, auto-increment counters, and
  statement-set `updated_at` don't come back.

### The AI agent (`internal/ai`, no TUI deps)

- `Client` speaks the OpenAI wire format. `NewAgentForClient` probes
  `ToolsSupported` and selects **native tools** or the **text-protocol fallback** —
  do not hardcode a transport on the TUI side.
- `Registry` holds the 10 tools. **Writes are gated in `Registry.Run`** on a `ConfirmFunc`
  — nil confirm ⇒ write rejected. `OnQuery` ships full rows to the results grid while
  `Summarize` gives the model a compact digest (never raw 100k rows).
- `Agent.Run` is bounded (12 calls), emits live `OnEvent`s, errors-as-data (a failed
  tool returns `"error: ..."` as its result so the model self-corrects), Esc cancels.

---

## Package playbook (add/change code here)

### `internal/tui` — the app

| File | Role |
|------|------|
| `model.go` | root model: tabs, focus, keymap, batch pump, connection wiring, AI wiring |
| `messages.go` | ALL shared `tea.Msg` types + `paneFocus` |
| `schema_pane.go` | DB/table tree navigation → `browseRequestMsg` |
| `editor.go` | multiline SQL editor + history → `runQueryMsg` |
| `results.go` | virtualized grid: sort/filter/keyset on `Columnar` |
| `rowpanel.go` | JSON-shaped per-field row editor (see Known gaps) |
| `history.go` | action history list → `undoActionMsg` (see Known gaps) |
| `writer.go` | undo contract executor (typed SQL + structured writes) |
| `aipanel.go` | Ask/Quick panel, confirm dialog, Esc interrupt |
| `connect.go` | connection dialog |
| `statusbar.go` | one-line footer |

**Adding a pane:** create the file (pure state + `view()`), add msg types to
`messages.go`, add a `paneFocus` value, wire Tab cycle + keys in `model.handleKey`,
render it in `View`. Test the pane logic without a live DB.

### `internal/db` — MariaDB (read)

- **Never** modify `fetch.go`'s `Columnar` layout (column-major) without updating every
  consumer — this bit a plan review (test data was row-major).
- New SQL: quote identifiers with `QuoteIdentifier`, use positional `?` args. No string
  interpolation of identifiers.
- Bulk reads use a plain `Rows` loop into `RawBytes`→`string`; no sqlx convenience layer.

### `internal/db/mutate` — MariaDB (write, high scrutiny)

- The undo contract lives here: `Classify` (feasibility before execution), `RowEditor`
  (before/after images), `LoadNextSQL` (keyset pagination). Writes never bypass this
  package.
- See its `DOC.md` for the invariants (never undo DDL, quote identifiers, before-image
  in a transaction). This is the security-critical package — review hardest here.

### `internal/ai` — agent

- `Client`/`Agent`/`Registry`/`Session`/`transport` are unexported types where the
  caller only needs a factory. New endpoint behaviors belong behind the `transport`
  interface (`native.go` / `fallback.go`), selected in `NewAgentForClient`.
- New tools: add a `Tool` in `Registry.build()`. **Set `ReadOnly:false` only for real
  writes** — they route through the confirm gate automatically.

### `internal/state` — SQLite

- `store.go` owns the schema migration (actions + turns tables). Add tables there,
  `IF NOT EXISTS`, parameterized SQL only.
- The store is app-wide (opened once in `model.ensureStore`), shared across connections.

### `internal/config` — config

- `Profile` (connections) + `AI` (base_url/model/key). Credentials from
  `SQUAL_OPENAI_API_KEY` env override the file. File is 0600; never commit it.

---

## Conventions

- **Go**: `gofmt` + `go vet` clean, always. No comments that merely restate the code;
  comments carry *why* (undo contract, column-major layout, errors-as-data).
- **Descriptive names.** `isQueryRunning`, not `q`.
- **Standard library first.** Reach for a dependency only when stdlib is genuinely worse.
- **Columnar data** for anything sorted/filtered client-side (SPEC §4).
- **Msg types live in `messages.go`.** Never define a `tea.Msg` inline in a pane.
- **Test-first (TDD)** — the superpowers way. Unit-test pane logic without a live DB;
  live-DB tests `t.Skip` when unreachable.
- **Update the module's `DOC.md` in the same change as the code.** A change that
  touches a package invariant (listed in that package's `DOC.md`) without updating
  the doc is incomplete.
- **Conventional commits:** `<type>(<scope>): <body>`, scopes `tui|db|ai|state|config`.
  `fix` for defects found in review, `feat` for new capability.

## Destructive Git Operations — NEVER

Force-push, delete branches, rewrite history, hard-reset, `git clean`. To undo, make a
new commit/PR that reverses the change.

## Environment

- Go toolchain: `~/go-sdk/go/bin` (`export PATH="$HOME/go-sdk/go/bin:$PATH"` if needed).
- Test MariaDB: `127.0.0.1:3306` `webstack`/`webstack` (docker `mariadb-master`),
  scratch schema `squal_smoke` (grant already applied).
- Smoke check: `go run ./cmd/smoke` — expects 100k rows fetched in <1s.

## Known gaps (not bugs — tracked scope)

The reading half is MVP-grade; the write half is now reachable but not polished.
Tracked as GitHub issues in `claymor333/squal`:

- **Row panel + hand-driven writes are wired** (#1, #2 closed in the 2026-08-10
  TUI rework): `enter`/`o` on a browsed row opens the right-side row panel, `s`
  saves, `delete` confirms, `u` opens the history panel for undo.
- **Deferred polish**: NULL vs empty-string handling in the row panel (#3),
  expandable BLOB/JSON cells (#4), column headers + horizontal scroll (#5),
  searchable history (#6), AI confirm-dialog polish (#7), AI session persistence
  wiring (#8).

## Verification Scripts

Do not create verification scripts when tests cover the behavior. `cmd/smoke` is the
only sanctioned integration check (real MariaDB).

## Documentation Files

Only create documentation files when explicitly requested.
