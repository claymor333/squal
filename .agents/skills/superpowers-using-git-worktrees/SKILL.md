---
name: superpowers-using-git-worktrees
description: Use when starting feature work that needs isolation from current workspace or before executing implementation plans
---

# Using Git Worktrees

## Overview

Ensure work happens in an isolated workspace. Prefer your platform's native worktree tools. Fall back to manual git worktrees only when no native tool is available.

**Core principle:** Detect existing isolation first. Then use native tools. Then fall back to git.

**Announce at start:** "I'm using the using-git-worktrees skill to set up an isolated workspace."

## Step 0: Detect Existing Isolation

**Before creating anything, check if you are already in an isolated workspace.**

```bash
GIT_DIR=$(cd "$(git rev-parse --git-dir)" 2>/dev/null && pwd -P)
GIT_COMMON=$(cd "$(git rev-parse --git-common-dir)" 2>/dev/null && pwd -P)
BRANCH=$(git branch --show-current)
```

**If `GIT_DIR != GIT_COMMON` (and not a submodule):** You are already in a linked worktree. Skip to Step 2.

**If `GIT_DIR == GIT_COMMON`:** You are in a normal repo checkout. Ask for consent before creating a worktree.

## Step 1: Create Isolated Workspace

### 1a. Native Worktree Tools (preferred)

Use your platform's native worktree tool if available.

**Worktree routing:**

| Harness | Method |
|---------|--------|
| `openchamber` | OpenChamber worktree UI (Session → New Worktree) |
| `cli` | Git worktree CLI fallback (Step 1b) |

**OpenChamber mode:** Use OpenChamber's worktree UI to create an isolated workspace. The UI handles branch creation, directory setup, and project detection automatically.

**CLI mode:** Use git worktree CLI fallback (Step 1b).

### 1b. Git Worktree Fallback

```bash
# Check for existing worktree directory
ls -d .worktrees 2>/dev/null || ls -d worktrees 2>/dev/null

# If neither exists, default to .worktrees/
# Verify directory is ignored
git check-ignore -q .worktrees

# Create the worktree
git worktree add "$path" -b "$BRANCH_NAME"
cd "$path"
```

## Step 2: Project Setup

Auto-detect and run appropriate setup:

```bash
# Node.js
if [ -f package.json ]; then npm install; fi

# PHP/Composer
if [ -f composer.json ]; then composer install; fi

# Python
if [ -f requirements.txt ]; then pip install -r requirements.txt; fi

# Go
if [ -f go.mod ]; then go mod download; fi
```

## Step 3: Verify Clean Baseline

Run tests to ensure workspace starts clean.

**If tests fail:** Report failures, ask whether to proceed or investigate.

**If tests pass:** Report ready.

## Quick Reference

| Situation | Action |
|-----------|--------|
| Already in linked worktree | Skip creation (Step 0) |
| Native worktree tool available | Use it (Step 1a) |
| No native tool | Git worktree fallback (Step 1b) |
| Directory not ignored | Add to .gitignore + commit |
| Tests fail during baseline | Report failures + ask |
