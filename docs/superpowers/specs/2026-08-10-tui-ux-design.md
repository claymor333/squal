# TUI Layout & Interaction Rework — Design

**Status:** Draft — for user review before implementation
**Applies to:** `internal/tui` (`model.go`, `mouse.go`, `results.go`, `editor.go`,
`schema_pane.go`, `rowpanel.go`, `history.go`, `aipanel.go`, `statusbar.go` + new
`layout.go`, `help.go`)
**Supersedes:** Nothing. Extends the v1-core surfaces (B4 layout, B6 statusbar,
A3 AI panel) and closes the known-gap issues #1 and #2 in AGENTS.md.

## Why this exists

The current TUI is a fixed vertical stack: tabs → schema → editor (1 line) →
results (hardcoded 8 rows) → AI → status, all rendered as one unbounded string
(`model.go:700`). Three defects define the ask:

1. **Overflow.** The schema tree renders uncapped. A database with many tables
   pushes editor/results/AI off the bottom of the terminal. Not polish — it
   breaks on real data.
2. **Dead grid.** Results rows do nothing. Row panel is unreachable (issue #1),
   save/delete/undo unwired (issue #2). "Browse and edit" is half a product.
3. **Single-line editor.** `Enter` runs the query (`model.go:365`), so multi-line
   SQL is impossible. SQL without newlines is a write-off.

Scope is: make the app not-broken, then make it feel like a tool. Structural code
(undo contract, streaming fetch, keyset pagination, AI agent) is untouched.

## Decisions

| Decision | Choice |
|----------|--------|
| Layout | Real pane **rectangles** derived from window size. All four main panes stay visible (browsing-first, SPEC §3); focus determines height weight. No more one-string render. |
| Overflow fix | Schema tree **scrolls internally**; results viewport height computed from available space (floor 8). Nothing renders off-screen. |
| Mouse | Hit-test against the real layout rects. Replaces the hand-computed Y-arithmetic in `mouse.go`. |
| Editor | **Multiline.** `Enter` inserts a newline; `Alt+Enter` / `Ctrl+R` runs; `Ctrl+P` / `Ctrl+N` walk history (up/down move the cursor now). |
| Results | Selectable column header (`←`/`→`); `s` sorts the cursor column (▲/▼ already exist); `f` opens an incremental filter — `resultsView.matches()` already implements the predicate, it just isn't bound. |
| Row panel | Opens on a highlighted row (`enter`/`o`) as a **right-side panel** (SPEC §5). Save is always-undoable via `RowEditor.Update`. `Delete` on a row → confirm modal (SPEC §6). Structured edits require a browsed table (PK known). |
| History | `u` toggles a history panel over the lower results area; `enter` selects an action to undo; `esc` closes. Feeds from `state.Store.List`. |
| Help | `?` opens a centered overlay listing every key by pane. |
| Status bar | Left: conn · rows · elapsed (streaming counter while loading). Right: keys for the focused pane. Errors are **scoped to the pane that owns them** — the single global `m.lastErr` slot (`model.go:124`) is gone. |
| Close safety | `q` opens a **confirm modal** ("close connection?"); `Ctrl+D` closes directly. One keystroke should never drop a tab. |
| AI panel | Header labels `Ask AI` vs `NL→SQL` (replaces opaque `[Ask]`/`[Quick]`), scrollable transcript, write-confirm rendered as a bordered centered modal. |

## Layout v2

Given `(w, h)` from `tea.WindowSizeMsg`, a new pure `layout` type computes a rect
per surface. Every line is accounted for; nothing renders past `h`.

```
┌ tabs (1 line) ────────────────────────────────────────────┐
├ schema ─────────────────────┬─────────────────────────────┤
│ (scrollable tree,           │                             │
│  weight grows when focused) │   row panel (right column,  │
├ editor ─────────────────────┤   only while open, SPEC §5): │
│ 1 line idle → N lines when  │   field inputs styled as    │
│ focused, multiline          │   JSON, raw-JSON toggle     │
├ results ────────────────────┤                             │
│ viewport = remaining height │   occupies right column of  │
│ floor 8, keyset next page   │   the editor+results region │
├ ai (1..~6 lines) ───────────┤                             │
├ status (1 line) ────────────┴─────────────────────────────┤
```

Rules:

- **Tabs** = row 0, **status** = last row. Body = `h - 2` rows split top-to-bottom.
- **Weights** (when not focused / focused): schema 30/45%, editor 1 line/`min(8, body/3)`,
  results remainder (floor 8), ai 1 line/`min(6, body/4)`. Focused pane expands, idle
  panes collapse — never zero, always ≥ their floor.
- **Schema scrolls internally.** `schemaPane` gains a `top` offset and renders a
  viewport of the tree. A 500-table schema no longer affects other panes.
- **Results viewport** = computed lines, replacing the `resultsViewport = 8` const
  (`model.go:592`). Floor 8, degrading to 1 when the window is too short to fit every
  pane (tabs + status always win; nothing collapses below one line). Cell width =
  `(w - borders) / ncols`, replacing the hardcoded 24-char `truncate` (`model.go:808`).
- **Row panel** is a right-hand column of the editor+results region, `w/3` wide,
  present only while open. Focus can move into it (`tab` cycle includes it when
  open).
- Panes expose small `SetViewport(...)`/`SetLines(...)` setters the model calls
  each frame from the layout; their bodies stay pure and unit-testable.

### Mouse

`handleMouse` hit-tests `(x, y)` against the computed rects (click header focuses,
click results body selects row, wheel scrolls). The old fixed-arithmetic
`paneLayout.at` (mouse.go:25-65) is deleted. The DOC.md invariant — "mouse layout
is computed, not hardcoded" — is preserved; it now derives from the same `layout`
the renderer uses, so it cannot drift.

## Keymap v2

| Key | Schema | Editor | Results | Row panel | AI | Global |
|-----|--------|--------|---------|-----------|----|----|
| `tab`/`shift+tab` | cycle focus | cycle | cycle | cycle | cycle | |
| `↑`/`↓` | move / scroll tree | move cursor line | move row | move field | | |
| `←`/`→` | | move cursor char | move column | | | |
| `enter` | open db / browse table | (insert newline) | **open row panel** | edit value | run ask/quick | |
| `alt+enter` | | **run query** | | | | |
| `ctrl+r` | | run query (alt) | | | | |
| `backspace` | | delete char | | edit value | | |
| `s` | | | sort cursor column | | | |
| `f` / `esc` | | | filter in/out | | | |
| `n` | | | keyset next page | | | |
| `o` | | | open row panel | | | |
| `delete` | | | → confirm delete | | | |
| `c` | new connection | | | | | |
| `u` | | | toggle history panel | | | |
| `q` | → confirm close | text | | | | |
| `ctrl+d` | close connection | | | | | |
| `?` | help | help | help | help | help | help |
| `esc` | | | close panel | close panel | interrupt | close modal |
| `y`/`n` | | | confirm delete | | confirm write | confirm close |

`Ctrl+P`/`Ctrl+N` walk editor history; `Ctrl+C` still quits.

## Error scoping

`m.lastErr` (model.go:124) is replaced by per-connection error fields:

- schema/connection error → rendered inside the schema pane, on its own tab;
- results error → inside the results pane (keeps the grid);
- writer/AI error → inside the AI panel;
- transient (e.g. "connection closed") → status bar slot, cleared on next action.

A failed query on tab A no longer wipes a success on tab B. Verdicts (undoable /
logged-only) stay in the status bar / history as today.

## Testing

Pure pane logic first, no live DB: layout rects, schema scroll clamp, results
col-nav + filter, editor cursor/run keys, help content, statusbar render, row
panel open/save state. Live-DB paths (RowEditor save/delete, store.List for
history) are integration-verified by `cmd/smoke` or skipped when unreachable.
Mouse hit-tests are unit-tested against `layout` fixtures.

## File ownership / sequencing

Model.go is the shared file — strictly sequenced (matches tui DOC.md invariant).

| Group | Tasks | Why |
|-------|-------|-----|
| **G1 [SEQ]** | **U1** layout shell (`layout.go`, `model.go`, `mouse.go`, pane viewport setters) | Everything renders inside these rects; must land first. |
| **G2 [PAR]** | **U2** results interaction, **U3** multiline editor | Disjoint files: `results.go`, `editor.go`. |
| **G3 [SEQ]** | **U4** bind U2/U3 keys + statusbar hints | Owns `model.go`. |
| **G4 [SEQ]** | **U5** row panel reachable + save/delete confirm | Owns `model.go`, touches `rowpanel.go`/`writer.go`. |
| **G5 [SEQ]** | **U6** history pane + undo | Owns `model.go`, touches `history.go`. |
| **G6 [PAR]** | **U7** help overlay, **U8** statusbar/errors view, **U9** AI panel polish | Disjoint files: `help.go`, `statusbar.go`, `aipanel.go`. |
| **G7 [SEQ]** | **U10** integration + close safety + error routing | Final `model.go` pass; binds `?`, `q`→confirm, routes errors. |

## Deferred (still tracked — known gaps #3–#8)

- NULL vs empty-string toggle in the row panel (#3)
- Expandable BLOB/JSON/TEXT cells, JSON pretty / BLOB hex+length (#4)
- Column headers + horizontal scroll for wide tables (#5)
- Searchable history (#6) — `u` panel lists; a filter box comes later
- AI session persistence wiring (#8)
- Per-field type validation in the row panel (SPEC §5 "no syntax errors possible") —
  U5 ships the inline edit buffer; per-type validation stays deferred
