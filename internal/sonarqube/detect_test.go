package sonarqube

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDetect_Cloud(t *testing.T) {
	for _, url := range []string{"https://sonarcloud.io", "https://sonarcloud.us", "https://sonarcloud.io/"} {
		inst, err := Detect(url, "tok", NewHTTPClient())
		if err != nil {
			t.Fatalf("url %s: unexpected error: %v", url, err)
		}
		if inst.Product != Cloud {
			t.Errorf("url %s: expected Cloud, got %v", url, inst.Product)
		}
		if inst.Version != "" {
			t.Errorf("url %s: expected empty version for Cloud, got %q", url, inst.Version)
		}
	}
}

func TestDetect_Server(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/server/version" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("10.8.0.100512"))
	}))
	defer srv.Close()

	inst, err := Detect(srv.URL, "tok", NewHTTPClient())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inst.Product != Server {
		t.Errorf("expected Server, got %v", inst.Product)
	}
	if inst.Version != "10.8.0.100512" {
		t.Errorf("expected version 10.8.0.100512, got %q", inst.Version)
	}
	if inst.BaseURL != srv.URL {
		t.Errorf("unexpected BaseURL: %q", inst.BaseURL)
	}
}

func TestDetect_Server401(t *testing.T) {
	t.Run("bare 401 uses fallback hint", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer srv.Close()

		_, err := Detect(srv.URL, "bad", NewHTTPClient())
		if err == nil {
			t.Fatal("expected error for 401, got nil")
		}
		if !strings.Contains(err.Error(), "invalid or missing token") {
			t.Errorf("expected fallback hint in error when body is empty, got: %v", err)
		}
	})

	t.Run("401 with body uses body content", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = fmt.Fprint(w, `{"errors":[{"msg":"Token has expired"}]}`)
		}))
		defer srv.Close()

		_, err := Detect(srv.URL, "bad", NewHTTPClient())
		if err == nil {
			t.Fatal("expected error for 401, got nil")
		}
		if !strings.Contains(err.Error(), "Token has expired") {
			t.Errorf("expected body content in error, got: %v", err)
		}
	})
}

func TestDetect_Server403(t *testing.T) {
	t.Run("bare 403 uses fallback hint", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		}))
		defer srv.Close()

		_, err := Detect(srv.URL, "bad", NewHTTPClient())
		if err == nil {
			t.Fatal("expected error for 403, got nil")
		}
		if !strings.Contains(err.Error(), "insufficient permissions") {
			t.Errorf("expected fallback hint in error when body is empty, got: %v", err)
		}
	})

	t.Run("403 with body uses body content", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, _ = fmt.Fprint(w, `{"errors":[{"msg":"Insufficient privileges"}]}`)
		}))
		defer srv.Close()

		_, err := Detect(srv.URL, "bad", NewHTTPClient())
		if err == nil {
			t.Fatal("expected error for 403, got nil")
		}
		if !strings.Contains(err.Error(), "Insufficient privileges") {
			t.Errorf("expected body content in error, got: %v", err)
		}
	})
}

func TestDetect_UnexpectedStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprint(w, "maintenance mode")
	}))
	defer srv.Close()

	_, err := Detect(srv.URL, "tok", NewHTTPClient())
	if err == nil {
		t.Fatal("expected error for 500, got nil")
	}
	if !strings.Contains(err.Error(), "maintenance mode") {
		t.Errorf("expected body content in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "/api/server/version") {
		t.Errorf("expected endpoint name in error, got: %v", err)
	}
}

func TestDetect_TrimsTrailingSlash(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("10.8.0.100512"))
	}))
	defer srv.Close()

	inst, err := Detect(srv.URL+"/", "tok", NewHTTPClient())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inst.BaseURL != srv.URL {
		t.Errorf("expected trailing slash stripped, got %q", inst.BaseURL)
	}
}

func TestTruncateStr(t *testing.T) {
	tests := []struct {
		name  string
		input string
		max   int
		want  string
	}{
		{name: "short string unchanged", input: "hello", max: 10, want: "hello"},
		{name: "exact length unchanged", input: "hello", max: 5, want: "hello"},
		{name: "ascii truncated", input: "hello world", max: 5, want: "hello…"},
		{name: "multi-byte chars truncated at rune boundary", input: "café au lait", max: 4, want: "café…"},
		{name: "truncation does not split multi-byte char", input: "日本語テスト", max: 3, want: "日本語…"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := truncateStr(tc.input, tc.max)
			if got != tc.want {
				t.Errorf("truncateStr(%q, %d) = %q, want %q", tc.input, tc.max, got, tc.want)
			}
		})
	}
}

func TestDetect_VersionValidation(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{name: "html body rejected", body: "</html>", wantErr: true},
		{name: "single integer rejected", body: "10", wantErr: true},
		{name: "empty body rejected", body: "", wantErr: true},
		{name: "whitespace-only rejected", body: "   ", wantErr: true},
		{name: "four-part version accepted", body: "10.8.0.91563", wantErr: false},
		{name: "year-based four-part version accepted", body: "2026.1.0.119033", wantErr: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			_, err := Detect(srv.URL, "tok", NewHTTPClient())
			if tc.wantErr && err == nil {
				t.Errorf("expected error for body %q, got nil", tc.body)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("expected no error for body %q, got: %v", tc.body, err)
			}
		})
	}
}
