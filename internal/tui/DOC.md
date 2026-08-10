# tui — bubbletea app

The TUI. Panes own their view + pure state; `model.go` is the dispatcher that
routes keys and messages. Panes talk to the DB only through the services
(`writer`, the fetch pump) — never directly.

## Owns

| File | Role |
|------|------|
| `model.go` | root model: tabs, focus, keymap, batch pump, connection + AI wiring, layout-driven render, rail state |
| `messages.go` | ALL shared `tea.Msg` types + `paneFocus` (rail tabs: row/hist/ai) |
| `layout.go` | pure pane-rect computation (five-zone: L/T/B/R/F), `clip`/`fit`/`paneBox` |
| `mouse.go` | `handleMouse` (rect hit-testing: click focus, click-select, wheel scroll, rail tab click) |
| `schema_pane.go` | DB/table/column tree (filter + collapse + internal scroll) → `browseRequestMsg` |
| `editor.go` | multiline SQL editor + history → `runQueryMsg`; default-DB title |
| `results.go` | virtualized grid on `Columnar`, viewport settable, result-set ring |
| `rowpanel.go` | row-editing rail tab (inline edit, raw JSON, save) |
| `history.go` | action history rail tab → `undoActionMsg` |
| `writer.go` | undo contract executor (the write service); `editorFor` + structured save/delete |
| `aipanel.go` | Ask/NL→SQL rail tab, confirm dialog, Esc interrupt |
| `help.go` | `F1`/`?` key-reference overlay |
| `connect.go` | connection dialog |
| `statusbar.go` | one-line footer (scoped error slot) + toast line |

## Invariants (do not break)

- **Msg types live in `messages.go`, never inline in a pane.**
- **The renderer and the mouse share one geometry source.** `layout.rects`
  derives pane rects from the window size; `View` renders inside those rects and
  `handleMouse` hit-tests them. A new pane means one change in `layout.go` plus
  its tests — never hand-maintained Y-arithmetic.
- **View never overflows the window.** Every pane body is clipped (`paneBox`, which
  pads to its rect height and `MaxWidth`-truncates — lipgloss `Width` wraps) to its
  rect; the schema tree scrolls internally; the results viewport is computed, not a
  constant. Anything that renders must stay inside `(w, h)`.
- **The context rail is one zone.** row/hist/ai are rail *tabs* (lazily created);
  exactly one is visible at a time. Never stack them back into separate panes.
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
  `paneFocus` value, Tab cycle + keys in `model.handleKey`, a rect in `layout.go`,
  render in `View`.
- New keymap entry → `handleKey` switch.

## Tests

`go test ./internal/tui/` — pane logic unit tests without a live DB (panes are
state + view, so they're testable as pure logic).

## Doc check

Did this change touch an invariant above? If yes, update it here.

## Known gaps (see AGENTS.md — tracked as issues)

Row panel, history, and writes are wired (#1/#2 closed); the five-zone rail
ships row/hist/ai tabs. Remaining polish items #3–#8 stand; `?` help is
full-screen rather than a centered overlay.
