package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestPreprocessArgs(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "no -l flag",
			input:    []string{"-d", "-n", "-b"},
			expected: []string{"-d", "-n", "-b"},
		},
		{
			name:     "standalone -l",
			input:    []string{"-l"},
			expected: []string{"--line"},
		},
		{
			name:     "standalone -l with other flags",
			input:    []string{"-l", "-d", "-n"},
			expected: []string{"--line", "-d", "-n"},
		},
		{
			name:     "-l followed by other flag",
			input:    []string{"-d", "-l", "-n"},
			expected: []string{"-d", "--line", "-n"},
		},
		{
			name:     "-l followed by languages (Python style)",
			input:    []string{"-l", "en,de"},
			expected: []string{"--langs", "en,de"},
		},
		{
			name:     "-l followed by single language",
			input:    []string{"-l", "en"},
			expected: []string{"--langs", "en"},
		},
		{
			name:     "-l with equals sign (Python style)",
			input:    []string{"-l=en,de"},
			expected: []string{"--langs", "en,de"},
		},
		{
			name:     "--l with equals sign",
			input:    []string{"--l=en"},
			expected: []string{"--langs", "en"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := preprocessArgs(tc.input)
			if !reflect.DeepEqual(got, tc.expected) {
				t.Errorf("preprocessArgs(%v) = %v; want %v", tc.input, got, tc.expected)
			}
		})
	}
}

func TestBatchMode(t *testing.T) {
	// Create temporary files
	tmpDir, err := os.MkdirTemp(".", "batch_test_")
	if err != nil {
		t.Fatalf("failed to create local temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	engFile := filepath.Join(tmpDir, "english.txt")
	fraFile := filepath.Join(tmpDir, "french.txt")
	missingFile := filepath.Join(tmpDir, "missing.txt")

	if err := os.WriteFile(engFile, []byte("this is a very simple english sentence"), 0644); err != nil {
		t.Fatalf("failed to write english file: %v", err)
	}
	if err := os.WriteFile(fraFile, []byte("ceci est une phrase en francais"), 0644); err != nil {
		t.Fatalf("failed to write french file: %v", err)
	}

	// Prepare stdin payload with paths
	stdinInput := strings.Join([]string{engFile, fraFile, missingFile}, "\n") + "\n"

	// Helper to run main.go with given args
	runCmd := func(args ...string) (string, error) {
		cmdArgs := append([]string{"run", "main.go"}, args...)
		cmd := exec.Command("go", cmdArgs...)
		cmd.Stdin = strings.NewReader(stdinInput)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()
		if err != nil {
			return "", fmt.Errorf("err: %v, stderr: %q", err, stderr.String())
		}
		return stdout.String(), nil
	}

	t.Run("classic format", func(t *testing.T) {
		out, err := runCmd("--batch", "--format", "classic")
		if err != nil {
			t.Fatalf("run cmd failed: %v", err)
		}
		lines := strings.Split(strings.TrimSpace(out), "\n")
		if len(lines) != 3 {
			t.Errorf("expected 3 lines of output, got %d:\n%s", len(lines), out)
		}
		if !strings.Contains(lines[0], "english.txt,('en',") {
			t.Errorf("expected english.txt to be classified as 'en' in classic, got: %q", lines[0])
		}
		if !strings.Contains(lines[1], "french.txt,('fr',") {
			t.Errorf("expected french.txt to be classified as 'fr' in classic, got: %q", lines[1])
		}
		if !strings.HasSuffix(lines[2], "missing.txt,NOSUCHFILE") {
			t.Errorf("expected missing.txt to say NOSUCHFILE, got: %q", lines[2])
		}
	})

	t.Run("csv format", func(t *testing.T) {
		out, err := runCmd("--batch", "--format", "csv")
		if err != nil {
			t.Fatalf("run cmd failed: %v", err)
		}
		r := csv.NewReader(strings.NewReader(out))
		records, err := r.ReadAll()
		if err != nil {
			t.Fatalf("failed to parse CSV: %v", err)
		}
		if len(records) != 3 {
			t.Errorf("expected 3 CSV records, got %d:\n%s", len(records), out)
		}
		// Row 0: english.txt, en, confidence
		if records[0][0] != engFile || records[0][1] != "en" {
			t.Errorf("unexpected record 0: %v", records[0])
		}
		// Row 1: french.txt, fr, confidence
		if records[1][0] != fraFile || records[1][1] != "fr" {
			t.Errorf("unexpected record 1: %v", records[1])
		}
		// Row 2: missing.txt, NOSUCHFILE, ""
		if records[2][0] != missingFile || records[2][1] != "NOSUCHFILE" || records[2][2] != "" {
			t.Errorf("unexpected record 2: %v", records[2])
		}
	})

	t.Run("jsonl format", func(t *testing.T) {
		out, err := runCmd("--batch", "--format", "jsonl")
		if err != nil {
			t.Fatalf("run cmd failed: %v", err)
		}
		lines := strings.Split(strings.TrimSpace(out), "\n")
		if len(lines) != 3 {
			t.Errorf("expected 3 lines, got %d:\n%s", len(lines), out)
		}

		type jsonlClassifyRow struct {
			Path       string  `json:"path"`
			Language   string  `json:"language"`
			Confidence float64 `json:"confidence"`
			Error      string  `json:"error"`
		}

		var row0, row1, row2 jsonlClassifyRow
		if err := json.Unmarshal([]byte(lines[0]), &row0); err != nil {
			t.Fatalf("failed to parse json row 0: %v", err)
		}
		if err := json.Unmarshal([]byte(lines[1]), &row1); err != nil {
			t.Fatalf("failed to parse json row 1: %v", err)
		}
		if err := json.Unmarshal([]byte(lines[2]), &row2); err != nil {
			t.Fatalf("failed to parse json row 2: %v", err)
		}

		if row0.Path != engFile || row0.Language != "en" || row0.Confidence >= 0 {
			t.Errorf("unexpected row 0: %+v", row0)
		}
		if row1.Path != fraFile || row1.Language != "fr" || row1.Confidence >= 0 {
			t.Errorf("unexpected row 1: %+v", row1)
		}
		if row2.Path != missingFile || row2.Error != "NOSUCHFILE" {
			t.Errorf("unexpected row 2: %+v", row2)
		}
	})
}
