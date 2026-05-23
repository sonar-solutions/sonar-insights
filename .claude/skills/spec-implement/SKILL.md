---
name: spec-implement
description: Implements a feature from a spec file. Creates a branch, writes code and tests, validates, and opens a PR. Use when a spec is approved and ready to build.
user-invocable: true
argument-hint: spec-file-path
---

You are implementing a feature for the sonar-insights CLI project. You have a spec to follow and must produce production-quality Go code that passes all validation gates before opening a PR.

## Phase 1 — Load the spec

1. If `$ARGUMENTS` is provided, read that file.
2. Otherwise, check if a spec file is open in the IDE (look for context from the conversation). If so, confirm with the user before proceeding.
3. Otherwise, list `specs/` and ask the user which spec to implement.

Read the full spec carefully. If it references an external implementation (e.g. a PowerShell script), fetch and read it — it is an authoritative behavioural reference.

## Phase 2 — Understand the existing codebase

Before writing any code, read the relevant parts of the codebase:
- `cmd/` — understand the CLI structure and how targets are wired up
- `internal/collector/` and `internal/analyzer/` — understand existing patterns
- `go.mod` — confirm available dependencies
- Any existing files that will be modified by this implementation

Do not guess at conventions — read the code.

## Phase 3 — Plan

Briefly summarise your implementation plan to the user (which files you will create or modify, what the key design decisions are). Wait for confirmation before writing any code.

## Phase 4 — Branch

Before writing code:
1. Check that the working tree is clean (`git status`). If there are uncommitted changes, stop and tell the user.
2. Check out `main` and pull to ensure it is up to date. If there are problems (conflicts, diverged), stop and report them.
3. Create a new branch with a descriptive name matching the spec (e.g. `feat/collect-bgtasks`).

## Phase 5 — Implement

Write the implementation following these project rules:

### Code quality
- Follow Go best practices and idiomatic Go patterns.
- Keep the code maintainable, testable, and expandable.
- Do not introduce new external dependencies without asking. Approved dependencies: `github.com/spf13/cobra`, `github.com/lfrystak/rptgen`.
- Write no comments unless the WHY is non-obvious. No docstring blocks.

### Testing
- Write tests for all non-trivial logic.
- Tests must be real — no mocking that would hide integration issues.

### CLI conventions
- New flags must have descriptive names, explicit defaults, and env var fallbacks where appropriate (following the pattern in `cmd/collect.go`).
- Flags that apply across multiple targets belong on the parent command, not target-specific code.

### Logging
- Use `slog` at INFO level for operation start/end messages.
- Use DEBUG level for progress details (page counts, version info).
- Errors must be wrapped with context using `fmt.Errorf("context: %w", err)`.

### Output
- Write raw API responses to disk without transformation unless the spec says otherwise.
- Follow the output path conventions in the spec exactly.
- Clear target-specific output directories before writing, as specified.

## Phase 6 — Validate

Run all three gates in order. Fix any failures before proceeding.

```bash
go fmt ./...
golangci-lint run
go test ./...
```

Do not skip or bypass any gate. If a gate fails and you cannot fix it, stop and explain the problem to the user.

## Phase 7 — Commit

Stage only the files relevant to this implementation (never `git add .` blindly).

Write a commit message with:
- **Subject line**: short imperative summary (e.g. `feat: implement bgtasks collector`)
- **Body**: what changed and why — key decisions, any deviations from the spec and the reason

Format:
```
<subject>

<body paragraph(s)>

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>
```

Never commit directly to `main`.

## Phase 8 — Pull request

Open a PR to `main` using `gh pr create`.

The PR description must include:
- What this implements (link the spec file)
- A summary of the key changes (what files were added/modified and why)
- Any deviations from the spec and the reason
- Testing notes (what was tested and how)

## Behaviour rules

- Never commit to `main`.
- Never skip `go fmt`, `golangci-lint`, or `go test`. All three must pass.
- Never use `--no-verify` or bypass hooks.
- If the spec is ambiguous on a point, make the conservative choice and leave a `// TODO:` comment with the question — do not silently assume.
- If something in the spec contradicts the reference implementation, stop and ask the user for clarification before proceeding.
- Prefer editing existing files over creating new ones unless the spec clearly calls for a new file.
