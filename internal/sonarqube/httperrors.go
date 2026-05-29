package sonarqube

import (
	"fmt"
	"io"
	"net/http"
	"unicode/utf8"
)

// ErrorBodySnippet reads up to 512 bytes of a non-OK response body
// and returns it as a string for inclusion in error messages.
func ErrorBodySnippet(resp *http.Response) string {
	const limit = 512
	body, _ := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if len(body) > limit {
		body = body[:limit]
		// Trim incomplete trailing UTF-8 sequence before appending the ellipsis.
		for len(body) > 0 && !utf8.Valid(body) {
			body = body[:len(body)-1]
		}
		body = append(body, []byte("…")...)
	}
	return string(body)
}

// HTTPError builds an error from a non-OK response. It appends the response
// body snippet when present; otherwise it falls back to fallback (if non-empty)
// so callers never produce a message ending with ": ".
func HTTPError(resp *http.Response, prefix, fallback string) error {
	if snippet := ErrorBodySnippet(resp); snippet != "" {
		return fmt.Errorf("%s: %s", prefix, snippet)
	}
	if fallback != "" {
		return fmt.Errorf("%s: %s", prefix, fallback)
	}
	return fmt.Errorf("%s", prefix)
}
