---
spec: 021
title: Record `Product` (Server vs. Cloud) in `collect-metadata.json`
author: code-review
date: 2026-05-25
draft-status: ready
impl-status: not-started
prerequisites: []
---

# IMP: Record `Product` (Server vs. Cloud) in `collect-metadata.json`

## Problem

`collect-metadata.json` today:

```json
{
    "sonarqubeURL": "https://sq-aks-...",
    "collectionTimestamp": "2026-05-24T05:50:56Z",
    "sonarqubeVersion": "2026.1.0.119033",
    "targets": ["bgtasks"]
}
```

Two issues:

1. The product type is implicit — readers infer it from `sonarqubeVersion`
   being non-null (Server) or null (Cloud). Spec 001 documents this inference
   rule. It works today but won't scale:
   - Some future target may collect data on either product and produce
     different output structure. The analyzer needs an explicit signal.
   - The metadata file is the contract between `collect` and `analyze`. The
     shape of the contract should not rely on null-as-magic.
   - A user reading the metadata file shouldn't have to know the inference rule.

2. The `targets` field is unnecessary. Which targets have been collected
   is self-evident from which subdirectories exist under `<data-dir>`. The
   metadata file records connection details only — not what was collected.

## Why it matters

- Required for landing [[007-BUG-cloud-detected-but-bgtasks-unsupported]]'s
  full fix — the analyzer should be able to reject data collected against
  the wrong product.
- Required for [[019-IMP-sonarclient-abstraction]] if Cloud-specific
  analyzers want to consume Cloud-collected data.

## Proposed fix

Add an explicit `product` field and remove `targets`:

```json
{
    "sonarqubeURL": "...",
    "collectionTimestamp": "...",
    "product": "Server",
    "sonarqubeVersion": "2026.1.0.119033"
}
```

```go
type collectMetadata struct {
    SonarQubeURL        string  `json:"sonarqubeURL"`
    CollectionTimestamp string  `json:"collectionTimestamp"`
    Product             string  `json:"product"` // "Server" | "Cloud"
    SonarQubeVersion    *string `json:"sonarqubeVersion"`
}
```

`sonarqube.Product` is a typed `int` (`Server = 0`, `Cloud = 1`). Convert
it to a string when writing the metadata — do not serialize the raw int.
Add a helper or use a switch in `writeCollectMetadata`:

```go
func productName(p sonarqube.Product) string {
    switch p {
    case sonarqube.Server:
        return "Server"
    case sonarqube.Cloud:
        return "Cloud"
    default:
        return "Unknown"
    }
}
```

This is a breaking change to anything that consumes the file today —
but the only consumer is this app itself, which has no current analyzer
code that reads `collect-metadata.json`. So the breakage is zero-cost.

## Validation

- Update `TestWriteCollectMetadata_Server` and `TestWriteCollectMetadata_Cloud`
  (in `cmd/collect_test.go`) to assert the `product` field contains the
  string `"Server"` or `"Cloud"` respectively — not an integer.
- The analyzer mismatched-product test requires first adding metadata reading
  to `AnalyzeBgTasks`: add a `loadCollectMetadata(dir string)` function in
  the analyzer that reads and unmarshals `collect-metadata.json`, then asserts
  the product matches what the target supports. Once that exists, add a test
  that points the analyzer at data collected for Cloud while it expects Server
  — assert it returns an error.

## Prerequisites

None.
