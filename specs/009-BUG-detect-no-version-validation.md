---
spec: 009
title: `Detect` accepts any HTTP 200 body as a SonarQube Server version
author: code-review
date: 2026-05-25
draft-status: ready
impl-status: in-progress
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

Define a package-level helper `truncateStr` (not `truncate`, to avoid shadowing
stdlib) and a compiled regex in `detect.go`:

```go
var sqVersionRE = regexp.MustCompile(`^\d+\.\d+(\.\d+){0,2}$`)

func truncateStr(s string, max int) string {
    if len(s) <= max {
        return s
    }
    return s[:max] + "…"
}

trimmed := strings.TrimSpace(string(body))
if !sqVersionRE.MatchString(trimmed) {
    return SonarInstance{}, fmt.Errorf(
        "URL responded to /api/server/version but body does not look like a SonarQube version: %q (got %d bytes)",
        truncateStr(trimmed, 80), len(body))
}
```

Note: the trailing `(\.\d+)?` in the original regex is redundant given
`(\.\d+){0,2}`; the corrected regex above uses `{0,2}` only, allowing
versions with 2 to 4 numeric parts (`major.minor[.patch[.build]]`).

## Validation

Add tests covering:

- Body = `"</html>"` → error (HTML does not match the version regex)
- Body = `"10"` → rejected (regex requires at least `major.minor`, two parts)
- Body = `""` (empty after trim) → error
- Body = `"10.8.0.91563"` → passes
- Body = `"2026.1.0.119033"` → passes

## Prerequisites

None.
