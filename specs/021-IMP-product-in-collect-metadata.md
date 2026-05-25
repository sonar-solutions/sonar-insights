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

The product type is implicit — readers infer it from `sonarqubeVersion`
being non-null (Server) or null (Cloud). Spec 001 documents this inference
rule. It works today but won't scale:

- Some future target may collect data on either product and produce
  different output structure. The analyzer needs an explicit signal.
- The metadata file is the contract between `collect` and `analyze`. The
  shape of the contract should not rely on null-as-magic.
- A user reading the metadata file shouldn't have to know the inference
  rule.

## Why it matters

- Required for landing [[007-BUG-cloud-detected-but-bgtasks-unsupported]]'s
  full fix — the analyzer should be able to reject data collected against
  the wrong product.
- Required for [[019-IMP-sonarclient-abstraction]] if Cloud-specific
  analyzers want to consume Cloud-collected data.

## Proposed fix

Add an explicit `product` field:

```json
{
    "sonarqubeURL": "...",
    "collectionTimestamp": "...",
    "product": "Server",
    "sonarqubeVersion": "2026.1.0.119033",
    "targets": ["bgtasks"]
}
```

```go
type collectMetadata struct {
    SonarQubeURL        string   `json:"sonarqubeURL"`
    CollectionTimestamp string   `json:"collectionTimestamp"`
    Product             string   `json:"product"` // "Server" | "Cloud"
    SonarQubeVersion    *string  `json:"sonarqubeVersion"`
    Targets             []string `json:"targets"`
}
```

This is a breaking change to anything that consumes the file today —
but the only consumer is this app itself, which has no current analyzer
code that reads `collect-metadata.json`. So the breakage is zero-cost.

## Validation

- `TestWriteCollectMetadata_Server` and `_Cloud` updated to assert the
  `product` field.
- An analyzer test that asserts mismatched product is rejected (covers
  [[007-BUG-cloud-detected-but-bgtasks-unsupported]] from the analyzer
  side).

## Prerequisites

None.
