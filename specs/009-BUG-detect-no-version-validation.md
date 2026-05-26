---
spec: 009
title: `Detect` accepts any HTTP 200 body as a SonarQube Server version
author: code-review
date: 2026-05-25
draft-status: draft
impl-status: not-started
prerequisites: []
---

# BUG: `Detect` accepts any HTTP 200 body as a SonarQube Server version

## Problem

`internal/sonarqube/detect.go`:

```go
resp, err := client.Do(req)
...
body, err := io.ReadAll(resp.Body)
...
return SonarInstance{
    Product: Server,
    Version: strings.TrimSpace(string(body)),
    ...
}, nil
```

The detection trusts anything that responds with 200 OK to
`/api/server/version` as a SonarQube Server. In practice:

- A reverse proxy or captive portal returning a 200 OK HTML login page is
  stored as `Version: "<html>...</html>"`. Later, `useBearer()` runs
  `strconv.Atoi` on the first dotted token, fails, and falls back to Bearer
  auth. The misdiagnosis is silent.
- A typo (`https://sonarqube.example.com.evil.com`) that happens to return
  200 is now "trusted" as a SonarQube Server, including its token being
  shipped over the wire on every request.
- An older SonarQube path returning HTML rather than the bare version string
  produces an undetectable garbage version.

## Why it matters

- Sends auth tokens to whatever responded — small security smell.
- Produces confusing downstream errors: a malformed `Version` ripples into
  `useBearer()`, capacity reports, metadata files, and golden-master test
  assertions.
- The fix is one regex; the cost of leaving it is recurring "why doesn't my
  collection work against this URL" support pain.

## Proposed fix

Validate the body matches a SonarQube version pattern before trusting it:

```go
var sqVersionRE = regexp.MustCompile(`^\d+\.\d+(\.\d+){0,2}(\.\d+)?$`)

trimmed := strings.TrimSpace(string(body))
if !sqVersionRE.MatchString(trimmed) {
    return SonarInstance{}, fmt.Errorf(
        "URL responded to /api/server/version but body does not look like a SonarQube version: %q (got %d bytes, content-type %q)",
        truncate(trimmed, 80), len(body), resp.Header.Get("Content-Type"))
}
```

Additionally, consider checking `Content-Type` is `text/plain` (or absent),
and rejecting HTML bodies outright.

## Validation

Add tests covering:

- Body = `"</html>"` → error mentioning content-type
- Body = `"10"` (single token) — decide: reject or treat as major-only
- Body = empty → error
- Body = the existing valid versions still pass

## Prerequisites

None.
