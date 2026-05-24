package bgtasks

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var nopLogger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

func TestLoad_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	tasks, err := Load(dir, nopLogger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(tasks))
	}
}

func TestLoad_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bad.json"), []byte("{not json}"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(dir, nopLogger)
	if err == nil {
		t.Error("expected error for malformed JSON, got nil")
	}
}

func TestLoad_DeduplicatesById(t *testing.T) {
	dir := t.TempDir()

	root := TasksRoot{
		Tasks: []BgTask{
			{ID: "a", Type: "REPORT", SubmittedAt: mustParseTime("2026-01-01T10:00:00+0000"), StartedAt: mustParseTime("2026-01-01T10:00:01+0000"), ExecutedAt: mustParseTime("2026-01-01T10:00:10+0000")},
			{ID: "b", Type: "REPORT", SubmittedAt: mustParseTime("2026-01-01T11:00:00+0000"), StartedAt: mustParseTime("2026-01-01T11:00:01+0000"), ExecutedAt: mustParseTime("2026-01-01T11:00:10+0000")},
			{ID: "a", Type: "REPORT", SubmittedAt: mustParseTime("2026-01-01T10:00:00+0000"), StartedAt: mustParseTime("2026-01-01T10:00:01+0000"), ExecutedAt: mustParseTime("2026-01-01T10:00:10+0000")}, // duplicate
		},
	}
	writeTasksFile(t, dir, "page-0001.json", root)

	tasks, err := Load(dir, nopLogger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tasks) != 2 {
		t.Errorf("expected 2 unique tasks, got %d", len(tasks))
	}
}

func TestLoad_CrossFileDeduplication(t *testing.T) {
	dir := t.TempDir()

	root1 := TasksRoot{Tasks: []BgTask{
		{ID: "x", SubmittedAt: mustParseTime("2026-01-01T10:00:00+0000"), StartedAt: mustParseTime("2026-01-01T10:00:01+0000"), ExecutedAt: mustParseTime("2026-01-01T10:00:10+0000")},
	}}
	root2 := TasksRoot{Tasks: []BgTask{
		{ID: "x", SubmittedAt: mustParseTime("2026-01-01T10:00:00+0000"), StartedAt: mustParseTime("2026-01-01T10:00:01+0000"), ExecutedAt: mustParseTime("2026-01-01T10:00:10+0000")}, // same ID, different file
		{ID: "y", SubmittedAt: mustParseTime("2026-01-02T10:00:00+0000"), StartedAt: mustParseTime("2026-01-02T10:00:01+0000"), ExecutedAt: mustParseTime("2026-01-02T10:00:10+0000")},
	}}
	writeTasksFile(t, dir, "page-0001.json", root1)
	writeTasksFile(t, dir, "page-0002.json", root2)

	tasks, err := Load(dir, nopLogger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tasks) != 2 {
		t.Errorf("expected 2 unique tasks (cross-file dedup), got %d", len(tasks))
	}
}

func TestLoad_GoldenMasterTaskCount(t *testing.T) {
	tasks, err := Load("../../../test-data", nopLogger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Golden master: TotalTasks=3816, TotalUniqueTasks=3815
	if len(tasks) != 3815 {
		t.Errorf("unique task count: got %d, want 3815", len(tasks))
	}
}

func writeTasksFile(t *testing.T, dir, name string, root TasksRoot) {
	t.Helper()
	data, err := json.Marshal(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustParseTime(s string) time.Time {
	t, err := parseUTC(s, "test")
	if err != nil {
		panic(err)
	}
	return t
}
