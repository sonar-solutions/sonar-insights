---
spec: 001
title: Collect Background Tasks (bgtasks)
author: lfrystak
date: 2026-05-23
draft-status: ready
impl-status: not-started
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
| Auth         | Bearer token or basic authettication (note below) |
| Pagination   | `p` (page) + `ps` (page size) |
| Key params   | `maxExecutedAt` |


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

## Further requirements

Before starting implementation, read the PowerShell script as this is an implementation
reference.

The collection script must fetch pages in parallel. Five by default, but it should
be configurable on the command line. The parameter that controls this is currently
missing and must be added. The flag should be `--parallel`. Page 1 must be fetched
first to determine the total page count; remaining pages are then fetched in parallel.
Use 250 as the page size. This doesn't have to be configurable.

Information about the SonarQube Server version should be printed as a debug message.

Information about how many pages were collected out of how many should be printed as debug messages.

All errors must be handled and logged correctly.

Log basic messages at INFO level to indicate the operation started and ended.

Because the data will be processed separately, just keep the approach where raw API JSON responses are
saved to disk.

Collected data is written to `<out-dir>/bgtasks/*`. Each API page is a separate file such as `background-tasks-page-{NNNN}.json`.
The API responses don't have to be manipulated in any way, just saved.

Respect the `maxExecutedAt` filter implemented in the PowerShell script.

The PowerShell script has a dedicated switch for deciding between bearer token and
basic authentication. I don't want to implement it here like that. I want this app
to check the SonarQube server version and automatically decide what is the authentication
that needs to be used.

While the `bgtasks` target collects data for a specific purpose, we are laying the
groundwork for future work. The SonarQube version should be detected outside of this
target. It will be common to all other targets. Additionally, the version should be stored
is a `collection metadata` object that should then be serialized into `<out-dir>/collect-metadata.json`.

This app should assume it's working with SonarQube server unless the URL is on of the following:
- https://sonarcloud.io
- https://sonarcloud.us
- Staging SonarQube Cloud instances, URLs to be added later as I don't know them yet. Add a TODO comment.

If SonarQube Server version detection fails, this app should stop. No other collection task will succeed.

This app should delete the content of the output folder before running if any output already
exists. Only the output for the target(s) running should be cleared.

Bearer token should be used by default. However, on SonarQube Server versions
older than 10.2.0.X, basic authentication must be used.

