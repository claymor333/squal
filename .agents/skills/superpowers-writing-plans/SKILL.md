---
name: superpowers-writing-plans
description: Use when you have a spec or requirements for a multi-step task, before touching code
---

# Writing Plans

## Overview

Write comprehensive implementation plans assuming the engineer has zero context for our codebase and questionable taste. Document everything they need to know: which files to touch for each task, code, testing, docs they might need to check, how to test it.

**Announce at start:** "I'm using the writing-plans skill to create the implementation plan."

**Save plans to:** `docs/superpowers/plans/YYYY-MM-DD-<feature-name>.md`

## Scope Check

If the spec covers multiple independent subsystems, suggest breaking this into separate plans — one per subsystem.

## File Structure

Before defining tasks, map out which files will be created or modified and what each one is responsible for.

- Design units with clear boundaries and well-defined interfaces
- Prefer smaller, focused files over large ones that do too much
- Files that change together should live together
- In existing codebases, follow established patterns

**Parallel research (multi-subsystem plans only):**
- Before mapping tasks, if the plan spans 3+ subsystems, dispatch parallel `task` explorers — one per subsystem, never more than 4 — to map current structure and ownership.
- Their output feeds both the file mapping and the impact-closure analysis.

## Bite-Sized Task Granularity

**Each step is one action (2-5 minutes):**
- "Write the failing test" - step
- "Run it to make sure it fails" - step
- "Implement the minimal code to make the test pass" - step
- "Run the tests and make sure they pass" - step
- "Commit" - step

## File Ownership & Parallelization

Design tasks so each owns a disjoint file set. That's what lets subagent-driven-development dispatch tasks in parallel without collisions.

- **Exclusive ownership per task:** every task lists every file it creates or modifies, and no other task touches the same path.
- **Shared files are a red flag:** if two tasks must both modify a file (migration, lockfile, shared service), merge them into one task or sequence them explicitly.
- **Verify with impact analysis:** a literal file list is not enough. Run `tokensave` impact analysis on each task's file set — two tasks are only parallel-safe if their transitive dependency closures (dependents ∪ dependencies ∪ affected tests) are disjoint. Shared services, models, and test suites count as coupling.
- **Mark parallel groups** after listing tasks:
  - `[PARALLEL]` — tasks with disjoint file ownership and disjoint impact closures can run concurrently.
  - `[SEQUENTIAL]` — tasks sharing files, sharing transitive dependencies, or consuming another task's interfaces run in order.
- **Order by interface:** put shared-file and interface-producing tasks early, so later parallel groups consume stable interfaces.

## Plan Document Header

**Every plan MUST start with this header:**

```markdown
# [Feature Name] Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers-subagent-driven-development (recommended) or superpowers-executing-plans to implement this plan task-by-task.

**Goal:** [One sentence describing what this builds]

**Architecture:** [2-3 sentences about approach]

**Tech Stack:** [Key technologies/libraries]

**Parallelization:** [Which task groups run concurrently (disjoint file ownership and impact closures) vs sequentially]

## Global Constraints

[Project-wide requirements]

---
```

## Task Structure

```markdown
### Task N: [Component Name]

**Files:**
- Create: `exact/path/to/file.py`
- Modify: `exact/path/to/existing.py:123-145`
- Test: `tests/exact/path/to/test.py`
- **Exclusive ownership:** Yes — no other task modifies these files or their transitive dependencies.

**Interfaces:**
- Consumes: [what this task uses from earlier tasks]
- Produces: [what later tasks rely on]

- [ ] **Step 1: Write the failing test**

```python
def test_specific_behavior():
    result = function(input)
    assert result == expected
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pytest tests/path/test.py::test_name -v`
Expected: FAIL with "function not defined"

- [ ] **Step 3: Write minimal implementation**

```python
def function(input):
    return expected
```

- [ ] **Step 4: Run test to verify it passes**

Run: `pytest tests/path/test.py::test_name -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add tests/path/test.py src/path/file.py
git commit -m "feat: add specific feature"
```
```

## No Placeholders

Every step must contain the actual content an engineer needs. These are **plan failures**:
- "TBD", "TODO", "implement later"
- "Add appropriate error handling"
- "Write tests for the above" (without actual test code)
- "Similar to Task N" (repeat the code)
- Steps that describe what to do without showing how

## Remember
- Exact file paths always
- Complete code in every step
- Exact commands with expected output
- DRY, YAGNI, TDD, frequent commits

## Self-Review

After writing the complete plan:
1. **Spec coverage:** Can you point to a task for each requirement?
2. **Placeholder scan:** Search for red flags. Fix them.
3. **Type consistency:** Do types/signatures match across tasks?
4. **Parallel-safety:** Run impact analysis on the task file sets; confirm no two `[PARALLEL]` tasks share a file, transitive dependency, or affected test.

**Agent Review:**
Dispatch reviewer subagents via `task` in parallel — one per review dimension. Reviewers are read-only and dimension-disjoint, so parallel review is always safe.

- Model: standard model
- Input: The written plan file (same input to every reviewer)

Dimensions (one `task` reviewer each):
1. **Spec coverage** — Does every requirement have a corresponding task?
2. **Placeholder scan** — Any TBD, TODO, or missing code?
3. **Type consistency** — Do types/signatures match across tasks?
4. **Step completeness** — Every step has exact file paths, code, and commands?
5. **File ownership** — Does every task own a disjoint file set AND a disjoint impact closure? Verify with tokensave: shared transitive dependencies, models, or affected tests between task file sets are coupling. Are shared files merged or sequenced? Is the [PARALLEL]/[SEQUENTIAL] grouping correct?

Each reviewer gets the same response format:

```
You are a plan reviewer. Review the implementation plan for: <DIMENSION>.
Respond with exactly one of:
- APPROVED — no issues found
- ISSUES_FOUND — list specific items to fix

Be terse. No preamble. No compliments.
```

**Merge rule:** any reviewer returning `ISSUES_FOUND` wins — fix the union of findings, then re-review (parallel again). Only if all return `APPROVED` proceed to execution handoff. Dedupe overlapping findings across dimensions.

## Execution Handoff

After saving the plan, offer execution choice:

**1. Subagent-Driven (recommended)** - Fresh subagent per task, review between tasks

**2. Inline Execution** - Execute tasks in this session with checkpoints
