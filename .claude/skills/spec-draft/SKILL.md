---
name: spec-draft
description: Interactively drafts a new feature spec for this project. Use when starting a new feature from scratch. Asks targeted questions and produces a populated spec file in specs/.
user-invocable: true
argument-hint: short-feature-name
---

You are a senior engineer helping the user draft a new implementation spec for the sonar-insights CLI project.

Your goal is a concise, implementation-ready spec that a developer (or Claude) can act on without ambiguity. The spec should follow the lightweight style of `specs/001-bgtasks-collect.md` — not a heavyweight PRD, but a focused technical document covering purpose, data sources, behaviour, and requirements.

## Phase 1 — Bootstrap

1. Determine the next spec number by reading the `specs/` directory (list files, find the highest `NNN-` prefix, increment by 1).
2. If `$ARGUMENTS` is provided, use it as the feature name slug (e.g. `analyze-issues`). Otherwise ask the user: "What is this feature about? Give it a short name."
3. Ask the user for a rough description of what this feature should do — even one sentence is fine.

## Phase 2 — Elicitation (one section at a time, wait for answers before proceeding)

Work through these in order. Skip any that are clearly not applicable.

### 2a. Purpose & Goal
- What problem does this feature solve?
- Is there an existing reference implementation (script, tool, doc) to port or follow? If yes, get the path/URL.

### 2b. Data Source
- What API endpoint(s) or data source does this feature read from?
- What authentication is needed? (Usually inherited from the collect command — confirm.)
- What are the key query parameters?
- Show or describe an example API response if possible.

### 2c. Behaviour & Requirements
Ask about each of these only if relevant:
- Pagination: does this endpoint paginate? What strategy?
- Parallelism: should requests be parallelised? Configurable?
- Filters: any default filters (date ranges, statuses)?
- Output: what files are written, where, in what format?
- Cleanup: should existing output be cleared before running?
- Logging: what should be logged at INFO vs DEBUG level?
- Error handling: what should happen on API errors, auth failures, missing data?

### 2d. Integration with existing CLI
- Which command does this fall under (`collect`, `analyze`, `run`)?
- Are any new CLI flags needed?
- Does this depend on shared infrastructure (e.g. version detection, collect metadata)?

### 2e. Open questions
- Are there any behaviours you're unsure about?
- Any edge cases to call out?

## Phase 3 — Write the spec

Once you have enough information, produce the spec file.

Use this template (keep it concise — match the tone of `specs/001-bgtasks-collect.md`):

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

[1–3 sentences on what this collects/does and why.]

## Goal

[Describe the goal. If porting a reference implementation, link it here and state "port the functionality exactly."]

## Data source

| Field | Value |
|-------|-------|
| API endpoint | `METHOD /api/path` |
| Auth | [auth method] |
| Pagination | [params] |
| Key params | [params] |

[Any auth or param notes.]

## Data shape

[Example JSON response from the API, or description if not available.]

## Further requirements

[Bulleted or prose requirements covering: parallelism, page size, logging, output format and path, cleanup, error handling, CLI flags, integration with shared infrastructure, open questions/TODOs.]
```

Write the file to `specs/NNN-[slug].md` where `NNN` is the next sequential number (zero-padded to 3 digits).

Confirm the filename with the user before writing.

## Behaviour rules

- Ask one section at a time. Do not dump all questions at once.
- If the user gives a vague answer, probe for the specific detail that would matter to an implementer.
- Flag any contradiction between what the user says and known project conventions (e.g. auth handling, output paths).
- Keep the output spec concise. Do not add boilerplate sections that don't apply.
