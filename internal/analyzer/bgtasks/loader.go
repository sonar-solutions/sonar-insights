package bgtasks

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

const loaderParallelism = 8

// Load reads all *.json files under dir recursively, deserialises them,
// deduplicates by ID, and returns the unique task slice.
func Load(dir string, logger *slog.Logger) ([]BgTask, error) {
	allFiles, err := collectJSONFiles(dir)
	if err != nil {
		return nil, err
	}
	logger.Debug(fmt.Sprintf("found %d bgtasks files to load", len(allFiles)))

	allTasks, err := loadFilesParallel(allFiles, logger)
	if err != nil {
		return nil, err
	}

	deduped := deduplicateTasks(allTasks)
	logger.Debug(fmt.Sprintf("removed %d duplicate tasks (kept %d unique)", len(allTasks)-len(deduped), len(deduped)))
	return deduped, nil
}

func collectJSONFiles(dir string) ([]string, error) {
	seen := make(map[string]struct{})
	var files []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && filepath.Ext(path) == ".json" {
			if _, ok := seen[path]; !ok {
				seen[path] = struct{}{}
				files = append(files, path)
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk bgtasks dir: %w", err)
	}
	return files, nil
}

type fileResult struct {
	tasks []BgTask
	file  string
	err   error
}

func loadFilesParallel(files []string, logger *slog.Logger) ([]BgTask, error) {
	results := make(chan fileResult, len(files))
	sem := make(chan struct{}, loaderParallelism)
	var wg sync.WaitGroup

	for _, f := range files {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results <- loadFile(path, logger)
		}(f)
	}

	wg.Wait()
	close(results)

	var allTasks []BgTask
	for r := range results {
		if r.err != nil {
			return nil, fmt.Errorf("load %s: %w", r.file, r.err)
		}
		allTasks = append(allTasks, r.tasks...)
	}
	return allTasks, nil
}

func loadFile(path string, logger *slog.Logger) fileResult {
	data, err := os.ReadFile(path)
	if err != nil {
		return fileResult{file: path, err: fmt.Errorf("read file: %w", err)}
	}
	var root TasksRoot
	if err := json.Unmarshal(data, &root); err != nil {
		return fileResult{file: path, err: fmt.Errorf("parse JSON: %w", err)}
	}
	logger.Debug(fmt.Sprintf("loaded %d tasks from %s", len(root.Tasks), filepath.Base(path)))
	return fileResult{tasks: root.Tasks, file: path}
}

func deduplicateTasks(tasks []BgTask) []BgTask {
	seen := make(map[string]struct{}, len(tasks))
	out := make([]BgTask, 0, len(tasks))
	for _, t := range tasks {
		if _, ok := seen[t.ID]; !ok {
			seen[t.ID] = struct{}{}
			out = append(out, t)
		}
	}
	return out
}
