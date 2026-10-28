package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		code       int
		wantStdout string
		wantStderr string
	}{
		{"no args", nil, 2, "", "Usage:"},
		{"help", []string{"help"}, 0, "Usage:", ""},
		{"version", []string{"version"}, 0, "dev", ""},
		{"unknown", []string{"deploy"}, 2, "", `unknown command "deploy"`},
		{"new help", []string{"new", "-h"}, 0, "", "-database"},
		{"bad flag", []string{"new", "--bogus"}, 2, "", "flag provided but not defined"},
		{"extra args", []string{"new", "--name", "a", "--team", "b", "--image", "i", "x"}, 2, "", "unexpected arguments"},
		{"invalid claim", []string{"new", "--name", "a", "--team", "payments", "--size", "huge"}, 1, "", "size \"huge\""},
		{
			"small internal",
			[]string{"new", "--name", "preview", "--team", "payments", "--image", "env-probe:e2e"},
			0, "namespace: team-payments", "",
		},
		{
			"medium public with database",
			[]string{"new", "--name", "checkout-api", "--team", "payments", "--size", "medium", "--database",
				"--exposure", "public", "--hostname", "checkout.example.test", "--image", "env-probe:e2e"},
			0, "hostname: checkout.example.test", "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(tc.args, &stdout, &stderr)
			if code != tc.code {
				t.Fatalf("exit code %d, want %d (stderr %q)", code, tc.code, stderr.String())
			}
			if !strings.Contains(stdout.String(), tc.wantStdout) {
				t.Errorf("stdout %q does not contain %q", stdout.String(), tc.wantStdout)
			}
			if !strings.Contains(stderr.String(), tc.wantStderr) {
				t.Errorf("stderr %q does not contain %q", stderr.String(), tc.wantStderr)
			}
		})
	}
}
