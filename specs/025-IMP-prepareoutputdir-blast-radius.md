---
spec: 025
title: Narrow `prepareOutputDir`'s blast radius — delete per-target, not whole `--out-dir`
author: code-review
date: 2026-05-25
draft-status: ready
impl-status: in-progress
prerequisites: []
---

# IMP: Narrow `prepareOutputDir`'s blast radius — delete per-target, not whole `--out-dir`

## Problem

`cmd/collect.go`:

```go
func prepareOutputDir(outDir string) error {
    cleaned := filepath.Clean(outDir)
    if cleaned == "/" || cleaned == "." || cleaned == ".." || cleaned == os.Getenv("HOME") {
        return fmt.Errorf("refusing to remove dangerous path: %s", outDir)
    }
    if err := os.RemoveAll(outDir); err != nil { ... }
    if err := os.MkdirAll(outDir, 0o755); err != nil { ... }
    return nil
}
```

Two layered problems:

### 1. The dangerous-path check is bypassable

Only literal post-`Clean` matches are caught. The check misses:

- `~/` (literal, not expanded).
- Trailing slashes that survive: `/` after `Clean` is still `/`, OK; but
  `/usr` is not caught even though it's clearly catastrophic.
- A symlink in the user's `--out-dir` argument resolving to `/`.
- An absolute path to the user's `Documents`, `Code`, etc.
- `--out-dir=$PWD` where `$PWD` happens to be the project root.

### 2. Blast radius is the whole `--out-dir`, not the target subdirectory

Today there is one target (`bgtasks`), and bgtasks writes to
`<out-dir>/bgtasks/`. The current code deletes the *parent* `<out-dir>`,
which means if a future target writes to `<out-dir>/metrics/`, running
`collect bgtasks` will delete the metrics data. There is no way to refresh
only one target without re-collecting everything.

Spec 001 line 207 does say "the entire output directory is deleted". So
the current behaviour is per spec — but the spec was written when there
was only one target. The constraint should be re-examined now that more
targets are expected.

## Why it matters

- A user pointing `--out-dir ~/work` (because they keep all SonarQube data
  there) loses everything. The "refusing to remove dangerous path" check
  doesn't catch `~/work`.
- The per-target deletion behaviour determines whether the tool is
  composable. Without it, you can never run `collect bgtasks` in
  isolation once you've also collected something else.

## Proposed fix

Two coordinated changes:

1. **Delete per-target.** Each target is responsible for its own subdir
   (`<out-dir>/bgtasks/`, etc.) and clears only that subdir. The shared
   `<out-dir>` exists if missing but is not recursively wiped.
   `collect-metadata.json` is always overwritten with the current run's
   connection details (URL, product, version, timestamp). It records no
   target list — which targets have been collected is inferred from which
   subdirectories exist.

2. **Strengthen the dangerous-path check.** Resolve to absolute path,
   reject:
   - Paths that resolve to `/`, the user's home directory, common system
     roots (`/usr`, `/etc`, `/var`, `/opt`, `/bin`, `/sbin`).
   - Paths inside `$GOPATH` or the running binary's directory.
   - Paths shorter than 2 path components below root (e.g. `/foo` →
     prompt; `/foo/bar` → OK).
   
   A simpler stance: require the deletion target to be a directory whose
   name matches the expected pattern (e.g. ends in `/bgtasks`), and never
   delete anything else.

Spec 001 should be updated to reflect the per-target semantics.

## Validation

- `TestPrepareOutputDir_DangerousPath` extended with real dangerous paths
  (`/usr`, the test runner's `$HOME`, etc.) — using a temp directory to
  avoid actual deletion attempts.
- New test: run `collect bgtasks` twice, then introduce a sibling
  `<out-dir>/metrics/foo.json`, run `collect bgtasks` again — assert
  `metrics/foo.json` still exists.

## Prerequisites

Naturally pairs with [[014-IMP-target-registry]], which centralises the
notion of "this target owns this subdirectory".
