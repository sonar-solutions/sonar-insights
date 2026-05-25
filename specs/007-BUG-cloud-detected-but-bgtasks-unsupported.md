---
spec: 007
title: SonarQube Cloud is detected but `bgtasks` collection silently fails against it
author: code-review
date: 2026-05-25
draft-status: ready
impl-status: not-started
prerequisites: []
---

# BUG: SonarQube Cloud is detected but `bgtasks` collection silently fails against it

## Problem

The `Detect` function returns a `SonarInstance{Product: Cloud}` when the user
points the tool at `sonarcloud.io` / `sonarcloud.us` — but the bgtasks
collector calls `GET /api/ce/activity`, which is a SonarQube **Server** API
and is not exposed on SonarQube Cloud. Today the chain is:

1. User runs `sonar-insights collect bgtasks` against `https://sonarcloud.io`
   (the *default URL* — they may not even realise it).
2. `Detect` returns `Product: Cloud` without error.
3. `CollectBgTasks` runs anyway. The call hits a 404 / unexpected status from
   Cloud and aborts with `"unexpected status 404 from /api/ce/activity"`.

The error message says nothing about why the call is unsupported, and the
fact that the default URL silently points at an unsupported product makes the
failure mode the first thing every new user hits.

Two compounding issues:

1. **No early guard.** Targets that only support Server should refuse to
   start when `instance.Product == Cloud`, before any HTTP traffic.
2. **The default URL `https://sonarcloud.io` is misleading** when the only
   currently-supported product is Server. Either the default should be empty
   (with a clear "no URL provided" error) or the help text must make the
   limitation explicit.

## Why it matters

- First-run UX: users running `sonar-insights collect` with no args get an
  opaque HTTP 404 instead of "this target only supports SonarQube Server".
- Future-hostile: when Cloud-specific collectors are added, the dispatch
  logic in `collect.go` has no notion of target ↔ product compatibility,
  inviting copy-paste mistakes.

## Proposed fix

Two coordinated changes:

1. Add a target-product compatibility declaration. Cleanest as part of the
   target registry (017-IMP-target-registry); short-term, add an explicit
   check at the top of `CollectBgTasks`:

   ```go
   if instance.Product == sonarqube.Cloud {
       return fmt.Errorf("target bgtasks is not supported on SonarQube Cloud (uses Server-only /api/ce/activity)")
   }
   ```

2. Change the default URL behaviour. Either:
   - Default to empty and require the user to set `--url` / `SONAR_HOST_URL`
     with a clear error message, or
   - Keep the default but make help text say so explicitly:
     `"SonarQube base URL (default: https://sonarcloud.io — note: bgtasks target requires SonarQube Server)"`.

## Validation

- Unit test in `internal/collector` that passes a Cloud `SonarInstance` and
  asserts a descriptive error is returned without any HTTP traffic.
- Update the existing CLI tests so the default-URL behaviour matches whatever
  the chosen approach is.

## Prerequisites

None for the immediate guard. The full fix is cleaner after
[[017-IMP-target-registry]].
