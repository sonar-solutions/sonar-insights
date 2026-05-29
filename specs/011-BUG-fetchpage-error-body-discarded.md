---
spec: 011
title: `fetchPage` discards response body on non-OK status, losing diagnostic context
author: code-review
date: 2026-05-25
draft-status: ready
impl-status: complete
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

Define `errorBodySnippet` in `internal/sonarqube/httperrors.go` (a new
file) so both the collector and `detect.go` can use it without duplication:

```go
package sonarqube

import (
    "io"
    "net/http"
)

// ErrorBodySnippet reads up to 512 bytes of a non-OK response body
// and returns it as a string for inclusion in error messages.
func ErrorBodySnippet(resp *http.Response) string {
    const max = 512
    body, _ := io.ReadAll(io.LimitReader(resp.Body, max+1))
    if len(body) > max {
        body = append(body[:max], []byte("…")...)
    }
    return string(body)
}
```

In `internal/collector/bgtasks.go`, apply to **all** non-2xx paths —
401, 403, and the generic case:

```go
case http.StatusUnauthorized:
    return pageResult{}, fmt.Errorf("authentication failed (HTTP 401): %s",
        sonarqube.ErrorBodySnippet(resp))
case http.StatusForbidden:
    return pageResult{}, fmt.Errorf("access forbidden (HTTP 403): %s",
        sonarqube.ErrorBodySnippet(resp))
...
if resp.StatusCode != http.StatusOK {
    return pageResult{}, fmt.Errorf("unexpected status %d from /api/ce/activity: %s",
        resp.StatusCode, sonarqube.ErrorBodySnippet(resp))
}
```

In `internal/sonarqube/detect.go`, apply to the non-2xx paths in `Detect`
the same way.

Note: `ErrorBodySnippet` must be called before the deferred
`resp.Body.Close()` returns. This is safe since the body is read on the
error path before the function returns.

## Validation

Update existing tests in `bgtasks_test.go`:

- `TestCollectBgTasks_401`: make the test server write a JSON body
  `{"errors":[{"msg":"Invalid authentication"}]}` and assert the returned
  error contains `"Invalid authentication"`.
- `TestCollectBgTasks_UnexpectedStatus`: make the test server write a body
  such as `"maintenance mode"` and assert the returned error contains
  `"maintenance mode"`.

## Prerequisites

None.
