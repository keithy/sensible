package main

import (
	"os"
	"testing"
)

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected []string
	}{
		{
			name:     "single script",
			args:     []string{"echo hello"},
			expected: []string{"echo hello"},
		},
		{
			name:     "three chained scripts",
			args:     []string{"echo a", "echo b", "echo c"},
			expected: []string{"echo a", "echo b", "echo c"},
		},
		{
			name:     "|| combines two scripts",
			args:     []string{"build", "||", "build-alt"},
			expected: []string{"ifelse { build } { } { build-alt }"},
		},
		{
			name:     "|| with following task",
			args:     []string{"build", "||", "build-alt", "deploy"},
			expected: []string{"ifelse { build } { } { build-alt }", "deploy"},
		},
		{
			name:     "multiple || chains",
			args:     []string{"build1", "||", "alt1", "deploy"},
			expected: []string{"ifelse { build1 } { } { alt1 }", "deploy"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseArgs(tt.args)
			if len(result) != len(tt.expected) {
				t.Errorf("got %d results, want %d", len(result), len(tt.expected))
				return
			}
			for i, r := range result {
				if r != tt.expected[i] {
					t.Errorf("result[%d] = %q, want %q", i, r, tt.expected[i])
				}
			}
		})
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}