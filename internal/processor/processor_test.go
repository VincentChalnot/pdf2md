package processor

import (
	"testing"
)

func TestLastNLines(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		n        int
		expected string
	}{
		{
			name:     "fewer lines than n",
			text:     "line1\nline2",
			n:        5,
			expected: "line1\nline2",
		},
		{
			name:     "exact n lines",
			text:     "line1\nline2\nline3",
			n:        3,
			expected: "line1\nline2\nline3",
		},
		{
			name:     "more lines than n",
			text:     "line1\nline2\nline3\nline4\nline5",
			n:        2,
			expected: "line4\nline5",
		},
		{
			name:     "single line",
			text:     "only line",
			n:        5,
			expected: "only line",
		},
		{
			name:     "trailing newline",
			text:     "line1\nline2\nline3\n",
			n:        2,
			expected: "line2\nline3",
		},
		{
			name:     "empty string",
			text:     "",
			n:        5,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := lastNLines(tt.text, tt.n)
			if result != tt.expected {
				t.Errorf("lastNLines(%q, %d) = %q, want %q", tt.text, tt.n, result, tt.expected)
			}
		})
	}
}
