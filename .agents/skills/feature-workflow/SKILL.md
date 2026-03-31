# Skill: feature-workflow

Complete feature lifecycle for glpictl-ai: code → commit → push → PR → merge.

## When to Use

- After completing ANY feature, fix, or meaningful change
- When the user says "commit", "push", "PR", "merge", "cerrar feature"
- At the end of a work session with uncommitted changes

## The Flow

```
1. Analyze changes (git diff, git status)
2. Create feature branch (if on main)
3. Stage and commit with conventional commit message
4. Push branch to remote
5. Create PR with detailed description
6. Merge PR with explanatory message
7. Clean up (switch to main, delete branch)
```

## Step-by-Step

### 1. Analyze Changes

```bash
git status --porcelain
git diff --staged  # if files staged
git diff           # if nothing staged
git log --oneline -5  # recent commits for context
```

### 2. Create Branch (only if on main)

```bash
# Branch naming: type/description-in-kebab-case
git checkout -b feat/my-feature
git checkout -b fix/bug-description
git checkout -b docs/topic
```

**Branch name MUST match:** `^(feat|fix|chore|docs|style|refactor|perf|test)/[a-z0-9._-]+$`

### 3. Stage and Commit

```bash
# Stage relevant files (NOT secrets, NOT .env)
git add path/to/files

# Commit with conventional format
git commit -m "$(cat <<'EOF'
feat(scope): short description under 72 chars

Longer explanation of what changed and why.
Focus on the "why", not the "what" (the diff shows the what).

Closes #N  # if issue exists
EOF
)"
```

**Commit types:** `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`

**NEVER add Co-Authored-By or AI attribution.**

### 4. Push

```bash
git push -u origin HEAD
```

### 5. Create PR

```bash
gh pr create --title "feat(scope): description" --body "$(cat <<'EOF'
## What

Short summary of what this PR does (1-3 bullet points).

## Why

Why was this change needed? What problem does it solve?

## Changes

| File | What changed |
|------|-------------|
| `path/to/file.py` | Description of change |

## How to test

```bash
# Commands to verify the feature works
glpictl-ai --help
glpictl-ai ping
```

## Notes

Any gotchas, tradeoffs, or follow-ups.
EOF
)"
```

### 6. Merge PR

```bash
# Get the PR number
PR_NUMBER=$(gh pr list --head $(git branch --show-current) --json number -q '.[0].number')

# Merge with explanatory message
gh pr merge $PR_NUMBER --merge --body "Merged: full feature description explaining what was built and why"
```

### 7. Clean Up

```bash
git checkout main
git pull origin main
git branch -d feat/my-feature
```

## Quick Reference

```bash
# Full flow in one shot (after code is done):
git checkout -b feat/my-feature
git add -A
git commit -m "feat(cli): add search command for computers"
git push -u origin HEAD
gh pr create --title "feat(cli): add search command for computers" --body "..."
PR=$(gh pr list --head feat/my-feature --json number -q '.[0].number')
gh pr merge $PR --merge --body "Feature: Computer search command with filters"
git checkout main && git pull && git branch -d feat/my-feature
```

## Commit Message Examples (for this project)

```
feat(cli): add search command for computers
feat(client): implement GLPI session management
feat(models): add Computer and Printer Pydantic models
fix(client): handle 401 session expiry with auto-refresh
docs(readme): add installation and configuration guide
test(client): add unit tests for search criteria builder
chore(deps): add httpx and pydantic dependencies
refactor(config): extract env var loading to separate module
```
