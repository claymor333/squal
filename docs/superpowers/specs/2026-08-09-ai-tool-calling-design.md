# AI Tool-Calling Agent — Design

**Status:** Approved by user via brainstorming (2026-08-09)
**Applies to:** `internal/ai`, `internal/state` (turns), `internal/tui/aipanel.go`, `internal/tui/model.go`
**Supersedes:** Main v1-core plan Phase C (A1/A2/A3). Do NOT execute A1/A2/A3; this replaces the simple NL→SQL loop with a tool-calling agent.

## Decisions (from brainstorming)

| Decision | Choice |
|----------|--------|
| Tool surface | Full agent: schema tools, `run_query`, `run_write`, row tools, explain tools (10 tools) |
| Write autonomy | Read-only tools auto-run; every write (run_write + row-write tools) pauses for TUI confirm |
| Result feedback | Model gets a **summary**; full rows land in the main results grid (reuses B3 Fetch) |
| Tool protocol | Native OpenAI `tools` when supported; **text-protocol fallback** when not |
| Loop UX | Tool calls **stream live** in the panel; Esc interrupts anytime |
| Loop budget | **12-call cap** per prompt; on cap, force the AI to summarize and stop |
| Mode | **Ask** (agent loop) vs **Quick SQL** (one-shot NL→SQL) toggle on the panel |

## Architecture

- `internal/ai/client.go` — OpenAI-compatible client. `Complete` (plain) stays for Quick SQL + fallback final answers; adds `CompleteTools` (native `tools`/`tool_calls`) and endpoint support detection.
- `internal/ai/tools.go` — `Tool` struct + `Registry`. The 10 tools, each with JSON-schema params and an `Execute` func. `ReadOnly` decides auto vs confirm — no special-casing in the loop.
- `internal/ai/agent.go` — transport-agnostic loop. Defines `transport` interface; `Run` does `prompt → tool_calls → execute → feed back → repeat`, 12-call cap, forced summary, Esc cancel, errors-as-data.
- `internal/ai/native.go` — native OpenAI tools transport.
- `internal/ai/fallback.go` — text-protocol transport (tools as JSON in system prompt; model emits `{"tool":...,"args":{...}}` or final text). Same loop, different transport.
- `internal/ai/session.go` — multi-turn message history + schema packing.
- `internal/state/turns.go` — SQLite persistence of the tool-call transcript (new table; E1's actions table is untouched).
- `internal/tui/aipanel.go` — live streaming panel, Ask/Quick toggle, write-confirm dialog, Esc interrupt.

## Data flow

```
user prompt → panel → agent.Run(transport, registry, session)
   loop (max 12):
     transport.Complete(messages, tools)
     ├─ tool_calls → for each: confirm? → registry.Execute → result→messages
     │               emit toolStarted/toolResult (panel streams) + run_query ships grid
     └─ final text → return
   cap hit → forcedSummary()
panel: live events, Esc cancel, confirm dialog (blocks via channel until user answers)
```

`run_query` tool: runs `db.Fetch`, drains to `Columnar`, ships Columnar to the main results grid (for the user), computes `Summarize(col)` (count, cols, types, numeric min/max/avg, first N rows) for the model.

Errors as data: failed `run_query`/`run_write` returns the DB error string as the tool result so the model self-corrects. Esc returns partial. Confirm-decline returns `"user declined"`.

## Testing

`httptest` fake OpenAI server scripts `tool_calls`/text-protocol responses. Unit tests cover: loop termination, 12-cap + forced summary, fallback protocol, confirm-decline, error-as-data self-correction, native-tools JSON shape. Tools tested against live `squal_smoke` DB (skip when unavailable). Loop itself never needs a live DB.

## File ownership (parallel groups)

- **Group 1 [PARALLEL]:** AI-1 client+detection, AI-2 tools/registry+summary, AI-5 session+turns persistence
- **Group 2 [SEQUENTIAL]:** AI-3 agent loop (consumes AI-1/AI-2, defines transport interface)
- **Group 3 [SEQUENTIAL]:** AI-4 text fallback transport (implements AI-3's interface)
- **Group 4 [SEQUENTIAL]:** AI-6 panel + model.go wiring (must follow main-plan B6 on model.go)

Prereqs from main plan: E1 (actions store), E2 (Classify/UndoVerdict), E3 (RowEditor), B3 (Fetch/Columnar) must be landed first.
