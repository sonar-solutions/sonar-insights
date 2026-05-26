---
name: spec-implement
description: Implements a feature from a spec file. Creates a worktree and branch, writes code and tests, validates, and opens a PR. Fully autonomous — no confirmation required. Use when a spec has draft-status ready.
user-invocable: true
argument-hint: spec-file-path
---

You are implementing a feature for the sonar-insights CLI project. You have a spec to follow and must produce production-quality Go code that passes all validation gates before opening a PR. Work autonomously from start to finish.

## Phase 1 — Load the spec

1. If `$ARGUMENTS` is provided, read that file.
2. Otherwise, list `specs/` filtered to `draft-status: ready` and `impl-status: not-started`, and pick the highest-priority one (lowest spec number). Announce which spec you are implementing.

Read the full spec carefully. If it references an external implementation (e.g. a PowerShell script), fetch and read it — it is an authoritative behavioural reference.

If the spec's `draft-status` is not `ready`, stop and tell the user: give the spec number, its current status, and what needs to happen before it can be implemented.

## Phase 2 — Understand the existing codebase

Before writing any code, read the relevant parts of the codebase:
- `cmd/` — CLI structure and how targets are wired up
- `internal/collector/` and `internal/analyzer/` — existing patterns
- `go.mod` — available dependencies
- Any existing files this implementation will modify

Do not guess at conventions — read the code.

## Phase 3 — Update spec status

Set `impl-status: in-progress` in the spec frontmatter before writing any code.

## Phase 4 — Branch (in a worktree)

First, detect whether you are already inside a git worktree by running `git rev-parse --git-dir`. A result ending in `/worktrees/<name>` (or `\.worktrees\<name>` on Windows) means you are already in a linked worktree — the branch is set. In that case, skip steps 1–3 and proceed directly to step 4.

If you are **not** already in a worktree:

1. Check that the working tree is clean (`git status`). If there are uncommitted changes, stop and tell the user.
2. Check out `main` and pull to ensure it is up to date. If there are problems, stop and report them.
3. Create a new git worktree at `.claude/worktrees/<branch-name>` on a new branch with a descriptive name matching the spec (e.g. `feat/collect-bgtasks`, `fix/bgtasks-semaphore`, `imp/target-registry`).

4. Do all subsequent work inside the worktree.

## Phase 5 — Implement

Write the implementation following these project rules:

### Code quality
- Follow Go best practices and idiomatic Go patterns.
- Keep the code maintainable, testable, and expandable.
- Do not introduce new external dependencies without asking. Approved: `github.com/spf13/cobra`, `github.com/lfrystak/rptgen`.
- Write no comments unless the WHY is non-obvious. No docstring blocks.

### Testing
- Read the spec's `## Validation` section before writing any tests.
- Write tests that directly cover each acceptance criterion and each test scenario listed there. These are the minimum required test cases — add more for non-trivial internal logic.
- Tests must be real — no mocking that would hide integration issues.

### CLI conventions
- New flags: descriptive names, explicit defaults, env var fallbacks where appropriate (follow pattern in `cmd/collect.go`).
- Flags that apply across multiple targets belong on the parent command, not target-specific code.

### Logging
- `slog` at INFO for operation start/end. DEBUG for progress details.
- Errors wrapped with context: `fmt.Errorf("context: %w", err)`.

### Output
- Write raw API responses to disk without transformation unless the spec says otherwise.
- Follow output path conventions in the spec exactly.
- Clear target-specific output directories before writing, as specified.

### When the spec is ambiguous
If a point is ambiguous despite the spec being marked `ready`, make the most conservative choice consistent with project conventions. Do not stop to ask — decide and proceed. Leave a `// TODO:` comment only if the ambiguity is significant enough that a reviewer should see it.

## Phase 6 — Validate

Run all gates from inside the worktree:

```bash
go fmt ./...
go build -v ./...
go test -v ./... -coverprofile=coverage.out
```

Then stage and lint:

```bash
git add <relevant files>
sonar analyze agentic --staged
golangci-lint run
```

If any gate fails: fix the issues and re-run the full suite. Do not proceed until all gates pass cleanly in one run. Do not skip or bypass any gate.

## Phase 6b — Verify acceptance criteria

Before opening the PR, confirm that every acceptance criterion in the spec's `## Validation` section is covered:

1. Check that each criterion has a corresponding test that would fail if the behaviour were absent.
2. If the spec includes a smoke test and the CLI can be built and run locally, execute it. Record the actual output.
3. If a smoke test requires a live SonarQube instance that is not available, note this explicitly in the PR description — do not silently skip it.

## Phase 7 — Ship

Invoke the `/ship` skill with a subject line derived from the spec (e.g. `feat: implement bgtasks collector`).

The PR description must include:
- What this implements (link the spec file)
- Summary of key changes (files added/modified and why)
- Any deviations from the spec and the reason
- Testing notes (what was tested and how)

## Phase 8 — Update spec status

After the PR is successfully opened:

1. Update the spec file: set `impl-status: complete`.
2. Commit and push this change:

```bash
git add <spec-file>
git commit -m "chore: mark spec <NNN> impl-status complete"
git push origin <branch>
git push personal <branch>
```

## Behaviour rules

- Never commit to `main`.
- Work entirely inside the worktree from Phase 4 onwards.
- Never skip any validation gate.
- Do not ask the user for input during implementation. If something is truly unresolvable without user input, stop, explain precisely what decision is needed and why you cannot proceed, then wait.
