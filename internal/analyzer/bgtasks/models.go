// Package bgtasks loads SonarQube Compute Engine activity records from disk,
// computes summary metrics and per-bucket capacity-demand statistics, and
// renders the result as a self-contained HTML report. It is invoked by the
// parent analyzer package when the bgtasks target is requested.
package bgtasks

import (
	"encoding/json"
	"fmt"
	"time"
)

// TasksRoot is the top-level JSON structure returned by GET /api/ce/activity.
type TasksRoot struct {
	Tasks  []BgTask   `json:"tasks"`
	Paging PagingInfo `json:"paging"`
}

type PagingInfo struct {
	PageIndex int `json:"pageIndex"`
	PageSize  int `json:"pageSize"`
	Total     int `json:"total"`
}

// BgTask represents a single background task from the SonarQube Compute Engine.
type BgTask struct {
	ID                 string    `json:"id"`
	Type               string    `json:"type"`
	Status             string    `json:"status"`
	Submitter          string    `json:"submitterLogin"`
	SubmittedAt        time.Time `json:"-"`
	StartedAt          time.Time `json:"-"`
	ExecutedAt         time.Time `json:"-"`
	ExecutionTimeMs    int       `json:"executionTimeMs"`
	ComponentID        string    `json:"componentId"`
	ComponentKey       string    `json:"componentKey"`
	ComponentName      string    `json:"componentName"`
	ComponentQualifier string    `json:"componentQualifier"`
	AnalysisID         string    `json:"analysisId"`
	HasScannerContext  bool      `json:"hasScannerContext"`
	Branch             string    `json:"branch"`
	BranchType         string    `json:"branchType"`
	PullRequestID      string    `json:"pullRequest"`
	WarningCount       int       `json:"warningCount"`
	Warnings           []string  `json:"warnings"`
	InfoMessages       []string  `json:"infoMessages"`
}

// bgTaskAlias breaks the UnmarshalJSON method chain so bgTaskJSON can decode without recursion.
type bgTaskAlias BgTask

// bgTaskJSON is the raw JSON shape used only during unmarshalling.
type bgTaskJSON struct {
	bgTaskAlias
	SubmittedAt string `json:"submittedAt"`
	StartedAt   string `json:"startedAt"`
	ExecutedAt  string `json:"executedAt"`
}

func (t *BgTask) UnmarshalJSON(data []byte) error {
	var raw bgTaskJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*t = BgTask(raw.bgTaskAlias)

	var err error
	t.SubmittedAt, err = parseUTC(raw.SubmittedAt, "submittedAt")
	if err != nil {
		return err
	}
	t.StartedAt, err = parseUTC(raw.StartedAt, "startedAt")
	if err != nil {
		return err
	}
	t.ExecutedAt, err = parseUTC(raw.ExecutedAt, "executedAt")
	if err != nil {
		return err
	}
	return nil
}

func parseUTC(s, field string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse("2006-01-02T15:04:05-0700", s)
	if err != nil {
		t, err = time.Parse(time.RFC3339, s)
		if err != nil {
			return time.Time{}, fmt.Errorf("parse %s %q: %w", field, s, err)
		}
	}
	return t.UTC(), nil
}

// utcDate returns the UTC date (year, month, day) of t, truncated to midnight.
func utcDate(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
