---
name: superpowers-subagent-driven-development
description: Use when executing implementation plans with independent tasks in the current session
---

# Subagent-Driven Development

Execute plan by dispatching a fresh implementer subagent per task, a task review (spec compliance + code quality) after each, and a broad whole-branch review at the end.

**Why subagents:** You delegate tasks to specialized agents with isolated context. They should never inherit your session's context or history — you construct exactly what they need.

**Core principle:** Fresh subagent per task + task review (spec + quality) + broad final review = high quality, fast iteration

**Continuous execution:** Do not pause to check in with your human partner between tasks. Execute all tasks from the plan without stopping.

## When to Use

**Use when:**
- Have implementation plan
- Tasks mostly independent
- Stay in this session

**vs. Executing Plans (parallel session):**
- Same session (no context switch)
- Fresh subagent per task (no context pollution)
- Review after each task
- Faster iteration

## The Process

1. Read plan, note context and global constraints, create todos
2. For each task:
   - Dispatch implementer subagent (see routing below)
   - Implementer implements, tests, commits, self-reviews
   - Dispatch task reviewer subagent (see routing below)
   - If issues found: dispatch fix subagent, re-review
   - Mark task complete
3. After all tasks: dispatch final code reviewer
4. Use superpowers-finishing-a-development-branch

**Dispatch routing:**

| Harness | Implementer | Reviewer |
|---------|-------------|----------|
| `openchamber` | `task` tool (parallel) | `task` tool |
| `cli` | `task` tool with `subagent_type: "general"` | `task` tool with `subagent_type: "general"` |

The `openchamber` tool (when available) is for user-facing parallel worktree sessions the human follows up on — never for reviewer delegation, which always uses `task`.

## Parallel Execution

Parallelize only tasks whose file scopes cannot collide. Read each task's **Files** list in the plan first; any two tasks sharing a path must run sequentially or be isolated. When the plan lacks `[PARALLEL]` markers, verify safety yourself: run `tokensave` impact analysis on each task's file set and confirm the transitive dependency closures are disjoint (shared services, models, and affected tests count as coupling).

**Method 1 — Parallel `task` (same workspace):** only when file ownership is disjoint.
- Verify no two tasks share a file, a transitive dependency, or an affected test.
- Constrain each subagent: "Touch only the files listed in your task. Change nothing else."
- Batch-dispatch in one response; review each result; run the full suite after.
- Watch shared resources: migrations, lockfiles, composer.json/package.json, shared services.

**Method 2 — Worktree sessions (`openchamber` tool):** for tasks that would collide, or when the user wants parallel isolation.
- Offer `session.create` with `worktree` + `branch` per task; the user follows up in OpenChamber and integrates branches in the Git view.
- Async and user-facing: results do not return to this session.

**Method 3 — `git worktree` + `task` (CLI isolation):** isolation with results coming back.
- Create a worktree per task, pass the worktree path in the subagent prompt, collect results, merge branches at the end.

**Rule:** disjoint closures → Method 1. Colliding tasks needing isolation → Methods 2 or 3. Sequential otherwise.

## Handling Implementer Status

**DONE:** Generate review package, dispatch task reviewer.

**DONE_WITH_CONCERNS:** Read concerns before proceeding. Address correctness/scope concerns before review.

**NEEDS_CONTEXT:** Provide missing context and re-dispatch.

**BLOCKED:** Assess the blocker:
1. Context problem → provide more context
2. Needs more reasoning → re-dispatch with more capable model
3. Too large → break into smaller pieces
4. Plan is wrong → escalate to human

## Model Selection

Use the least powerful model that can handle each role:
- **Mechanical implementation** (isolated functions, clear specs): cheap model
- **Integration tasks** (multi-file coordination): standard model
- **Architecture and design tasks**: most capable model
- **Review tasks**: scale to diff complexity

## Durable Progress

Track progress in a ledger file, not only in todos:
- Check for ledger at `.superpowers/sdd/progress.md`
- Append one line per completed task
- After compaction, trust the ledger and `git log`

## Red Flags

**Never:**
- Start implementation on main/master without explicit user consent
- Skip task review
- Proceed with unfixed issues
- Dispatch parallel implementers in the same workspace unless their file scopes are provably disjoint
- Make a subagent read the whole plan file

**If reviewer finds issues:**
- Implementer fixes them
- Reviewer reviews again
- Repeat until approved

## Integration

**Required workflow skills:**
- **superpowers-using-git-worktrees** - Isolated workspace
- **superpowers-writing-plans** - Creates the plan
- **superpowers-requesting-code-review** - Final whole-branch review
- **superpowers-finishing-a-development-branch** - Complete development

**Subagents should use:**
- **superpowers-test-driven-development** - TDD for each task
