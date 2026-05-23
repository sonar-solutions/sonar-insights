package collector

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/sonar-solutions/sonar-insights/internal/sonarqube"
)

const pageSize = 250

type pageResult struct {
	body  []byte
	total int
}

type asyncResult struct {
	page int
	body []byte
	err  error
}

func CollectBgTasks(instance sonarqube.SonarInstance, outDir string, parallel int, logger *slog.Logger) error {
	logger.Info("collecting background tasks")

	targetDir := filepath.Join(outDir, "bgtasks")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("create bgtasks output dir: %w", err)
	}

	maxExecutedAt := time.Now().Add(-5 * time.Minute).Format("2006-01-02T15:04:05-0700")
	maxExecutedAtEncoded := url.QueryEscape(maxExecutedAt)

	logger.Debug("fetching page 1 to determine total pages")
	page1, err := fetchPage(instance, maxExecutedAtEncoded, 1)
	if err != nil {
		return fmt.Errorf("fetch page 1: %w", err)
	}
	if err := writePage(page1.body, targetDir, 1); err != nil {
		return fmt.Errorf("write page 1: %w", err)
	}

	totalPages := (page1.total + pageSize - 1) / pageSize
	logger.Debug(fmt.Sprintf("collected page 1 of %d", totalPages))

	if totalPages <= 1 {
		logger.Info("background task collection complete")
		return nil
	}

	if err := fetchAndWriteRemainingPages(instance, maxExecutedAtEncoded, targetDir, totalPages, parallel, logger); err != nil {
		return err
	}

	logger.Info("background task collection complete")
	return nil
}

func fetchAndWriteRemainingPages(instance sonarqube.SonarInstance, maxExecutedAtEncoded, targetDir string, totalPages, parallel int, logger *slog.Logger) error {
	results := make(chan asyncResult, totalPages-1)
	sem := make(chan struct{}, parallel)
	var wg sync.WaitGroup

	for p := 2; p <= totalPages; p++ {
		wg.Add(1)
		go func(page int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			r, err := fetchPage(instance, maxExecutedAtEncoded, page)
			if err != nil {
				results <- asyncResult{page: page, err: err}
				return
			}
			results <- asyncResult{page: page, body: r.body}
		}(p)
	}

	wg.Wait()
	close(results)

	return writePageResults(results, targetDir, totalPages, logger)
}

func writePageResults(results <-chan asyncResult, targetDir string, totalPages int, logger *slog.Logger) error {
	var firstErr error
	for r := range results {
		if r.err != nil {
			logger.Error(fmt.Sprintf("failed to fetch page %d: %v", r.page, r.err))
			if firstErr == nil {
				firstErr = fmt.Errorf("fetch page %d: %w", r.page, r.err)
			}
			continue
		}
		logger.Debug(fmt.Sprintf("collected page %d of %d", r.page, totalPages))
		if err := writePage(r.body, targetDir, r.page); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("write page %d: %w", r.page, err)
			}
		}
	}
	return firstErr
}

func fetchPage(instance sonarqube.SonarInstance, maxExecutedAtEncoded string, page int) (pageResult, error) {
	apiURL := fmt.Sprintf("%s/api/ce/activity?maxExecutedAt=%s&ps=%d&p=%d",
		instance.BaseURL, maxExecutedAtEncoded, pageSize, page)

	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return pageResult{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", instance.AuthorizationHeader())

	resp, err := instance.Client.Do(req)
	if err != nil {
		return pageResult{}, fmt.Errorf("execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return pageResult{}, fmt.Errorf("authentication failed: invalid or missing token (HTTP 401)")
	case http.StatusForbidden:
		return pageResult{}, fmt.Errorf("access forbidden: token lacks required permissions (HTTP 403)")
	}
	if resp.StatusCode != http.StatusOK {
		return pageResult{}, fmt.Errorf("unexpected status %d from /api/ce/activity", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return pageResult{}, fmt.Errorf("read response: %w", err)
	}

	var parsed struct {
		Paging struct {
			Total int `json:"total"`
		} `json:"paging"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return pageResult{}, fmt.Errorf("parse response: %w", err)
	}

	return pageResult{body: body, total: parsed.Paging.Total}, nil
}

func writePage(body []byte, dir string, page int) error {
	filename := fmt.Sprintf("background-tasks-page-%04d.json", page)
	return os.WriteFile(filepath.Join(dir, filename), body, 0o644)
}
