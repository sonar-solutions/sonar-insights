---
spec: 001
title: Collect Background Tasks (bgtasks)
author: lfrystak
date: 2026-05-23
draft-status: ready
impl-status: complete
prerequisites: []
---

# Spec: Collect Background Tasks (bgtasks)

## Purpose

Collect SonarQube Server background task (Compute Engine) activity to be able
to analyze this data later.

## Goal

The goal of this implementation is to port the functionality already implemented
in this PowerShell script: `https://raw.githubusercontent.com/lukas-frystak-sonarsource/sonarqube-server-scripts/refs/heads/main/src/Get-SqsBackgroundTasks.ps1`. This script was used in the past and works well.
I want to port the functionality exactly. If any other part of the specification
contradicts the existing script, ask for clarification.

## Data source

| Field        | Value |
|--------------|-------|
| SonarQube Server version | `GET /api/server/version` |
| API endpoint | `GET /api/ce/activity` |
| Auth         | Bearer token or basic authentication (see Authentication section) |
| Pagination   | `p` (page) + `ps` (page size) |
| Key params   | `maxExecutedAt` — hardcoded to `now - 5 minutes` |

## Data shape

The below snippet is an actual example of data returned by the API.

```json
{
    "tasks": [
        {
            "id": "3b8bac00-937a-45d8-bc48-87cd3d945d44",
            "type": "SCA_RESCAN_BRANCH",
            "componentId": "058329ec-685e-4dea-bcf8-3b23c6366ad9",
            "componentKey": "sqs-oss-analysis-cli:chatwoot-chatwoot",
            "componentName": "chatwoot/chatwoot",
            "componentQualifier": "TRK",
            "status": "SUCCESS",
            "submittedAt": "2026-05-23T05:55:31+0000",
            "startedAt": "2026-05-23T05:55:31+0000",
            "executedAt": "2026-05-23T05:55:33+0000",
            "executionTimeMs": 1968,
            "hasScannerContext": false,
            "warningCount": 0,
            "warnings": [],
            "nodeName": "sonarqube-rel-sonarqube-dce-app-b78599d4c-6dskp",
            "infoMessages": []
        },
        {
            "id": "4b311646-32d8-41ff-afbf-6874138ff3ff",
            "type": "SCA_RESCAN_BRANCH",
            "componentId": "3ceed2cd-b966-470f-bd99-67762d5aea60",
            "componentKey": "sqs-oss-analysis-cli:home-assistant-iOS",
            "componentName": "home-assistant/iOS",
            "componentQualifier": "TRK",
            "status": "SUCCESS",
            "submittedAt": "2026-05-22T19:47:55+0000",
            "startedAt": "2026-05-22T19:47:55+0000",
            "executedAt": "2026-05-22T19:47:56+0000",
            "executionTimeMs": 626,
            "hasScannerContext": false,
            "warningCount": 0,
            "warnings": [],
            "nodeName": "sonarqube-rel-sonarqube-dce-app-b78599d4c-zrrzp",
            "infoMessages": []
        }
    ],
    "paging": {
        "pageIndex": 1,
        "pageSize": 100,
        "total": 44491
    }
}
```

## Internal architecture

### SonarQube instance detection

Instance detection runs once before any collection target executes. It is implemented in `internal/sonarqube/`, which is the shared foundation for all current and future targets.

```
internal/sonarqube/
  client.go      — NewHTTPClient() *http.Client; the single place to configure timeouts,
                   and eventually custom TLS certificates.
  instance.go    — SonarInstance type and Product enum (Server | Cloud).
  detect.go      — Detect(baseURL, token, client) (SonarInstance, error).
                   Checks URL against known Cloud patterns first; otherwise calls
                   GET /api/server/version to confirm Server and parse its version.
```

`SonarInstance` carries everything a target needs:

