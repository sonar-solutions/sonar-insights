package sonarqube

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

var sqVersionRE = regexp.MustCompile(`^\d+\.\d+(\.\d+){0,2}$`)

func truncateStr(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[:limit]) + "…"
}

var cloudBaseURLs = []string{
	"https://sonarcloud.io",
	"https://sonarcloud.us",
}

func Detect(baseURL, token string, client *http.Client) (SonarInstance, error) {
	trimmed := strings.TrimRight(baseURL, "/")

	for _, cloudURL := range cloudBaseURLs {
		if strings.EqualFold(trimmed, cloudURL) {
			return SonarInstance{
				Product: Cloud,
				Version: "",
				BaseURL: trimmed,
				Token:   token,
				Client:  client,
			}, nil
		}
	}

	req, err := http.NewRequest(http.MethodGet, trimmed+"/api/server/version", nil)
	if err != nil {
		return SonarInstance{}, fmt.Errorf("create version request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return SonarInstance{}, fmt.Errorf("fetch server version: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return SonarInstance{}, fmt.Errorf("authentication failed (HTTP 401): %s",
			ErrorBodySnippet(resp))
	case http.StatusForbidden:
		return SonarInstance{}, fmt.Errorf("access forbidden (HTTP 403): %s",
			ErrorBodySnippet(resp))
	}
	if resp.StatusCode != http.StatusOK {
		return SonarInstance{}, fmt.Errorf("unexpected status %d from /api/server/version: %s",
			resp.StatusCode, ErrorBodySnippet(resp))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return SonarInstance{}, fmt.Errorf("read server version response: %w", err)
	}

	version := strings.TrimSpace(string(body))
	if !sqVersionRE.MatchString(version) {
		return SonarInstance{}, fmt.Errorf(
			"URL responded to /api/server/version but body does not look like a SonarQube version: %q (got %d bytes)",
			truncateStr(version, 80), len(body))
	}

	return SonarInstance{
		Product: Server,
		Version: version,
		BaseURL: trimmed,
		Token:   token,
		Client:  client,
	}, nil
}
