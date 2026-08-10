# TUI Layout & Interaction — Design

**Status:** Approved. Phase 1 (U1–U10) shipped — commit `7b1bda6`. Phase 2
(U11–U14) is the five-zone convergence: right rail becomes a tabbed context
rail, results get a history ring, the schema tree goes three levels deep, and
the footer gains toasts.
**Applies to:** `internal/tui` (`model.go`, `mouse.go`, `layout.go`, `help.go`,
`results.go`, `editor.go`, `schema_pane.go`, `rowpanel.go`, `history.go`,
`aipanel.go`, `statusbar.go`)
**Supersedes:** Nothing. Extends the v1-core surfaces (B4 layout, B6 statusbar,
A3 AI panel) and closes the known-gap issues #1 and #2 in AGENTS.md.

## Why this exists

Phase 1 fixed three defects of the original fixed vertical stack:

1. **Overflow.** The schema tree rendered uncapped and pushed panes off-screen.
   Real pane rects + internal scroll made overflow impossible.
2. **Dead grid.** Results rows did nothing. The row panel, save/delete, and
   undo are now wired through the undo contract.
3. **Single-line editor.** Now multiline: `Enter` = newline, `Alt+Enter`/`Ctrl+R`
   runs.

Phase 2 converges the app onto a **five-zone canvas** so every surface has one
home: the row panel, history, and AI panel currently *stack* in the right column
and fight for height; the schema tree only goes two levels; running a query
destroys the previous result set; and write verdicts have no voice except the
status bar. Scope is presentation and navigation — structural code (undo
contract, streaming fetch, keyset, AI agent) is untouched.

## Decisions

| Decision | Choice |
|----------|--------|
| Layout | Real pane **rectangles** derived from window size. Five-zone canvas: L left rail, T top (editor), B bottom (results stage), R right rail (tabbed context), F footer. One pane per zone — nothing stacks. |
| Overflow fix | Every pane renders inside its rect (height-clipped, `MaxWidth`-truncated); schema scrolls internally; results viewport computed (floor 8, degrades in tiny windows). Nothing renders off-screen. |
| Mouse | Hit-test against the same rects the renderer uses, so geometry cannot drift. |
| Editor | **Multiline**, cursor movement, `Ctrl+P`/`Ctrl+N` history. The editor border shows the **current default database** — the schema unqualified SQL resolves against. |
| Results | Selectable column header (`←`/`→`), `s` sorts the cursor column, `f` incremental filter, `n` keyset next page. Plus a **result-set ring**: keep the last N grids, `[`/`]` flip, `del` drops. Honours SPEC §3's no-query-tabs decision. |
| Schema tree | Three levels: **db → table → columns**. `/` incremental filter over the tree; `L` collapses the left rail to give the stage full width; estimated row counts beside tables. |
| Right rail | **Tabbed context rail** — one zone, three tabs: `row` (highlighted row editor), `hist` (undo log), `ai` (prompt + transcript). Switch with `1`/`2`/`3` (or `tab` inside the rail). Auto-focus: after a write → `hist`; when the agent runs → `ai`. |
| Row panel | Opens on a highlighted row (`enter`/`o`) as the `row` tab. Save is always-undoable via `RowEditor.Update`. `Delete` on a row → confirm modal. Structured edits require a browsed table (PK known). |
| History | The `hist` tab. `enter` selects an action to undo; `esc` closes the rail. Feeds from `state.Store.List`. |
| Toasts | A second footer line for **transient write verdicts** ("saved — undoable", "deleted", "undo done") and errors. Auto-clears; distinct from the persistent left/right status cluster. |
| Help | `F1` / `?` (outside text panes) opens the key-reference overlay. |
| Close safety | `q` confirms before closing a connection; `Ctrl+D` closes directly; `Ctrl+C` **always exits the app** (never drops a tab), closing connections + store first. |
| AI panel | `Ask AI` vs `NL→SQL` labels, scrollable transcript, write-confirm prompt. Lives on the `ai` rail tab. |

## Layout: the five-zone canvas

Given `(w, h)`, a pure `layout` type computes a rect per surface. Row 0 is the
tab bar, the last row is the footer. The body between them has **two columns**
(L and R) and the right column has **two rows** (T and B).

```
┌ tabs (1 line) ────────────────────────────────┬────────────────────────────┐
├ L left rail          │ T top (editor)         │ R right rail (tabbed)      │
│  schema tree         ├────────────────────────┤  [row | hist | ai]         │
│  db > table > cols   │ B bottom (results      │   one tab active          │
│  (rail-collapsible)  │   stage, main canvas)  │                           │
├ footer ──────────────┴────────────────────────┴────────────────────────────┤
│ conn | rows | elapsed | keys          [error slot]          [toast line]  │
```

Rules:

- **Tabs** = row 0, **footer** = last 1–2 rows. Body = `h - 2` rows.
- **L (left rail)** — the schema tree, `22%` wide (32% when focused), full body
  height. `L` collapses it to a sliver (or hides it) for maximum stage width.
  Scrolls internally; a 500-table schema never affects other panes.
- **R (right rail)** — one tabbed zone on the right, `rightW/3` wide when open,
  full body height. Closed (zero width) when no tab is active. Contains `row`,
  `hist`, and `ai` tabs; exactly one visible at a time.
- **T (editor)** — `1` line idle → `min(8, body/3)` when focused. Border shows
  the current default database.
- **B (results stage)** — the largest surface, `min` floor 8 rows. Holds the
  active result set plus the ring of recent ones. This is where the product
  lives.
