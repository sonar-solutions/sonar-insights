---
spec: 005
title: `--report-name` allows path traversal outside `--report-dir`
author: code-review
date: 2026-05-25
draft-status: ready
impl-status: complete
prerequisites: []
---

# BUG: `--report-name` allows path traversal outside `--report-dir`

## Problem

In `internal/analyzer/bgtasks.go` the report path is built via:

```go
reportPath := filepath.Join(reportDir, reportName+".html")
```

`reportName` comes straight from the `--report-name` flag with no validation.
A user (or a wrapping script) can pass:

```
sonar-insights analyze bgtasks --report-name '../../etc/passwd'
```

…and the file `/etc/passwd.html` will be created (or overwritten if it exists
and is writable). Other obvious bad inputs:

- `--report-name ../foo` → writes `<report-dir>/../foo.html`, outside the
  reports directory
- `--report-name /tmp/evil` → `filepath.Join` will combine but if the user
  supplies an absolute path, `filepath.Join` returns the absolute path,
  silently writing outside `--report-dir`
- `--report-name ''` → produces `<report-dir>/.html` — surprising hidden file

## Why it matters

- Path traversal is a classic OWASP issue. Even though this is a local CLI
  invoked by the user themselves, the tool may be wrapped by automation that
  forwards untrusted strings (CI pipelines, web UIs, etc.).
- The flag has a defined contract in spec 002: "Output report filename
  (without `.html` extension)". Anything containing a separator, an absolute
  path, or `..` should be rejected.

## Proposed fix

Validate `--report-name` before any file I/O:

```go
func validateReportName(name string) error {
	if name == "" {
		return fmt.Errorf("--report-name must not be empty")
	}
	if strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("--report-name must not contain path separators: %q", name)
	}
	if name == "." || name == ".." || strings.HasPrefix(name, ".") {
		return fmt.Errorf("--report-name must not begin with '.' or be a relative-path component: %q", name)
	}
	return nil
}
```

Place `validateReportName` in `cmd/analyze.go`. Call it in
`runAnalyzeBgtasksCmd` immediately after reading the `--report-name` flag,
before passing it to `runAnalyze`. The `strings.ContainsAny(name, "/\\")` check
intentionally rejects both forward and back slashes unconditionally —
this is the correct stance for a CLI tool that may run on either OS.

An additional defensive check after building the path is optional but
recommended:

```go
if rel, err := filepath.Rel(reportDir, reportPath); err != nil || strings.HasPrefix(rel, "..") {
    return fmt.Errorf("report path escapes --report-dir: %s", reportPath)
}
```

## Validation

Write tests in `cmd/analyze_test.go` (create the file, following the pattern
of `cmd/collect_test.go`). Test `validateReportName` directly:

- `""` → error containing "must not be empty"
- `"../escape"` → error containing "path separator"
- `"/abs"` → error containing "path separator"
- `"a/b"` → error containing "path separator"
- `"."` → error containing "'.'"
- `".hidden"` → error containing "'.'"
- `"my-report"` → nil (happy path)
- `"report_2026"` → nil (happy path)

## Prerequisites

None.
