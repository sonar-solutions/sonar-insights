package cmd

import (
	"strings"
	"testing"
)

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
