package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
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

func TestResolveServeHostDefaultsToLoopback(t *testing.T) {
	if got := resolveServeHost("", false); got != "127.0.0.1" {
		t.Fatalf("expected loopback default host, got %q", got)
	}
	if got := resolveServeHost("0.0.0.0", false); got != "0.0.0.0" {
		t.Fatalf("expected explicit host to be preserved, got %q", got)
	}
}

func TestReadAllLimited(t *testing.T) {
	t.Run("within limit", func(t *testing.T) {
		data, err := readAllLimited(strings.NewReader("hello"), 5)
		if err != nil {
			t.Fatalf("readAllLimited failed: %v", err)
		}
		if string(data) != "hello" {
			t.Fatalf("unexpected data %q", data)
		}
	})

	t.Run("over limit", func(t *testing.T) {
		_, err := readAllLimited(strings.NewReader("hello"), 4)
		if err == nil {
			t.Fatal("expected size limit error")
		}
		if !strings.Contains(err.Error(), "limit 4 bytes") {
			t.Fatalf("unexpected error %v", err)
		}
	})

	t.Run("no limit", func(t *testing.T) {
		data, err := readAllLimited(strings.NewReader("hello"), 0)
		if err != nil {
			t.Fatalf("readAllLimited failed: %v", err)
		}
		if string(data) != "hello" {
			t.Fatalf("unexpected data %q", data)
		}
	})
}

func TestBatchErrorCode(t *testing.T) {
	if got := batchErrorCode(os.ErrNotExist); got != "NOSUCHFILE" {
		t.Fatalf("expected NOSUCHFILE, got %q", got)
	}
	if got := batchErrorCode(fmt.Errorf("wrap: %w", errInputTooLarge)); got != "INPUTTOOLARGE" {
		t.Fatalf("expected INPUTTOOLARGE, got %q", got)
	}
	if got := batchErrorCode(io.EOF); got != "READERR" {
		t.Fatalf("expected READERR, got %q", got)
	}
}

func TestBatchMode(t *testing.T) {
	// Create temporary files
	tmpDir := t.TempDir()

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

	t.Run("ignore-missing", func(t *testing.T) {
		out, err := runCmd("--batch", "--ignore-missing")
		if err != nil {
			t.Fatalf("run cmd failed: %v", err)
		}
		lines := strings.Split(strings.TrimSpace(out), "\n")
		// Since missingFile is ignored, we only expect 2 output lines (english.txt and french.txt).
		if len(lines) != 2 {
			t.Errorf("expected 2 lines of output, got %d:\n%s", len(lines), out)
		}
		if !strings.Contains(lines[0], "english.txt") {
			t.Errorf("expected first line to contain english.txt, got: %q", lines[0])
		}
		if !strings.Contains(lines[1], "french.txt") {
			t.Errorf("expected second line to contain french.txt, got: %q", lines[1])
		}
	})

	t.Run("command arguments", func(t *testing.T) {
		// Run with english.txt and french.txt as command arguments, and pass an invalid path on stdin.
		// If it reads command arguments, it should successfully process english.txt and french.txt and NOT process stdin.
		cmdArgs := []string{"run", "main.go", "--batch", engFile, fraFile}
		cmd := exec.Command("go", cmdArgs...)
		cmd.Stdin = strings.NewReader(missingFile + "\n")
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()
		if err != nil {
			t.Fatalf("run cmd failed: %v, stderr: %q", err, stderr.String())
		}

		out := stdout.String()
		lines := strings.Split(strings.TrimSpace(out), "\n")
		// We expect 2 output lines corresponding to engFile and fraFile, and NO missingFile (since missingFile is on stdin, which should be ignored).
		if len(lines) != 2 {
			t.Errorf("expected 2 lines of output, got %d:\n%s", len(lines), out)
		}
		if !strings.Contains(lines[0], "english.txt,('en',") {
			t.Errorf("expected english.txt to be classified as 'en' from command args, got: %q", lines[0])
		}
		if !strings.Contains(lines[1], "french.txt,('fr',") {
			t.Errorf("expected french.txt to be classified as 'fr' from command args, got: %q", lines[1])
		}
	})
}

