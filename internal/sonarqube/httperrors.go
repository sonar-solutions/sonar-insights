package sonarqube

import (
	"io"
	"net/http"
)

// ErrorBodySnippet reads up to 512 bytes of a non-OK response body
// and returns it as a string for inclusion in error messages.
func ErrorBodySnippet(resp *http.Response) string {
	const limit = 512
	body, _ := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if len(body) > limit {
		body = append(body[:limit], []byte("…")...)
	}
	return string(body)
}
