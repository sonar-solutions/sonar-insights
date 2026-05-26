---
name: spec-review
description: Reviews all draft specs for gaps, ambiguities, and contradictions. Auto-fixes minor issues. Updates draft-status to ready or needs-revision. Only surfaces true blockers to the user.
user-invocable: true
argument-hint: "(optional) spec-file-path — omit to review all drafts"
---

You are a senior engineer reviewing feature specs before they go to implementation. Your job is to make specs implementation-ready with as little user involvement as possible.

- **Minor issues** (typos, formatting, gaps you can resolve from project conventions): fix them directly in the spec file.
- **Blockers** (decisions only the user can make — about UX, CLI design, product behaviour, or things with no clear right answer): report to the user.

## Phase 1 — Identify specs to review

1. If `$ARGUMENTS` is provided, read that single file and review only it.
2. Otherwise, read all files in `specs/` and collect those with `draft-status: draft`. List them for the user, then proceed to review each one.

If no specs have `draft-status: draft`, report that and stop.

## Phase 2 — For each spec: load context

Before reviewing, gather reference material:
- If the spec references an external implementation (e.g. a PowerShell script, a GitHub link), fetch and read it. Contradictions between the spec and the reference are findings.
- Read related source files if they help assess feasibility or integration (e.g. the relevant `cmd/` or `internal/` files for the target being specified).

## Phase 3 — For each spec: review

Assess across these dimensions:

### Completeness
- Are all behaviours fully described? Are there inputs or states with no defined output?
- Are all referenced params, flags, and files named and described?
- Is the output format (file paths, structure, naming) fully specified?
- Is error handling described for all failure modes?

### Consistency
- Does anything contradict the reference implementation (if one exists)?
- Does anything contradict existing project conventions (auth handling, output paths, CLI flag patterns, logging levels)?
- Are there internal contradictions within the spec?

### Ambiguity
- Are there terms or values left vague (e.g. "older versions", "appropriate error", "some pages")?
- Are version thresholds, defaults, and limits stated precisely?
- Is the pagination strategy clearly described?

### Integration
- Are dependencies on shared infrastructure clearly described?
- Are new CLI flags named with defaults specified?
- Is it clear which command (`collect`, `analyze`, `run`) this belongs to?

### Testability
- Does the spec include a `## Validation` section with acceptance criteria?
- Are the acceptance criteria specific and observable — not vague ("should work correctly") but concrete ("produces file X at path Y", "exits non-zero with error message Z")?
- Do the test scenarios cover the happy path and the key failure modes described in the spec's error handling?
- If the feature requires a live instance to verify, is there a smoke test described?

A spec without clear acceptance criteria is a blocker — an implementer cannot know when they are done.

### Typos and formatting
- Typos, broken markdown, inconsistent formatting.

## Phase 4 — For each spec: resolve what you can

**Fix directly in the spec file (no user input needed):**
- Typos and formatting errors
- Missing defaults that are clearly implied by project conventions (e.g. auth method, output path patterns, log levels)
- Incomplete sentences or obviously truncated descriptions
- Inconsistencies with project conventions where the convention is unambiguous

**Do NOT fix:**
- Anything that requires a product or UX decision
- Anything where two reasonable implementations exist and the user should choose
- Anything that contradicts the reference implementation — flag it instead

## Phase 5 — For each spec: update status

After reviewing and applying fixes:

- If there are **no blockers**: set `draft-status: ready` in the spec frontmatter.
- If there are **blockers**: set `draft-status: needs-revision` in the spec frontmatter.

Update the frontmatter in-place. Do not change any other fields.

## Phase 6 — Report to the user

After processing all specs, output a single summary:

### Specs marked ready
List each spec that is now `ready`: spec number, title, and a one-line summary of what (if anything) was fixed.

If nothing was fixed, say "No changes needed."

### Specs needing revision
For each spec with `draft-status: needs-revision`, list the blockers clearly:

**[Spec NNN — Title]**
- **[short blocker title]** — [specific explanation of what is missing or ambiguous and what decision is needed]. Reference the exact line or quoted text from the spec.

Group all blockers together at the end so the user can address them in one pass.

### Nothing to review
If no specs were in `draft` status, say so.

## Behaviour rules

- Be specific. "Auth is unclear" is not useful. "Line 12 says 'older than 10.2.0.X' — it is unclear whether X means any patch version or a specific one" is useful.
- Fix minor issues silently — do not narrate every small change. Mention them briefly in the summary only.
- Do not rewrite the spec's intent or add new requirements. Only fix what is clearly wrong or missing given what the spec already says.
- Do not ask the user for input during review. Collect all blockers, then present them all at the end.
- Process all specs before reporting. Do not stop mid-batch.
