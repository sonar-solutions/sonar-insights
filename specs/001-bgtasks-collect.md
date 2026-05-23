# Spec: Background Tasks (bgtasks)

## Purpose

Collect and analyze SonarQube background task (Compute Engine) activity to surface trends in
task durations, failure rates, and queue behaviour across projects.

## Data source

| Field       | Value |
|-------------|-------|
| API endpoint | `GET /api/ce/activity` |
| Auth         | Bearer token (`SONAR_TOKEN`) |
| Pagination   | `p` (page) + `ps` (page size) |
| Key params   | `status`, `minSubmittedAt`, `maxExecutedAt` |

## Data shape

```json
{
  "tasks": [
    {
      "id": "string",
      "type": "string",
      "componentKey": "string",
      "componentName": "string",
      "status": "SUCCESS | FAILED | CANCELLED | PENDING | IN_PROGRESS",
      "submittedAt": "ISO8601",
      "startedAt": "ISO8601",
      "executedAt": "ISO8601",
      "executionTimeMs": 0,
      "errorMessage": "string | null"
    }
  ],
  "paging": {
    "pageIndex": 1,
    "pageSize": 100,
    "total": 0
  }
}
```

Collected data is written to `<out-dir>/bgtasks.json`.

## Analysis intent

- Task volume over time (daily/weekly counts)
- Success vs. failure rate breakdown
- p50 / p95 / p99 execution time distribution
- Top projects by task count and by total execution time
- Failed tasks with error messages for triage

## Report output

- Section title: **Background Tasks**
- Chart: task volume over time (bar chart by day)
- Table: top N projects by task count
- Table: recent failures with error messages
- KPI cards: total tasks, failure rate %, p95 execution time
