---
spec: 030
title: README Current Scope
author: lukas-frystak-sonarsource
date: 2026-06-03
draft-status: ready
impl-status: not-started
prerequisites: []
---

# Spec: README Current Scope

## Purpose

The README overstates the tool's capabilities by claiming SonarQube Cloud support and shows a quickstart that relies on pre-exported environment variables rather than explicit flags. This spec corrects both to accurately reflect the tool's current state.

## Goal

A reader of the README gets an accurate picture of the tool's current scope (SonarQube Server only) and sees explicit `--url`/`--token` flags in the quickstart example rather than pre-exported environment variables.

## Changes

### 1. Intro — remove Cloud claim

**Current paragraph (line 14):**

> It works against both **SonarQube Cloud** (`sonarcloud.io`, `sonarcloud.us`) and self-hosted **SonarQube Server**.

**Replace with:**

> It currently supports self-hosted **SonarQube Server**. SonarQube Cloud support is not yet available.

### 2. Usage — quickstart block

**Current quickstart block:**

```sh
export SONAR_TOKEN=squ_xxxxxxxxxxxxxxxxxxxxxxxxxxxx
sonar-insights run bgtasks
```

**Replace the block with:**

```sh
sonar-insights run bgtasks \
  --url https://sonarqube.example.com \
  --token squ_xxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

**Append the following sentence** after the existing paragraph that follows the block ("This fetches the data…works offline."):

> `--url` and `--token` can also be supplied via the `SONAR_HOST_URL` and `SONAR_TOKEN` environment variables; see the [Configuration](#configuration) section.

### 3. Configuration table — fix `--url` default

The Configuration table incorrectly shows `https://sonarcloud.io` as the default for `--url`. The code returns an error when neither `--url` nor `SONAR_HOST_URL` is set (`"--url or SONAR_HOST_URL is required"`), so there is no default — it is required.

**Current row:**

| SonarQube base URL   | `--url`    | `SONAR_HOST_URL`     | `https://sonarcloud.io`  |

**Replace with:**

| SonarQube base URL   | `--url`    | `SONAR_HOST_URL`     | _(required)_             |

## Validation

### Acceptance criteria
- The README body contains no claim that SonarQube Cloud is supported.
- The quickstart block uses `--url` and `--token` flags directly, with no `export` statement.
- Environment variables are mentioned as an alternative immediately after the quickstart, with a link to the Configuration section.
- The Configuration table shows `--url` as `_(required)_`, not `https://sonarcloud.io`.
- All other sections (How it works, Installation, Collecting data only, Analyzing previously collected data, Output layout, Development, License) are unchanged.

### Smoke test
Read `README.md` and verify:
1. `grep "sonarcloud.io"` matches only badge URLs at the top of the file, not body text.
2. The quickstart block contains `--url` and `--token`.
3. The string `export SONAR_TOKEN` does not appear in the file.
