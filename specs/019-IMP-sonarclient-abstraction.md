---
spec: 019
title: Introduce a `SonarClient` interface so Server and Cloud can diverge cleanly
author: code-review
date: 2026-05-25
draft-status: draft
impl-status: not-started
prerequisites: []
---

# IMP: Introduce a `SonarClient` interface so Server and Cloud can diverge cleanly

## Problem

`SonarInstance` is a struct holding `Product`, `Version`, `BaseURL`,
`Token`, and `*http.Client`. Every consumer (today: just the bgtasks
collector) reaches into it directly:

```go
req, _ := http.NewRequest("GET", instance.BaseURL+"/api/...", nil)
req.Header.Set("Authorization", instance.AuthorizationHeader())
resp, _ := instance.Client.Do(req)
```

This is fine for one collector against one product. The roadmap requires:

- Cloud-specific endpoints (e.g. metrics or audit-log APIs that exist on
  Cloud but not Server).
- Server-specific endpoints (the current bgtasks).
- A growing surface where target X works on Server but not Cloud, or vice
  versa.

With the current shape, every collector reimplements URL construction,
authorisation, retry logic, status-code handling, and response decoding
from scratch.

## Why it matters

- The first new target (or first Cloud-vs-Server divergence) will copy
  `fetchPage` and inherit every quirk and bug — and there are several
  ([[011-BUG-fetchpage-error-body-discarded]],
   [[006-BUG-maxexecutedat-local-timezone]]).
- Tests of collectors today have to stand up a full `httptest.Server` and
  re-wire it through a `SonarInstance`. A thin client interface makes
  pure-Go fakes possible.

## Proposed approach

Introduce a small interface in `internal/sonarqube`:

```go
type Client interface {
    Product() Product
    Version() string                       // empty for Cloud

    // GetJSON issues a GET against path (relative to BaseURL), handles
    // auth, status codes, error-body capture, and returns the raw body
    // bytes on 2xx.
    GetJSON(ctx context.Context, path string, query url.Values) ([]byte, error)
}
```

`HTTPClient` is the concrete `*http.Client`-backed implementation; the
existing `SonarInstance` becomes its configuration. The interface is the
seam that:

- Collectors can be tested with hand-rolled fakes.
- Cloud and Server subclient implementations can later override what they
  need (different base path, different auth, different pagination).
- Error handling (body capture from [[011-BUG-...]]) lives in one place
  rather than per-collector.

## Concrete impact

- Makes the [[014-IMP-target-registry]] cleaner — targets receive a
  `Client`, not a struct full of fields.
- Resolves several latent bugs in one place ([[006-BUG-...]],
  [[011-BUG-...]], potentially [[009-BUG-...]]).
- Provides an explicit boundary between "SonarQube transport" and
  "analysis logic" that the current direct-struct-access pattern blurs.

## Validation

- Existing collector tests can use a simple fake implementing `Client`
  instead of `httptest.Server` — verify they still cover the same cases.
- New tests for the concrete `HTTPClient` cover URL building, auth header,
  status-code handling, error-body capture, and retry.

## Prerequisites

Better landed alongside or after [[014-IMP-target-registry]] so the new
interface can be wired through the registry cleanly.
