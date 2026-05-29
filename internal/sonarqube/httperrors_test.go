package sonarqube

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"
	"unicode/utf8"
)

func makeResp(body string) *http.Response {
	return &http.Response{Body: io.NopCloser(strings.NewReader(body))}
}

func TestHTTPError_WithBody(t *testing.T) {
	err := HTTPError(makeResp(`{"errors":[{"msg":"License expired"}]}`), "unexpected status 400", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "License expired") {
		t.Errorf("expected body in error, got: %v", err)
	}
}

func TestHTTPError_EmptyBodyUsesFallback(t *testing.T) {
	err := HTTPError(makeResp(""), "access forbidden (HTTP 403)", "token lacks required permissions")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "token lacks required permissions") {
		t.Errorf("expected fallback in error, got: %v", err)
	}
}

func TestHTTPError_EmptyBodyNoFallback(t *testing.T) {
	err := HTTPError(makeResp(""), "unexpected status 503", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	msg := err.Error()
	if strings.HasSuffix(msg, ": ") {
		t.Errorf("error message must not end with trailing colon-space, got: %q", msg)
	}
	if !strings.Contains(msg, "unexpected status 503") {
		t.Errorf("expected prefix in error, got: %v", err)
	}
}

func TestErrorBodySnippet_ShortBody(t *testing.T) {
	got := ErrorBodySnippet(makeResp(`{"errors":[{"msg":"bad token"}]}`))
	if !strings.Contains(got, "bad token") {
		t.Errorf("expected body content, got %q", got)
	}
}

func TestErrorBodySnippet_EmptyBody(t *testing.T) {
	got := ErrorBodySnippet(makeResp(""))
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestErrorBodySnippet_TruncatesAt512(t *testing.T) {
	body := strings.Repeat("a", 600)
	got := ErrorBodySnippet(makeResp(body))
	if !strings.HasSuffix(got, "…") {
		t.Errorf("expected ellipsis suffix, got %q", got)
	}
	if utf8.RuneCountInString(got) > 512+1 { // 512 bytes + ellipsis rune
		t.Errorf("result too long: %d runes", utf8.RuneCountInString(got))
	}
}

func TestErrorBodySnippet_UTF8BoundaryRespected(t *testing.T) {
	// Build a body where a 3-byte CJK rune straddles the 512-byte boundary.
	// Fill 511 bytes with ASCII, then append a 3-byte rune (e.g. '中' = 0xE4 0xB8 0xAD).
	prefix := bytes.Repeat([]byte("a"), 511)
	cjk := []byte("中") // 3 bytes: 0xE4 0xB8 0xAD
	body := string(append(prefix, cjk...))

	got := ErrorBodySnippet(makeResp(body))

	if !utf8.ValidString(got) {
		t.Errorf("result is not valid UTF-8: %q", got)
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("expected ellipsis suffix, got %q", got)
	}
}