```go
type Product int

const (
    Server Product = iota
    Cloud
)

type SonarInstance struct {
    Product Product
    Version string      // semver string (e.g. "10.8.0.100512"); empty for Cloud
    BaseURL string
    Token   string
    Client  *http.Client
}
```

The `collect` command:
1. Builds a shared `*http.Client` via `sonarqube.NewHTTPClient()`.
2. Calls `sonarqube.Detect(url, token, client)` — returns a `SonarInstance`.
3. Writes `collect-metadata.json` from the instance.
4. Passes the instance into each collector: `collector.CollectBgTasks(instance, outDir, parallel, logger)`.

Each collector branches on `instance.Product` where behaviour differs between Server and Cloud.

### HTTP client

All API calls use the single `*http.Client` held in `SonarInstance`. `sonarqube.NewHTTPClient()` is the only place HTTP client configuration lives. Do not create `http.Client` instances elsewhere. This function starts minimal (sane timeouts) and will be extended when custom TLS support is needed.

## Collection metadata

After successful instance detection, the app writes `<out-dir>/collect-metadata.json` before any target runs. If instance detection fails, the app stops and this file is not written.

Schema:

```json
{
    "sonarqubeURL": "https://example.sonarqube.com",
    "collectionTimestamp": "2026-05-23T10:00:00Z",
    "sonarqubeVersion": "10.8.0.100512",
    "targets": ["bgtasks"]
}
```

- `sonarqubeURL`: the base URL provided by the user.
- `collectionTimestamp`: UTC timestamp of when the collection run started (RFC 3339).
- `sonarqubeVersion`: the SonarQube Server version string; `null` when talking to SonarQube Cloud.
- `targets`: list of collection targets executed in this run.

## Authentication

Authentication is determined automatically based on the detected instance type and version. There is no user-facing switch.

| Condition | Auth method |
|-----------|-------------|
| SonarQube Cloud | Bearer token |
| SonarQube Server ≥ 10.2.\*.\* | Bearer token |
| SonarQube Server < 10.2.\*.\* | Basic authentication |

For basic authentication, the token is used as the username with an empty password.

## Cloud vs. Server detection

The app assumes it is communicating with SonarQube Server unless the base URL matches one of the following:

- `https://sonarcloud.io`
- `https://sonarcloud.us`
- TODO: add staging SonarQube Cloud instance URLs when known.

## Error handling

- **HTTP 401**: log an error indicating the token is invalid or missing, then abort.
- **HTTP 403**: log an error indicating the token does not have sufficient permissions, then abort.
- **Version detection failure**: log an error and abort. No collection target will run.
- **Page fetch failure during parallel collection**: log an error and abort. This should not happen under normal operation.
- All other unexpected errors must be logged with enough context to diagnose the failure, then abort.

## Further requirements

Before starting implementation, read the PowerShell script as this is an implementation
reference.

### Parallelism

The `--parallel` flag is added to the `collect` command (default: `5`). It controls how many pages are fetched concurrently and is passed into every collection target. Page 1 is always fetched first to determine the total page count; remaining pages are then fetched in parallel up to the configured limit.

Page size is fixed at 250. This is not configurable.

### Verbose flag

A `-v` / `--verbose` persistent flag must be added to the root command. When set, the logger is reconfigured to `DEBUG` level before any subcommand runs. Default is `false` (INFO level).

This flag is not currently implemented and must be added as part of this spec.

### Logging

- Log at INFO level when collection starts and ends.
- Log the detected SonarQube Server version at DEBUG level.
- Log page progress at DEBUG level (e.g. "collected page X of Y").

### Output

The entire output directory (`<out-dir>`) is deleted before any collection begins if it already exists.

Collected data is written to `<out-dir>/bgtasks/`. Each API page is saved as a separate file:

```
background-tasks-page-0001.json
background-tasks-page-0002.json
...
```

Page numbering starts at 1 and is zero-padded to 4 digits. API responses are saved verbatim with no transformation.
