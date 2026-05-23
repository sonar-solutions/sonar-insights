package sonarqube

import (
	"net/http"
	"net/http/httptest"
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, err := Detect(srv.URL, "bad", NewHTTPClient())
	if err == nil {
		t.Fatal("expected error for 401, got nil")
	}
}

func TestDetect_Server403(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	_, err := Detect(srv.URL, "bad", NewHTTPClient())
	if err == nil {
		t.Fatal("expected error for 403, got nil")
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
