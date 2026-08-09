# Squall — AI-enabled TUI SQL/MariaDB browser

A terminal UI for browsing and editing MariaDB (and MySQL-compatible) databases, with
an OpenAI-compatible AI endpoint for natural-language → SQL, plus undoable write
actions. Goal: beat HeidiSQL's 1k-row wall with a fast, keyboard-first, local-only tool.

## Guiding principles

1. **Read is primary.** A browser first; structured writes are the on-ramp to editing.
2. **No undo lies.** Every action is either *undoable* or *logged-only* — and the UI
   says which, before and after execution. Never a silent middle ground.
3. **AI is a verb on data, not a chat box.** The AI can *run* queries and *show* data,
   with multi-turn memory per tab.
4. **Local connections only** for now. No SSH tunnels, no remote orchestration.

---

## 1. Stack

- **Language**: Go
- **TUI**: bubbletea (bubbletea event loop maps cleanly onto goroutine → message for
  async fetch)
- **DB driver**: `database/sql` + `go-sql-driver/mysql`, binary protocol via prepared
  statements, plain Rows loop for the bulk path (no sqlx convenience wrappers)
- **SQL parser** (for undo feasibility): `github.com/xwb1989/sqlparser` (Vitess),
  upgrade to `pingcap/tidb/parser` if the syntax surface needs it
- **AI**: OpenAI-compatible HTTP endpoint. Config: `base_url`, `api_key`, `model`.
  Works with opencode's endpoint, OpenAI, Ollama, vLLM, LM Studio, or any proxy.
  Key via `SQUAL_OPENAI_API_KEY` env var, never in the config file's history.

---

## 2. Connections

- Local only. Connect via host:port or unix socket, user, password, database,
  charset, optional SSL toggle, connect + read timeouts.
- Multiple connections **at once**, one per tab.
- Saved profiles in a config file (mode `0600`). Credentials never written to shell
  history, never logged. Env-var override for password when a profile omits it.
- Header shows active connection; a switcher cycles open connections.

## 3. Tabs

- **One tab per connection** (schema + editor + results are already three panes;
  per-query tabs would be chaos). Each tab owns:
  - schema tree pane
  - SQL editor pane
  - results pane
  - row editor side panel (opens on row highlight)
  - AI panel (session-scoped to this tab)
- Tab lifecycle: close = close connection; connection errors surface inline, never
  block the TUI.

## 4. Browsing / result grid

- Progressive fetch: query runs on a goroutine, results stream into an append-only
  buffer, repaint per batch. First rows appear instantly, the rest fill in.
- Virtualized viewport: only visible rows are rendered.
- Default fetch cap **50k rows**, `load next` continues via keyset pagination using the
  table's primary key when the query is a bare `SELECT ... FROM t`. `OFFSET` is never
  used for large jumps.
- Client-side sort + filter over the loaded buffer (columnar storage:
  `[][]string`, one slice per column). Sort/filter never hit the server once loaded.
- Columns: header row, horizontal scroll for wide tables, column type shown on hover.
- Cell preview: truncated cells expand via the row detail panel.

## 5. Row editor / JSON side panel

- Highlight a row → right-side panel renders the row as **per-field inputs styled like
  a JSON object** (key: value pairs). Validation per type; no syntax errors possible.
- Toggle to a **raw JSON textarea** for copy/paste, with parse validation on the way out.
- Save = `UPDATE ... WHERE pk`. Always undoable (see §6).
- Giant `TEXT`/`JSON`/`BLOB` columns render as expandable content in the panel:
  JSON pretty-printed, BLOB as hex + length. Editing large values happens here too.
- NULL handling: explicit NULL vs empty string; a column toggle for it.

## 6. Writes, history, and undo — THE CONTRACT

### App-side action log (the undo store)

- Every write action is recorded in an app-side DB (SQLite):
  connection, table, PK, before-image, after-image, timestamp, statement, verdict.
