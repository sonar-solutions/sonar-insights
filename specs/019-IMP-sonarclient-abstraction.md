---
spec: 019
title: Introduce a `SonarClient` interface so Server and Cloud can diverge cleanly
author: code-review
date: 2026-05-25
draft-status: ready
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
    Version() string // empty string for Cloud

    // GetJSON issues a GET against path (relative to BaseURL, e.g.
    // "/api/ce/activity"), appends query params, handles auth headers,
    // status-code checking with error-body capture, and returns the raw
    // body bytes on 2xx.
    GetJSON(ctx context.Context, path string, query url.Values) ([]byte, error)
}
```

The concrete implementation is named `sonarqube.RealClient` (not
`HTTPClient` — that name is too close to `http.Client` from stdlib and
causes confusion). It wraps the existing `SonarInstance` as configuration:

```go
type RealClient struct {
    instance SonarInstance
}

func NewRealClient(inst SonarInstance) *RealClient { return &RealClient{instance: inst} }

func (c *RealClient) GetJSON(ctx context.Context, path string, query url.Values) ([]byte, error) {
    // builds URL from c.instance.BaseURL + path + query,
    // sets auth header via c.instance.AuthorizationHeader(),
    // reads body on error via ErrorBodySnippet (from [[011-BUG-...]]),
    // returns body bytes on 200.
}
```

`SonarInstance` is kept as-is; it becomes the configuration struct passed
to `NewRealClient`. Existing callers that construct `SonarInstance` do not
change — they wrap it in `NewRealClient` before passing it to collectors.

**Retry is out of scope** for this spec. The validation section below does
not cover retry behavior.

The interface seam allows:
- Collectors to be tested with hand-rolled fakes implementing `Client`.
- Error handling (body capture from [[011-BUG-...]]) in one place.
- Future Cloud/Server divergence by implementing separate `Client` types.

## Concrete impact

- Makes the [[014-IMP-target-registry]] cleaner — targets receive a
  `Client`, not a struct full of fields.
- Resolves several latent bugs in one place ([[006-BUG-...]],
  [[011-BUG-...]], potentially [[009-BUG-...]]).
- Provides an explicit boundary between "SonarQube transport" and
  "analysis logic" that the current direct-struct-access pattern blurs.

## Validation

- Existing collector tests can replace `httptest.Server` with a simple fake
  implementing `Client` — verify they still cover the same cases.
- New tests for `RealClient.GetJSON` cover:
  - URL construction from base URL + path + query params
  - Auth header is set correctly
  - 200 → returns body bytes
  - 401/403/500 → error includes body snippet (per [[011-BUG-...]])

## Prerequisites

Better landed alongside or after [[014-IMP-target-registry]] so the new
interface can be wired through the registry cleanly.
