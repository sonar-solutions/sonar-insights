package sonarqube

import (
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
