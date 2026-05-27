---
name: ship
description: Validates, commits, pushes to both remotes, and opens PRs on both origin and personal. Use when code is ready to ship — after implementation is complete and you want to commit + push + open PRs.
user-invocable: true
argument-hint: "optional commit subject (e.g. 'feat: add issue collector')"
---

You are shipping code for the sonar-insights CLI project. Your job is to validate, commit, push to both remotes, and open pull requests on both.

## Phase 1 — Preflight

1. Detect whether you are inside a git worktree by running `git rev-parse --git-dir`. A result ending in `/worktrees/<name>` (or `\.worktrees\<name>` on Windows) means you are in a linked worktree — the branch is already set, no new branch is needed. Otherwise confirm the working tree is on a feature branch (not `main`); if on `main`, stop and tell the user.
2. Run `git status` to identify modified files. If the tree is clean, stop and tell the user there is nothing to commit.
3. Confirm with the user which files should be staged if it is not obvious (i.e. unrelated files are modified).

## Phase 2 — Validate

Run all gates in order:

```bash
go fmt ./...
go build -v ./...
go test -v ./... -coverprofile=coverage.out
```

Then stage the changes that will be committed:

```bash
git add <relevant files>
```

Then run the SonarQube analysis on staged files and the linter:

```bash
sonar analyze agentic --staged
golangci-lint run
```

**If any gate fails:** fix the issues, then restart Phase 2 from the top — re-run every gate in sequence. Do not continue to commit or push until the entire validation suite passes cleanly in a single run. Do not skip or bypass any gate. Do not use `--no-verify`. If a gate fails and you cannot fix it, stop and explain the problem to the user before asking how to proceed.

## Phase 3 — Commit

Write a commit message with:
- **Subject line**: short imperative summary (e.g. `feat: implement bgtasks collector`)
- **Body**: what changed and why — key decisions, context for reviewers

Format:
```
<subject>

<body paragraph(s)>
```

If `$ARGUMENTS` is provided, use it as the subject line. Otherwise derive it from the staged changes.

Commit the files staged in Phase 2 — do not re-stage.

## Phase 4 — Push to both remotes

Push the branch to both remotes:

```bash
git push origin <branch>
git push personal <branch>
```

If either push fails (e.g. no upstream set), use `-u` to set the upstream on first push.

## Phase 5 — Open pull requests

Open a PR on both remotes targeting `main`. Use `gh pr create` for `origin` and derive the `--repo` flag for `personal` from `git remote get-url personal`.

For each PR, the description must include:
- What this change implements or fixes (link to spec file if applicable)
- A summary of key changes (what files were added/modified and why)
- Any notable decisions or deviations
- Testing notes (what was tested and how)

**origin PR** — derive owner/repo from `git remote get-url origin`:
```bash
gh pr create --repo <origin-owner/repo> --head <origin-owner>:<branch> --base main --title "<subject>" --body "<description>"
```

**personal PR** — derive owner/repo from `git remote get-url personal`:
```bash
gh pr create --repo <personal-owner/repo> --head <personal-owner>:<branch> --base main --title "<subject>" --body "<description>"
```

Both PRs should have identical titles and descriptions.

## Behaviour rules

- Never commit to `main`.
- Never skip any validation gate.
- Always push to both `origin` and `personal`.
- Always open PRs on both remotes.
- If `personal` remote does not exist, warn the user but still complete the `origin` push and PR.
