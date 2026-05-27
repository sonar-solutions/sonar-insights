package collector

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sonar-solutions/sonar-insights/internal/sonarqube"
)

func noopLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func makeInstance(serverURL string) sonarqube.SonarInstance {
	return sonarqube.SonarInstance{
		Product: sonarqube.Server,
		Version: "10.8.0.100512",
		BaseURL: serverURL,
		Token:   "testtoken",
		Client:  sonarqube.NewHTTPClient(),
	}
}

func makeResponse(total, pageIndex int) []byte {
	resp := map[string]any{
		"tasks": []any{},
		"paging": map[string]any{
			"pageIndex": pageIndex,
			"pageSize":  pageSize,
			"total":     total,
		},
	}
	data, _ := json.Marshal(resp)
	return data
}

func TestBuildMaxExecutedAt_AlwaysUTC(t *testing.T) {
	loc, _ := time.LoadLocation("America/New_York")
	now := time.Date(2026, 5, 25, 12, 0, 0, 0, loc)
	got := buildMaxExecutedAt(now)
	if !strings.HasSuffix(got, "+0000") {
		t.Errorf("expected +0000 suffix, got %q", got)
	}
}

func TestCollectBgTasks_CloudNotSupported(t *testing.T) {
	inst := sonarqube.SonarInstance{
		Product: sonarqube.Cloud,
		BaseURL: "https://sonarcloud.io",
		Token:   "testtoken",
		Client:  sonarqube.NewHTTPClient(),
	}
	err := CollectBgTasks(inst, t.TempDir(), 5, noopLogger())
	if err == nil {
		t.Fatal("expected error for Cloud instance, got nil")
	}
	if !strings.Contains(err.Error(), "not supported on SonarQube Cloud") {
		t.Errorf("expected descriptive error mentioning Cloud, got: %v", err)
	}
}

func TestCollectBgTasks_SinglePage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(makeResponse(10, 1))
	}))
	defer srv.Close()

	outDir := t.TempDir()
	inst := makeInstance(srv.URL)

	if err := CollectBgTasks(inst, outDir, 5, noopLogger()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries, err := os.ReadDir(filepath.Join(outDir, "bgtasks"))
	if err != nil {
		t.Fatalf("read bgtasks dir: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 file, got %d", len(entries))
	}
	if entries[0].Name() != "background-tasks-page-0001.json" {
		t.Errorf("unexpected file name: %s", entries[0].Name())
	}
}

func TestCollectBgTasks_MultiplePages(t *testing.T) {
	const total = 600 // 3 pages of 250
	var pageCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pageCount.Add(1)
		page := 1
		_, _ = fmt.Sscanf(r.URL.Query().Get("p"), "%d", &page)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(makeResponse(total, page))
	}))
	defer srv.Close()

	outDir := t.TempDir()
	inst := makeInstance(srv.URL)

	if err := CollectBgTasks(inst, outDir, 5, noopLogger()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries, err := os.ReadDir(filepath.Join(outDir, "bgtasks"))
	if err != nil {
		t.Fatalf("read bgtasks dir: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("expected 3 files, got %d", len(entries))
	}
	if int(pageCount.Load()) != 3 {
		t.Errorf("expected 3 HTTP requests, got %d", pageCount.Load())
	}
}

func TestCollectBgTasks_FileNaming(t *testing.T) {
	const total = 510 // 3 pages
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(makeResponse(total, 1))
	}))
	defer srv.Close()

	outDir := t.TempDir()
	if err := CollectBgTasks(makeInstance(srv.URL), outDir, 5, noopLogger()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, name := range []string{
		"background-tasks-page-0001.json",
		"background-tasks-page-0002.json",
		"background-tasks-page-0003.json",
	} {
		if _, err := os.Stat(filepath.Join(outDir, "bgtasks", name)); err != nil {
			t.Errorf("expected file %s: %v", name, err)
		}
	}
}

func TestCollectBgTasks_Zero(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(makeResponse(0, 1))
	}))
	defer srv.Close()

	outDir := t.TempDir()
	if err := CollectBgTasks(makeInstance(srv.URL), outDir, 5, noopLogger()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries, _ := os.ReadDir(filepath.Join(outDir, "bgtasks"))
	if len(entries) != 1 {
		t.Errorf("expected 1 file (page 1 always written), got %d", len(entries))
	}
}

func TestCollectBgTasks_401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	outDir := t.TempDir()
	err := CollectBgTasks(makeInstance(srv.URL), outDir, 5, noopLogger())
	if err == nil {
		t.Fatal("expected error for 401, got nil")
	}
}

func TestCollectBgTasks_403(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	err := CollectBgTasks(makeInstance(srv.URL), t.TempDir(), 5, noopLogger())
	if err == nil {
		t.Fatal("expected error for 403, got nil")
	}
}

func TestCollectBgTasks_UnexpectedStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	err := CollectBgTasks(makeInstance(srv.URL), t.TempDir(), 5, noopLogger())
	if err == nil {
		t.Fatal("expected error for 500, got nil")
	}
}

func TestCollectBgTasks_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, "{invalid")
	}))
	defer srv.Close()

	err := CollectBgTasks(makeInstance(srv.URL), t.TempDir(), 5, noopLogger())
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

func TestCollectBgTasks_NetworkError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close()

	err := CollectBgTasks(makeInstance(url), t.TempDir(), 5, noopLogger())
	if err == nil {
		t.Fatal("expected error for network failure, got nil")
	}
}

func TestCollectBgTasks_ConcurrentPageError(t *testing.T) {
	const total = 600 // 3 pages of 250

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := 1
		_, _ = fmt.Sscanf(r.URL.Query().Get("p"), "%d", &page)
		if page == 1 {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(makeResponse(total, 1))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	err := CollectBgTasks(makeInstance(srv.URL), t.TempDir(), 5, noopLogger())
	if err == nil {
		t.Fatal("expected error when concurrent pages fail, got nil")
	}
}

func TestCollectBgTasks_VerbatimResponse(t *testing.T) {
	raw := makeResponse(1, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(raw)
	}))
	defer srv.Close()

	outDir := t.TempDir()
	if err := CollectBgTasks(makeInstance(srv.URL), outDir, 5, noopLogger()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	written, err := os.ReadFile(filepath.Join(outDir, "bgtasks", "background-tasks-page-0001.json"))
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(written) != string(raw) {
		t.Errorf("file content does not match raw response")
	}
}
