package main

import (
	"reflect"
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