- History is browsable and searchable; each entry is tagged **undoable** or
  **logged-only**, visible before and after execution.

### Structured edits (row editor, JSON panel, grid delete)

- Always undoable. App fetches the full row before writing → before-image → executes →
  after-image. Undo = restore before-image.
- DELETE via grid: `Delete` key → confirm dialog. Also exposed in the JSON panel.

### Raw SQL and AI-generated SQL

- Parser judges **undo feasibility before execution**:
  - Single-table statement with a translatable predicate → undoable. Runs in a
    transaction, before-image SELECT synthesized and captured, logged. Undo restores.
  - Not provable (multi-table UPDATE/DELETE, subqueries across tables, `INSERT ... SELECT`
    unverifiable, DDL, `TRUNCATE`) → runs as **logged-only**, flagged
    `NOT UNDOABLE` in the history panel. No silent middle ground.
- `INSERT` undo = delete the inserted PKs (via `LAST_INSERT_ID` / readback).
- AI-generated queries go through the exact same contract as typed SQL.

### Honest limits of undo

- Undo restores **row data only**, not side effects: triggers fired by the write,
  auto-increment counters, statement-set `updated_at` timestamps.
- DDL is never undoable. `TRUNCATE` is never undoable. No mechanism fixes this.
- If the parser cannot prove safety, the answer is *logged-only*, always.

## 7. AI panel (per tab)

- OpenAI-compatible endpoint: `base_url` + `model` + `api_key` (env var).
  Any provider speaking the OpenAI wire format works.
- **Multi-turn memory**: the panel keeps the conversation for this tab, including the
  queries it ran and their results. Refinements build on prior turns.
- Flow: NL request → schema-aware prompt (trimmed schema of referenced tables) → SQL →
  **executes it** → renders rows → user refines ("only the ones with >3 orders").
- Verbs available:
  - **Ask/Write**: NL → SQL → run → show data
  - **Refine**: adjust the previous query in-context
  - **Explain**: a query or a result set, in plain English
  - **Fix**: paste an error, get corrected SQL (feasibility-checked before running)
- The AI never runs SQL silently: every generated statement passes the §6 contract and
  requires confirmation before execution.

## 8. Config & app storage

- `~/.config/squal/config.toml` (or `SQUAL_CONFIG`): profiles, AI endpoint config, prefs.
  Mode `0600`.
- App-state DB: `~/.local/share/squal/state.db` — action log, AI session memory, history.

## 9. Out of scope (v1) — deliberate

- SSH tunnels / remote orchestration (local only)
- Full-screen freeform AI chat panel (AI verbs, not a chat box)
- Transactions / explicit session management (connect → query → results is the loop)
- Replica/binlog-based undo (option if binlog ROW format is *detected*; not built on)
- Multi-user / team features

## 10. Compatible environments

- **OS**: Linux (primary), macOS, and **native Windows**. Go + bubbletea are portable
  across all three; `go-sql-driver` is pure Go (no CGO) so cross-compilation from any
  host to `windows/amd64` just works. Windows MariaDB server is a first-class platform.
- **Terminals**: Linux/macOS — anything ANSI/xterm-compatible: tmux, screen, Alacritty,
  Kitty, foot, iTerm2, GNOME Terminal. **Windows — Windows Terminal only** (full
  ANSI/truecolor/alt-screen/mouse; this is the target). Legacy `conhost` is
  **unsupported**: partial ANSI, flaky mouse, alt-screen glitches — document it and
  move on rather than fight it.
- Truecolor + UTF-8 assumed; bubbletea degrades gracefully to 256-color when the
  terminal can't do it.
- **Databases**: MariaDB 10.3 → 11.x (wire-compatible; MySQL 5.7/8.0 will *mostly*
  work but is untested → document "works, not supported").
- **Go toolchain**: 1.22+; cross-compiles to any platform above from any host.
