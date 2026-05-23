package sonarqube

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

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
		return SonarInstance{}, fmt.Errorf("authentication failed: invalid or missing token (HTTP 401)")
	case http.StatusForbidden:
		return SonarInstance{}, fmt.Errorf("access forbidden: insufficient permissions (HTTP 403)")
	}
	if resp.StatusCode != http.StatusOK {
		return SonarInstance{}, fmt.Errorf("unexpected status %d from /api/server/version", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return SonarInstance{}, fmt.Errorf("read server version response: %w", err)
	}

	return SonarInstance{
		Product: Server,
		Version: strings.TrimSpace(string(body)),
		BaseURL: trimmed,
		Token:   token,
		Client:  client,
	}, nil
}