func TestGoldenBatchLegacy(t *testing.T) {
	// Create temporary files
	tmpDir := t.TempDir()

	tmpDirAbs, err := filepath.Abs(tmpDir)
	if err != nil {
		t.Fatalf("failed to get absolute path of temp dir: %v", err)
	}

	// We use the same content and filenames so the output is consistent
	engFile := filepath.Join(tmpDirAbs, "english.txt")
	fraFile := filepath.Join(tmpDirAbs, "french.txt")
	missingFile := filepath.Join(tmpDirAbs, "missing.txt")

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

	// Make sure the testdata directory exists
	testdataDir := filepath.Join("testdata")
	if err := os.MkdirAll(testdataDir, 0755); err != nil {
		t.Fatalf("failed to create testdata dir: %v", err)
	}

	// We'll replace the full local temporary directory path with [TMPDIR]/ to make it platform-independent
	cleanOutput := func(out string) string {
		out = strings.ReplaceAll(out, tmpDirAbs+string(filepath.Separator), "[TMPDIR]/")
		// Also standardizing line endings just in case of Windows
		out = strings.ReplaceAll(out, "\r\n", "\n")
		return out
	}

	testCases := []struct {
		name       string
		args       []string
		goldenFile string
	}{
		{
			name:       "classic basic",
			args:       []string{"--batch", "--format", "classic"},
			goldenFile: "classic_basic.txt",
		},
		{
			name:       "classic dist",
			args:       []string{"--batch", "--format", "classic", "--dist"},
			goldenFile: "classic_dist.txt",
		},
		{
			name:       "classic normalize",
			args:       []string{"--batch", "--format", "classic", "--normalize"},
			goldenFile: "classic_norm.txt",
		},
		{
			name:       "classic dist normalize",
			args:       []string{"--batch", "--format", "classic", "--dist", "--normalize"},
			goldenFile: "classic_dist_norm.txt",
		},
	}

	update := os.Getenv("UPDATE_GOLDEN") != ""

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := runCmd(tc.args...)
			if err != nil {
				t.Fatalf("runCmd failed: %v", err)
			}

			cleaned := cleanOutput(out)
			goldenPath := filepath.Join(testdataDir, tc.goldenFile)

			if update {
				if err := os.WriteFile(goldenPath, []byte(cleaned), 0644); err != nil {
					t.Fatalf("failed to write golden file %s: %v", goldenPath, err)
				}
				t.Logf("updated golden file %s", goldenPath)
				return
			}

			expectedBytes, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("failed to read golden file %s: %v (run with UPDATE_GOLDEN=1 to generate)", goldenPath, err)
			}

			expected := string(expectedBytes)
			if cleaned != expected {
				t.Errorf("output mismatch for %s:\nGOT:\n%s\nWANT:\n%s", tc.name, cleaned, expected)
			}
		})
	}
}

func TestInteractiveMode(t *testing.T) {
	t.Run("standard interactive mode", func(t *testing.T) {
		cmd := exec.Command("go", "run", "main.go")
		cmd.Env = append(os.Environ(), "FORCE_TTY=1")

		var stdout bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stdin = strings.NewReader("this is a simple english sentence\nceci est une phrase en francais\n")

		err := cmd.Run()
		if err != nil {
			t.Fatalf("failed to run interactive mode: %v", err)
		}

		out := stdout.String()
		lines := strings.Split(out, "\n")

		if len(lines) < 4 {
			t.Fatalf("expected at least 4 output lines, got %d:\n%q", len(lines), out)
		}

		if !strings.HasPrefix(lines[0], ">>> ") || !strings.Contains(lines[0], "'en'") {
			t.Errorf("expected line 0 to have prompt and classify as english ('en'), got: %q", lines[0])
		}

		if !strings.HasPrefix(lines[1], ">>> ") || !strings.Contains(lines[1], "'fr'") {
			t.Errorf("expected line 1 to have prompt and classify as french ('fr'), got: %q", lines[1])
		}

		if lines[2] != ">>> " {
			t.Errorf("expected line 2 to be trailing prompt, got: %q", lines[2])
		}

		if lines[3] != "" {
			t.Errorf("expected line 3 to be empty after final newline, got: %q", lines[3])
		}
	})

	t.Run("normalize interactive mode", func(t *testing.T) {
		cmd := exec.Command("go", "run", "main.go", "--normalize")
		cmd.Env = append(os.Environ(), "FORCE_TTY=1")

		var stdout bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stdin = strings.NewReader("this is a simple english sentence\n")

		err := cmd.Run()
		if err != nil {
			t.Fatalf("failed to run interactive mode with normalize: %v", err)
		}

		out := stdout.String()
		lines := strings.Split(out, "\n")

		if len(lines) < 3 {
			t.Fatalf("expected at least 3 output lines, got %d:\n%q", len(lines), out)
		}

		if !strings.HasPrefix(lines[0], ">>> ") || !strings.Contains(lines[0], "'en'") || !strings.Contains(lines[0], "1.0000") {
			t.Errorf("expected line 0 to have prompt, 'en' classification, and normalized score (e.g. 1.0000), got: %q", lines[0])
		}
	})

	t.Run("dist interactive mode", func(t *testing.T) {
		cmd := exec.Command("go", "run", "main.go", "--dist")
		cmd.Env = append(os.Environ(), "FORCE_TTY=1")

		var stdout bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stdin = strings.NewReader("this is a simple english sentence\n")

		err := cmd.Run()
		if err != nil {
			t.Fatalf("failed to run interactive mode with dist: %v", err)
		}

		out := stdout.String()
		lines := strings.Split(out, "\n")

		if len(lines) < 3 {
			t.Fatalf("expected at least 3 output lines, got %d:\n%q", len(lines), out)
		}

		if !strings.HasPrefix(lines[0], ">>> ") || !strings.Contains(lines[0], "[") || !strings.Contains(lines[0], "]") {
			t.Errorf("expected line 0 to print full ranking brackets, got: %q", lines[0])
		}
	})
}
