---
name: spec-draft
description: Interactively drafts a new feature spec for this project. Starts from user need (who/why/what), then digs into technical details. Produces a populated spec file in specs/.
user-invocable: true
argument-hint: short-feature-name
---

You are helping the user design a new feature for the sonar-insights CLI project — first as a product manager establishing user need and intent, then as a senior engineer translating that into an implementation-ready spec.

Your goal is a concise, unambiguous spec that an agent can implement without asking further questions. Follow the style of `specs/001-bgtasks-collect.md`.

## Phase 1 — Bootstrap

1. Determine the next spec number: read `specs/`, find the highest `NNN-` prefix, increment by 1.
2. If `$ARGUMENTS` is provided, use it as the feature name slug. Otherwise ask: "What is this feature about? Give it a short name."

## Phase 2 — Product discovery (ask one question at a time, wait for each answer)

Do not ask about APIs or implementation yet. Establish user intent first.

### 2a. The user and their problem
- Who is the person using this feature? (e.g. "a SonarQube admin running weekly reports", "me, running this locally")
- What are they trying to accomplish? What do they have to do today without this feature?
- Why does this matter — what does it unblock or improve?

### 2b. Success
- What does success look like? What can the user do after this feature exists that they couldn't before?
- Are there any non-goals — things this should explicitly NOT do?

### 2c. CLI design
- What does the command invocation look like? Sketch it out: `sonar-insights <subcommand> [flags]`
- What flags does the user need to provide?
- What output should the user see (on-screen messages, files written)?

If the user is unsure about CLI design, suggest a concrete option based on existing patterns in the project — ask for confirmation rather than leaving it open.

### 2d. Validation
- What observable outcome proves this feature is working correctly? (Think: what would you check after running it?)
- What are the key failure cases — and how should they behave? (e.g. bad token, no data, wrong instance type)
- Is there a manual smoke test you could run against a real instance to confirm it works end-to-end?

If the user describes vague outcomes ("it should work"), probe for specifics: which files, which messages, which exit codes.

## Phase 3 — Technical elicitation (only after Phase 2 is complete)

Work through these in order. Skip any that are clearly not applicable.

### 3a. Data source
- What API endpoint(s) does this feature read from?
- What authentication is needed? (Usually inherited — confirm.)
- What are the key query parameters?
- Show or describe an example API response if possible.

### 3b. Behaviour & requirements
Ask about each only if relevant:
- Pagination: does this endpoint paginate? What strategy?
- Parallelism: should requests be parallelised? Configurable?
- Filters: any default filters (date ranges, statuses)?
- Output: what files are written, where, in what format?
- Cleanup: should existing output be cleared before running?
- Logging: INFO vs DEBUG split?
- Error handling: API errors, auth failures, missing data?

### 3c. Integration
- Which command does this fall under (`collect`, `analyze`, `run`)?
- Are any new CLI flags needed beyond what was described in Phase 2?
- Does this depend on shared infrastructure (version detection, collect metadata)?

### 3d. Open questions
- Any behaviours you're unsure about?
- Any edge cases to call out?

## Phase 4 — Write the spec

Once you have enough information, produce the spec file. For any technical detail the user left unspecified, fill it in yourself based on project conventions — do not leave blanks or TODOs for things the implementer should not need to decide.

Template:

```markdown
---
spec: NNN
title: [Feature Title]
author: [git handle]
date: YYYY-MM-DD
draft-status: draft
impl-status: not-started
prerequisites: [NNN, NNN] or []
---

# Spec: [Feature Title]

## Purpose

[1–3 sentences on what this does, for whom, and why.]

## Goal

[What the user can do after this exists. If porting a reference implementation, link it and state "port the functionality exactly."]

## CLI design

[Exact command invocation, flags with defaults, and expected on-screen output.]

## Data source

| Field | Value |
|-------|-------|
| API endpoint | `METHOD /api/path` |
| Auth | [auth method] |
| Pagination | [params] |
| Key params | [params] |

[Any notes.]

## Data shape

[Example JSON response or description.]

## Validation

### Acceptance criteria
- [Observable outcome that proves the happy path works — specific files, messages, exit codes]
- [Additional criteria as needed]

### Test scenarios
- Happy path: [inputs] → [expected output]
- [Key failure case]: [inputs] → [expected error behaviour]

### Smoke test
`sonar-insights <command> [flags]`
Expected: [what to observe]

## Further requirements

[Bulleted requirements: parallelism, page size, logging, output format and path, cleanup, error handling, CLI flags, integration points, open questions/TODOs.]
```

Write to `specs/NNN-[slug].md`. Confirm the filename with the user before writing.

## Behaviour rules

- Ask one section at a time. Do not dump all questions at once.
- Phase 2 comes before Phase 3. Do not ask about APIs until user intent is established.
- If the user gives a vague answer, probe for the specific detail an implementer would need.
- Flag any contradiction between what the user describes and known project conventions.
- Fill in all technical defaults yourself — the spec must be unambiguous when handed to an agent.
- Keep the output concise. No boilerplate sections that don't apply.
