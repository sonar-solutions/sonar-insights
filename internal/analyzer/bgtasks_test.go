package analyzer

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sonar-solutions/sonar-insights/internal/analyzer/bgtasks"
)

var testLogger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

func ptr(t time.Time) *time.Time { return &t }

func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func TestToEndOfDay(t *testing.T) {
	cases := []struct {
		name string
		in   time.Time
		want time.Time
	}{
		{
			name: "midnight input becomes 23:59:59",
			in:   time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC),
			want: time.Date(2024, 3, 15, 23, 59, 59, 0, time.UTC),
		},
		{
			name: "non-UTC input is normalised to UTC date",
			in:   time.Date(2024, 3, 15, 12, 30, 0, 0, time.FixedZone("EST", -5*3600)),
			want: time.Date(2024, 3, 15, 23, 59, 59, 0, time.UTC),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := toEndOfDay(tc.in)
			if !got.Equal(tc.want) {
				t.Errorf("toEndOfDay(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestFilterByDate(t *testing.T) {
	may10 := date(2024, time.May, 10)
	may11 := date(2024, time.May, 11)
	may12 := date(2024, time.May, 12)

	task := func(submittedAt time.Time) bgtasks.BgTask {
		return bgtasks.BgTask{SubmittedAt: submittedAt}
	}

	cases := []struct {
		name    string
		tasks   []bgtasks.BgTask
		from    *time.Time
		to      *time.Time
		wantLen int
	}{
		{
			name:    "to boundary: midnight on to-date is included",
			tasks:   []bgtasks.BgTask{task(time.Date(2024, time.May, 11, 0, 0, 0, 0, time.UTC))},
			to:      ptr(may11),
			wantLen: 1,
		},
		{
			name:    "to boundary: 23:59:59 on to-date is included",
			tasks:   []bgtasks.BgTask{task(time.Date(2024, time.May, 11, 23, 59, 59, 0, time.UTC))},
			to:      ptr(may11),
			wantLen: 1,
		},
		{
			name:    "to boundary: midnight on day after to-date is excluded",
			tasks:   []bgtasks.BgTask{task(time.Date(2024, time.May, 12, 0, 0, 0, 0, time.UTC))},
			to:      ptr(may11),
			wantLen: 0,
		},
		{
			name:    "from boundary: midnight on from-date is included",
			tasks:   []bgtasks.BgTask{task(time.Date(2024, time.May, 11, 0, 0, 0, 0, time.UTC))},
			from:    ptr(may11),
			wantLen: 1,
		},
		{
			name:    "from boundary: 23:59:59 on day before from-date is excluded",
			tasks:   []bgtasks.BgTask{task(time.Date(2024, time.May, 10, 23, 59, 59, 0, time.UTC))},
			from:    ptr(may11),
			wantLen: 0,
		},
		{
			name: "both bounds: tasks inside range are kept, outside are dropped",
			tasks: []bgtasks.BgTask{
				task(time.Date(2024, time.May, 9, 12, 0, 0, 0, time.UTC)),    // before from
				task(time.Date(2024, time.May, 10, 0, 0, 0, 0, time.UTC)),    // exactly from
				task(time.Date(2024, time.May, 11, 12, 0, 0, 0, time.UTC)),   // between
				task(time.Date(2024, time.May, 12, 23, 59, 59, 0, time.UTC)), // exactly to end
				task(time.Date(2024, time.May, 13, 0, 0, 0, 0, time.UTC)),    // after to
			},
			from:    ptr(may10),
			to:      ptr(may12),
			wantLen: 3,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := filterByDate(tc.tasks, tc.from, tc.to)
			if len(got) != tc.wantLen {
				t.Errorf("filterByDate returned %d tasks, want %d", len(got), tc.wantLen)
			}
		})
	}
}

// --- filterType ---

func TestFilterType_RemovesMatchingType(t *testing.T) {
	tasks := []bgtasks.BgTask{
		{Type: "REPORT"},
		{Type: "ISSUE_SYNC"},
		{Type: "REPORT"},
		{Type: "ISSUE_SYNC"},
	}
	got := filterType(tasks, "ISSUE_SYNC")
	if len(got) != 2 {
		t.Fatalf("got %d tasks, want 2", len(got))
	}
	for _, task := range got {
		if task.Type == "ISSUE_SYNC" {
			t.Errorf("ISSUE_SYNC task was not filtered out")
		}
	}
}

func TestFilterType_NothingMatchingIsNoop(t *testing.T) {
	tasks := []bgtasks.BgTask{{Type: "REPORT"}, {Type: "ISSUE_SYNC"}}
	got := filterType(tasks, "NONEXISTENT")
	if len(got) != 2 {
		t.Errorf("got %d tasks, want 2 (nothing filtered)", len(got))
	}
}

// --- AnalyzeBgTasks error paths ---

const minimalTaskJSON = `{
	"tasks": [{
		"id": "task-001",
		"type": "REPORT",
		"status": "SUCCESS",
		"submittedAt": "2026-01-15T10:00:00+0000",
		"startedAt":   "2026-01-15T10:00:01+0000",
		"executedAt":  "2026-01-15T10:00:10+0000",
		"executionTimeMs": 9000,
		"componentKey": "my-project",
		"branchType": "BRANCH"
	}],
	"paging": {"pageIndex": 1, "pageSize": 500, "total": 1}
}`

func writeBgtasksDir(t *testing.T, parent string) string {
	t.Helper()
	dir := filepath.Join(parent, "bgtasks")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir bgtasks: %v", err)
	}
	return dir
}

func TestAnalyzeBgTasks_MissingBgtasksDir(t *testing.T) {
	dir := t.TempDir() // no bgtasks/ subdirectory
	err := AnalyzeBgTasks(dir, t.TempDir(), "report", nil, nil, testLogger)
	if err == nil {
		t.Fatal("expected error for missing bgtasks directory, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found', got: %v", err)
	}
}

func TestAnalyzeBgTasks_EmptyBgtasksDir(t *testing.T) {
	dir := t.TempDir()
	writeBgtasksDir(t, dir) // exists but has no JSON files
	err := AnalyzeBgTasks(dir, t.TempDir(), "report", nil, nil, testLogger)
	if err == nil {
		t.Fatal("expected error for empty bgtasks directory, got nil")
	}
	if !strings.Contains(err.Error(), "no tasks found") {
		t.Errorf("error should mention 'no tasks found', got: %v", err)
	}
}

func TestAnalyzeBgTasks_NoTasksAfterDateFilter(t *testing.T) {
	dir := t.TempDir()
	bgtasksDir := writeBgtasksDir(t, dir)
	if err := os.WriteFile(filepath.Join(bgtasksDir, "page-0001.json"), []byte(minimalTaskJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	// Task is on 2026-01-15; filter from 2026-02-01 excludes it.
	from := ptr(date(2026, time.February, 1))
	err := AnalyzeBgTasks(dir, t.TempDir(), "report", from, nil, testLogger)
	if err == nil {
		t.Fatal("expected error when all tasks are filtered by date, got nil")
	}
	if !strings.Contains(err.Error(), "no tasks matched") {
		t.Errorf("error should mention 'no tasks matched', got: %v", err)
	}
}

func TestAnalyzeBgTasks_HappyPath(t *testing.T) {
	dir := t.TempDir()
	bgtasksDir := writeBgtasksDir(t, dir)
	if err := os.WriteFile(filepath.Join(bgtasksDir, "page-0001.json"), []byte(minimalTaskJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	reportDir := t.TempDir()
	if err := AnalyzeBgTasks(dir, reportDir, "myreport", nil, nil, testLogger); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(reportDir, "myreport.html")); os.IsNotExist(err) {
		t.Error("expected report file myreport.html to be created")
	}
}