- **F (footer)** — line 1: conn · rows · elapsed · focused-pane keys + scoped
  error slot. Line 2 (transient): toasts.
- Panes expose `SetViewport(...)`/`SetLines(...)` setters the model calls from
  the layout each frame; their bodies stay pure and unit-testable.

### Mouse

`handleMouse` hit-tests `(x, y)` against the same rects the renderer uses (click
focus, click row select, wheel scroll, click a rail tab to activate it).

## The right rail

The rail is a single `rect` with a 1-line tab strip on top (`row | hist | ai`)
and the active tab's body below. State lives on `connData`:

- `railTab paneFocus` — which tab is active; `railOpen bool`.
- `row` / `hist` / `ai` are **created lazily** when their tab is activated, not
  all at once — activating `row` opens the row editor, activating `hist` fires a
  `store.List`, activating `ai` shows the panel.
- Auto-focus rules: opening the row editor (`enter`/`o`) activates `row`; a
  completed write activates `hist`; the agent's first tool call activates `ai`.
  The user can override with `1`/`2`/`3` at any time.

The tab strip replaces the old "history is a bottom band" placement — the
`u`-toggle history band is gone.

## Result-set ring

`connData` keeps `resultsRing []*resultsView` (cap 8) plus the active one.
Running a query pushes the current grid onto the ring. `[` / `]` flip through
the ring (each grid keeps its own sort/filter/scroll), `del` drops the shown
one, and `enter` on a ring grid re-sorts/edits as normal. The footer shows
`ring i/N` when more than one exists.

## Keymap v3

**Shortcuts always use a modifier** (`alt+…`, `ctrl+…`, `F1`, `del`, `pgup`/`pgdn`).
Bare keys are reserved for text input (SQL, filters, the AI request) and
navigation (arrows, `enter`, `esc`, `tab`). This is what lets the editor and
filters accept any character without a shortcut hijacking it. The F1 overlay
renders from `key.Binding`s, so it is the live source of truth.

| Key | L rail (schema) | T editor | B results | R rail | Footer | Global |
|-----|-----------------|----------|-----------|--------|--------|--------|
| `tab`/`shift+tab` | cycle zones | cycle | cycle | cycle | | |
| `↑`/`↓` | move / scroll tree | move cursor line | move row | move tab body | | |
| `←`/`→` | | move cursor char | move column | | | |
| `enter` | open db / browse | newline | open row (rail) | | | |
| `alt+enter` | | **run query** | | | | |
| `alt+f` | filter tree | | filter | | | |
| `alt+x` | expand columns | | | | | |
| `alt+s` | | | sort cursor column | save (row tab) | | |
| `alt+n` | | | keyset next page | | | |
| `pgup`/`pgdn` | | | result ring prev/next | | | |
| `del` | | | ring drop / delete row | | | |
| `alt+1`/`2`/`3` | | | | rail tab row/hist/ai | | |
| `alt+l` | | | | | | collapse left rail |
| `alt+c` | new connection | | | | | |
| `alt+q` | | | | | | confirm close |
| `ctrl+d` | | | | | | close connection |
| `ctrl+c` | | | | | | **exit app** |
| `F1` | help | help | help | help | | help |
| `esc` | end filter | | end filter | close rail / cancel edit | | dismiss |

`Ctrl+P`/`Ctrl+N` walk editor history; `Ctrl+R` also runs the query. The AI tab:
`enter` runs, `alt+a` toggles Ask/NL→SQL, bare keys type; `y`/`n` answer a
pending write confirm. A confirm modal answers with bare `y`/`n`/`esc`.

## Error scoping

Errors are scoped to the zone that owns them:

- schema/connection error → rendered inside the left rail on its tab;
- results error → inside the results stage (keeps the grid);
- writer/AI error → inside the rail's `hist`/`ai` tab;
- transient (config save, store read, "connection closed") → footer error slot,
  cleared on the next key.

A failed query on tab A never wipes a success on tab B. Verdicts (undoable /
logged-only) go to the toast line and stay in the history tab.

## Testing

Pure pane logic first, no live DB: layout rects (five-zone geometry, rail
open/closed, ring caps), schema column-depth + filter + collapse, ring
push/flip/drop, rail tab switching + lazy activation, toast render/clear,
statusbar, help. Live-DB paths (RowEditor save/delete, `store.List`) stay
integration-verified by `cmd/smoke` or skipped when unreachable. Mouse hit-tests
are unit-tested against `layout` fixtures.

## File ownership / sequencing

`model.go` is the shared file — strictly sequenced (tui DOC.md invariant).
Phase 1 (U1–U10) is shipped.

| Group | Tasks | Why |
|-------|-------|-----|
| **G8 [PAR]** | **U11** schema depth + filter + collapse, **U12** result-set ring, **U14a** toast view + editor default-DB | Disjoint files: `schema_pane.go`, `results.go`, `statusbar.go`/`editor.go`. Pure pane logic + tests; no `model.go`. |
| **G9 [SEQ]** | **U13** rail unification + key binding + toast routing | Owns `model.go` + `layout.go`; binds G8 keys, turns the right strip into the tabbed rail, moves history/AI onto it, routes toasts. Last pass. |

## Deferred (still tracked — known gaps #3–#8)

- NULL vs empty-string toggle in the row panel (#3)
- Expandable BLOB/JSON/TEXT cells, JSON pretty / BLOB hex+length (#4)
- Column headers + horizontal scroll for wide tables (#5) — the ring and rail
  land first
- Searchable history (#6) — the `hist` tab lists; a filter box comes later
- AI session persistence wiring (#8)
- Per-field type validation in the row panel (SPEC §5 "no syntax errors possible")
