package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestAnalyzeHelp_ContainsDataDir(t *testing.T) {
	buf := new(bytes.Buffer)
	analyzeCmd.SetOut(buf)
	_ = analyzeCmd.Help()
	if !strings.Contains(buf.String(), "--data-dir") {
		t.Errorf("analyze --help output does not contain --data-dir:\n%s", buf.String())
	}
}

func TestValidateEstimateWorkers(t *testing.T) {
	cases := []struct {
		input       []int
		wantErr     bool
		errContains string
	}{
		{input: []int{}, wantErr: false},
		{input: []int{1}, wantErr: false},
		{input: []int{1, 2, 4}, wantErr: false},
		{input: []int{0}, wantErr: true, errContains: ">= 1"},
		{input: []int{-1}, wantErr: true, errContains: ">= 1"},
		{input: []int{2, 0, 4}, wantErr: true, errContains: ">= 1"},
	}

	for _, tc := range cases {
		err := validateEstimateWorkers(tc.input)
		if tc.wantErr {
			if err == nil {
				t.Errorf("validateEstimateWorkers(%v): expected error, got nil", tc.input)
				continue
			}
			if !strings.Contains(err.Error(), tc.errContains) {
				t.Errorf("validateEstimateWorkers(%v): error %q does not contain %q", tc.input, err.Error(), tc.errContains)
			}
		} else {
			if err != nil {
				t.Errorf("validateEstimateWorkers(%v): unexpected error: %v", tc.input, err)
			}
		}
	}
}

func TestValidateReportName(t *testing.T) {
	cases := []struct {
		input       string
		wantErr     bool
		errContains string
	}{
		{input: "", wantErr: true, errContains: "must not be empty"},
		{input: "../escape", wantErr: true, errContains: "path separator"},
		{input: "/abs", wantErr: true, errContains: "path separator"},
		{input: "a/b", wantErr: true, errContains: "path separator"},
		{input: "a\\b", wantErr: true, errContains: "path separator"},
		{input: ".", wantErr: true, errContains: "'.'"},
		{input: ".hidden", wantErr: true, errContains: "'.'"},
		{input: "my-report", wantErr: false},
		{input: "report_2026", wantErr: false},
	}

	for _, tc := range cases {
		err := validateReportName(tc.input)
		if tc.wantErr {
			if err == nil {
				t.Errorf("validateReportName(%q): expected error, got nil", tc.input)
				continue
			}
			if !strings.Contains(err.Error(), tc.errContains) {
				t.Errorf("validateReportName(%q): error %q does not contain %q", tc.input, err.Error(), tc.errContains)
			}
		} else {
			if err != nil {
				t.Errorf("validateReportName(%q): unexpected error: %v", tc.input, err)
			}
		}
	}
}
