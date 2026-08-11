---
name: superpowers-using-superpowers
description: Use when starting any conversation - establishes how to find and use skills, requiring skill invocation before ANY response including clarifying questions
---

<SUBAGENT-STOP>
If you were dispatched as a subagent to execute a specific task, ignore this skill.
</SUBAGENT-STOP>

<EXTREMELY-IMPORTANT>
If you think there is even a 1% chance a skill might apply to what you are doing, you ABSOLUTELY MUST invoke the skill.

IF A SKILL APPLIES TO YOUR TASK, YOU DO NOT HAVE A CHOICE. YOU MUST USE IT.

This is not negotiable. You cannot rationalize your way out of this.
</EXTREMELY-IMPORTANT>

## The Rule

**Invoke relevant or requested skills BEFORE any response or action** — including clarifying questions, exploring the codebase, or checking files. If it turns out wrong for the situation, you don't have to use it.

**Before entering plan mode:** if you haven't already brainstormed, invoke the brainstorming skill first.

Then announce "Using [skill] to [purpose]" and follow the skill exactly. If it has a checklist, create a todo per item.

## Skill Priority

When multiple skills apply, process skills come first — they set the approach, then implementation skills carry it out.

- "Let's build X" → superpowers-brainstorming first, then implementation skills.
- "Fix this bug" → superpowers-systematic-debugging first, then domain skills.

## Red Flags

These thoughts mean STOP—you're rationalizing:

| Thought | Reality |
|---------|---------|
| "This is just a simple question" | Questions are tasks. Check for skills. |
| "I need more context first" | Skill check comes BEFORE clarifying questions. |
| "Let me explore the codebase first" | Skills tell you HOW to explore. Check first. |
| "This doesn't need a formal skill" | If a skill exists, use it. |
| "The skill is overkill" | Simple things become complex. Use it. |
| "I'll just do this one thing first" | Check BEFORE doing anything. |

## User Instructions

User instructions (CLAUDE.md, AGENTS.md, direct requests) take precedence over skills, which in turn override default behavior. Only skip skill workflows or instructions when your human partner has explicitly told you to.

## Harness Detection

At the start of each session, detect whether running inside OpenChamber or plain CLI.

**Detection function:**

```bash
detect_harness() {
  if curl -s --max-time 1 "http://127.0.0.1:3000/api/session-activity" >/dev/null 2>&1; then
    echo "openchamber"
  else
    echo "cli"
  fi
}
```

**How to use:** Check once at session start. Set `HARNESS=openchamber` or `HARNESS=cli`.

**What changes based on harness:**

| Capability | OpenChamber | CLI |
|------------|-------------|-----|
| Review loops | `task` tool subagent | `task` tool subagent |
| Parallel dispatch | `task` tool (parallel) | `task` tool (concurrent) |
| Worktree creation | `openchamber` tool if available, else OpenChamber worktree UI | `git worktree` CLI |
| PR creation | Visual Git sidebar | `git push` + API |
| Plan viewing | OpenChamber Plan view | Markdown file |
| Inline feedback | Comment drafts on diffs | Not available |

**The `openchamber` tool (when available):** use it only to create user-facing sessions, worktree sessions, and scheduled tasks the human follows up on. Never use it to delegate parts of your own current task — for that, use the `task` tool.

**Skills that use harness context:**
- `superpowers-using-git-worktrees` — worktree creation routing
- `superpowers-finishing-a-development-branch` — PR creation routing
- `superpowers-executing-plans` — plan view routing
- `superpowers-receiving-code-review` — inline feedback routing
- `superpowers-subagent-driven-development`, `superpowers-dispatching-parallel-agents`, `superpowers-requesting-code-review`, `superpowers-brainstorming`, `superpowers-writing-plans` — subagent dispatch always uses `task`; the `openchamber` tool (when available) is only for user-facing parallel sessions.
