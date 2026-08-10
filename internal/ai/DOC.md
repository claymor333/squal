# ai — OpenAI-compatible client + tool-calling agent

The AI layer. Zero TUI dependencies — it exposes factories, and the TUI wires
them to the panel. This is the reference shape for the repo: transport behind an
interface, gates centralized, no hardcoded endpoint behavior in callers.

## Owns

| File | Role |
|------|------|
| `client.go` | `Client`, `Message`/`ToolCall`/`ToolDef`/`Response`, `CompleteTools`, `ToolsSupported` |
| `native.go` | `nativeTransport` (OpenAI native `tools` parameter) |
| `fallback.go` | `textTransport` (tools embedded in the system prompt) |
| `agent.go` | `transport` interface, `Agent` loop (12-call budget), `NewAgentForClient` |
| `tools.go` | `Tool`, `Registry` (incl. `Run` write-gate), the 10 tools, `Summarize`, `ensureEditor` |
| `session.go` | `Session` multi-turn memory |

## Invariants (do not break)

- **Transport selection belongs here, not the TUI.** `NewAgentForClient` probes
  `ToolsSupported` and picks native or text fallback. A plan review caught the TUI
  hardcoding native — do not reintroduce that.
- **Writes are gated in `Registry.Run`** on a `ConfirmFunc`. Nil confirm ⇒ write
  rejected. No caller (loop, panel, tool) can bypass it.
- **Errors-as-data.** A failed tool returns `"error: ..."` as its *result* so the
  model self-corrects. `Agent.runTool` never aborts the loop on a tool error.
- **The model never sees raw 100k rows.** `run_query` returns `Summarize` (count,
  cols, types, numeric min/max) and ships full rows via `OnQuery` to the grid.
- **`ReadOnly:false` only for real writes.** Set it on `run_write`/`update_row`/
  `delete_row` and nothing else.

## Change here

- New tool → `Tool` in `tools.go` + register in `Registry.build()`.
- New endpoint behavior → a new transport behind `transport`, selected in
  `NewAgentForClient`.
- New model capability → extend `Agent.Run`'s loop or `Session`.

## Tests

`go test ./internal/ai/` — httptest fake server scripts tool_calls/fallback;
loop termination, 12-cap, error-as-data, confirm gate, fallback selection.

## Doc check

Did this change touch an invariant above? If yes, update it here.
