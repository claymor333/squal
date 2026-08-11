---
name: superpowers-systematic-debugging
description: Use when encountering any bug, test failure, or unexpected behavior, before proposing fixes
---

# Systematic Debugging

## Overview

Random fixes waste time and create new bugs. Quick patches mask underlying issues.

**Core principle:** ALWAYS find root cause before attempting fixes. Symptom fixes are failure.

## The Iron Law

```
NO FIXES WITHOUT ROOT CAUSE INVESTIGATION FIRST
```

If you haven't completed Phase 1, you cannot propose fixes.

## When to Use

Use for ANY technical issue:
- Test failures
- Bugs in production
- Unexpected behavior
- Performance problems
- Build failures
- Integration issues

**Use this ESPECIALLY when:**
- Under time pressure
- "Just one quick fix" seems obvious
- You've already tried multiple fixes
- You don't fully understand the issue

## The Four Phases

### Phase 1: Root Cause Investigation

**BEFORE attempting ANY fix:**

1. **Read Error Messages Carefully** - Don't skip past errors. Read stack traces completely.
2. **Reproduce Consistently** - Can you trigger it reliably? What are the exact steps?
3. **Check Recent Changes** - What changed that could cause this? Git diff, recent commits.
4. **Gather Evidence in Multi-Component Systems** - Add diagnostic instrumentation at each component boundary.
5. **Trace Data Flow** - Where does bad value originate? Keep tracing up until you find the source.

**Parallel Evidence Gathering (multi-component systems only):**
- When the bug spans several independent components, dispatch parallel `task` investigators — each traces a different code path, data source, or component boundary and returns its evidence.
- **Cap:** one investigator per independent component, never more than 4. A single-component bug gets no fan-out — investigate inline.
- Each investigator gets: the symptom, the component/path to trace, and a required evidence format.
- Converge the evidence BEFORE forming hypotheses.
- Never parallelize hypothesis testing — that is shotgun debugging and violates the scientific method in Phase 3.

### Phase 2: Pattern Analysis

**Find the pattern before fixing:**

1. **Find Working Examples** - Locate similar working code in same codebase
2. **Compare Against References** - Read reference implementation COMPLETELY
3. **Identify Differences** - What's different between working and broken?
4. **Understand Dependencies** - What other components does this need?

### Phase 3: Hypothesis and Testing

**Scientific method:**

1. **Form Single Hypothesis** - State clearly: "I think X is the root cause because Y"
2. **Test Minimally** - Make the SMALLEST possible change to test hypothesis
3. **Verify Before Continuing** - Did it work? Yes → Phase 4. Didn't work? Form NEW hypothesis.
4. **When You Don't Know** - Say "I don't understand X". Don't pretend.

### Phase 4: Implementation

**Fix the root cause, not the symptom:**

1. **Create Failing Test Case** - Simplest possible reproduction. MUST have before fixing.
2. **Implement Single Fix** - Address the root cause identified. ONE change at a time.
3. **Verify Fix** - Test passes now? No other tests broken?
4. **If Fix Doesn't Work** - STOP. If < 3 attempts: return to Phase 1. If ≥ 3: question the architecture.

## Red Flags - STOP and Follow Process

If you catch yourself thinking:
- "Quick fix for now, investigate later"
- "Just try changing X and see if it works"
- "Add multiple changes, run tests"
- "It's probably X, let me fix that"
- "I don't fully understand but this might work"
- "One more fix attempt" (when already tried 2+)

**ALL of these mean: STOP. Return to Phase 1.**

## Quick Reference

| Phase | Key Activities | Success Criteria |
|-------|---------------|------------------|
| **1. Root Cause** | Read errors, reproduce, check changes, gather evidence | Understand WHAT and WHY |
| **2. Pattern** | Find working examples, compare | Identify differences |
| **3. Hypothesis** | Form theory, test minimally | Confirmed or new hypothesis |
| **4. Implementation** | Create test, fix, verify | Bug resolved, tests pass |

## Related skills

- **superpowers-test-driven-development** - For creating failing test case (Phase 4, Step 1)
- **superpowers-verification-before-completion** - Verify fix worked before claiming success
