# tui — bubbletea app

The TUI. Panes own their view + pure state; `model.go` is the dispatcher that
routes keys and messages. Panes talk to the DB only through the services
(`writer`, the fetch pump) — never directly.

## Owns

| File | Role |
|------|------|
| `model.go` | root model: tabs, focus, keymap, batch pump, connection + AI wiring |
| `messages.go` | ALL shared `tea.Msg` types + `paneFocus` |
| `schema_pane.go` | DB/table tree → `browseRequestMsg` |
| `editor.go` | multiline SQL editor + history → `runQueryMsg` |
| `results.go` | virtualized grid on `Columnar` |
| `rowpanel.go` | JSON-shaped row editor (see Known gaps) |
| `history.go` | action history → `undoActionMsg` (see Known gaps) |
| `writer.go` | undo contract executor (the write service) |
| `aipanel.go` | Ask/Quick panel, confirm dialog, Esc interrupt |
| `connect.go` | connection dialog |
| `statusbar.go` | one-line footer |

## Invariants (do not break)

- **Msg types live in `messages.go`, never inline in a pane.**
- **Panes are pure-ish**: mutate `connData`, emit `tea.Msg`s, return `tea.Cmd`s.
  They don't hold a `*sql.DB`.
- **The batch pump orders reads.** The fetch goroutine appends into `Columnar`;
  the model reads it only after a `batchMsg` arrives (channel send/receive orders
  the writes). Don't read `Columnar` mid-fetch.
- **One owner of `model.go` at a time.** It's the shared file; sequence changes
  (SDD tasks own it exclusively in turn).
- **The write path must respect the undo contract** — `writer.runTypedSQL` decides
  undoable vs logged-only before executing; the UI says which.

## Change here

- New pane → new file (pure state + `view()`), msg types in `messages.go`, a
  `paneFocus` value, Tab cycle + keys in `model.handleKey`, render in `View`.
- New keymap entry → `handleKey` switch.

## Tests

`go test ./internal/tui/` — pane logic unit tests without a live DB (panes are
state + view, so they're testable as pure logic).

## Doc check

Did this change touch an invariant above? If yes, update it here.

## Known gaps (see AGENTS.md — tracked as issues)

Row panel unreachable (#1), hand-driven writes not dispatched (#2), plus polish
items #3–#8.
