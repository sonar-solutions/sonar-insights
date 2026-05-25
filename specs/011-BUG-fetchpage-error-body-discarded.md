---
spec: 011
title: `fetchPage` discards response body on non-OK status, losing diagnostic context
author: code-review
date: 2026-05-25
draft-status: ready
impl-status: not-started
prerequisites: []
---

# BUG: `fetchPage` discards response body on non-OK status, losing diagnostic context

## Problem

`internal/collector/bgtasks.go`:

```go
switch resp.StatusCode {
case http.StatusUnauthorized:
    return pageResult{}, fmt.Errorf("authentication failed: invalid or missing token (HTTP 401)")
case http.StatusForbidden:
    return pageResult{}, fmt.Errorf("access forbidden: token lacks required permissions (HTTP 403)")
}
if resp.StatusCode != http.StatusOK {
    return pageResult{}, fmt.Errorf("unexpected status %d from /api/ce/activity", resp.StatusCode)
}
```

For every non-2xx status, the response body is closed (deferred) and the
error is returned with only the status code. SonarQube's `/api/ce/activity`,
like most SonarQube endpoints, returns a JSON error body of the form:

```json
{"errors":[{"msg":"License has expired"}]}
```

Today the user sees `unexpected status 400 from /api/ce/activity` and has no
idea why. The same is true for 401/403: the body often clarifies *which*
permission is missing.

This is also true of the `Detect` path, which has the same pattern.

## Why it matters

- Real-world incidents (expired license, paused organisation, throttling,
  WAF rules) all come back as opaque status codes. Every "it's not working"
  ticket then needs a manual `curl` repro to learn what the actual problem
  is.
- Cost of the fix is trivial; cost of leaving it is recurring support load
  proportional to deployments.

## Proposed fix

Read up to N bytes of the body on error paths and include in the error:

```go
func errorBodySnippet(resp *http.Response) string {
    const max = 512
    body, _ := io.ReadAll(io.LimitReader(resp.Body, max+1))
    if len(body) > max {
        body = append(body[:max], '…')
    }
    return string(body)
}

if resp.StatusCode != http.StatusOK {
    return pageResult{}, fmt.Errorf("unexpected status %d from /api/ce/activity: %s",
        resp.StatusCode, errorBodySnippet(resp))
}
```

Apply the same helper in `detect.go`.

## Validation

Update existing tests:

- `TestCollectBgTasks_401` to assert the SonarQube error message is included
  in the wrapped error.
- `TestCollectBgTasks_UnexpectedStatus` to assert any test-provided body is
  echoed.

## Prerequisites

None.
