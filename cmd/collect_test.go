package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/sonar-solutions/sonar-insights/internal/sonarqube"
)

func TestPrepareOutputDir_DangerousPath(t *testing.T) {
	cases := []string{"/", ".", ".."}
	if home := os.Getenv("HOME"); home != "" {
		cases = append(cases, home)
	}
	for _, p := range cases {
		if err := prepareOutputDir(p); err == nil {
			t.Errorf("prepareOutputDir(%q): expected error for dangerous path, got nil", p)
		}
	}
}

func TestPrepareOutputDir_CreatesDir(t *testing.T) {
	target := filepath.Join(t.TempDir(), "output")
	if err := prepareOutputDir(target); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(target); err != nil {
		t.Errorf("directory not created: %v", err)
	}
}

func TestPrepareOutputDir_RemovesExistingContent(t *testing.T) {
	target := t.TempDir()
	sentinel := filepath.Join(target, "sentinel.txt")
	if err := os.WriteFile(sentinel, []byte("existing"), 0o644); err != nil {
		t.Fatalf("write sentinel: %v", err)
	}
	if err := prepareOutputDir(target); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
		t.Error("expected existing file to be removed after prepareOutputDir")
	}
	if _, err := os.Stat(target); err != nil {
		t.Errorf("output directory should exist after recreation: %v", err)
	}
}

func TestWriteCollectMetadata_Server(t *testing.T) {
	outDir := t.TempDir()
	inst := sonarqube.SonarInstance{
		Product: sonarqube.Server,
		Version: "10.8.0.100512",
		BaseURL: "https://sonarqube.example.com",
		Token:   "tok",
	}
	if err := writeCollectMetadata(inst, []string{"bgtasks"}, outDir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(outDir, "collect-metadata.json"))
	if err != nil {
		t.Fatalf("read metadata file: %v", err)
	}
	var meta collectMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if meta.SonarQubeURL != inst.BaseURL {
		t.Errorf("SonarQubeURL: got %q, want %q", meta.SonarQubeURL, inst.BaseURL)
	}
	if meta.SonarQubeVersion == nil || *meta.SonarQubeVersion != inst.Version {
		t.Errorf("SonarQubeVersion: got %v, want %q", meta.SonarQubeVersion, inst.Version)
	}
	if len(meta.Targets) != 1 || meta.Targets[0] != "bgtasks" {
		t.Errorf("Targets: got %v, want [bgtasks]", meta.Targets)
	}
	if meta.CollectionTimestamp == "" {
		t.Error("CollectionTimestamp must not be empty")
	}
}

func TestWriteCollectMetadata_Cloud(t *testing.T) {
	outDir := t.TempDir()
	inst := sonarqube.SonarInstance{
		Product: sonarqube.Cloud,
		BaseURL: "https://sonarcloud.io",
		Token:   "tok",
	}
	if err := writeCollectMetadata(inst, []string{"bgtasks"}, outDir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(outDir, "collect-metadata.json"))
	if err != nil {
		t.Fatalf("read metadata file: %v", err)
	}
	var meta collectMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if meta.SonarQubeVersion != nil {
		t.Errorf("SonarQubeVersion: got %q, want nil for Cloud product", *meta.SonarQubeVersion)
	}
}
