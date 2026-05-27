---
spec: 020
title: Replace `"no project key - error"` sentinel with a neutral placeholder
author: code-review
date: 2026-05-25
draft-status: ready
impl-status: complete
prerequisites: []
---

# IMP: Replace `"no project key - error"` sentinel with a neutral placeholder

## Problem

`internal/analyzer/bgtasks/metrics.go`:

```go
const (
    noProjectKey  = "no project key - error"
    noType        = "no type - error"
    noStatus      = "no status - error"
    noSubmitter   = "no submitter - error"
)
```

These strings are used as fallback values when a task has an empty field.
They flow directly into the rendered HTML report — appearing in pie chart
legends, top-projects tables, and per-day charts. A user opening the
report sees rows like:

| Project                  | Count | Percentage |
|--------------------------|-------|------------|
| my-actual-project        | 488   | 36.36%     |
| no project key - error   | 12    | 0.89%      |

That string is visually startling — it reads like a defect in the report
itself, not like a legitimate "this task didn't have a project key"
classification.

## Why it matters

- The report's value depends on the user trusting it. A label that
  literally says "error" undermines that trust, even when nothing is
  actually wrong.
- The fallback is also useful diagnostic data — knowing tasks with no
  project key is a legitimate finding — so the right answer isn't to hide
  it, just to label it cleanly.

## Proposed fix

Use neutral placeholders that read as data, not warnings:

```go
const (
    noProjectKey = "(no project key)"
    noType       = "(no type)"
    noStatus     = "(no status)"
    noSubmitter  = "(no submitter)"
)
```

If genuine errors need a different track, add a separate `WarningCount`
or `Diagnostics` field to surface them deliberately — but those go in a
diagnostics section of the report, not mixed into the data.

## Validation

- The golden-master tests in `metrics_test.go` check only map sizes (counts
  of distinct types, statuses, submitters) — they do not assert on the
  literal sentinel strings. No test updates are required. Only the four
  constants in `metrics.go` change.
- Confirm with `grep -r "no project key" .` that no other file references
  the old strings.

## Prerequisites

None.
