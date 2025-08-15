package processing

import (
	"testing"
)

func TestProcessorsImplementInterface(t *testing.T) {
	var _ LineProcessor = (*WordCountProcessor)(nil)
	var _ LineProcessor = (*LineCountProcessor)(nil)
	var _ LineProcessor = (*CharacterCountProcessor)(nil)
}

func TestWordCountProcessor(t *testing.T) {
	tests := []struct {
		name     string
		lines    []string
		expected int
	}{
		{
			name:     "empty lines",
			lines:    []string{},
			expected: 0,
		},
		{
			name:     "single line with words",
			lines:    []string{"hello world"},
			expected: 2,
		},
		{
			name:     "multiple lines with words",
			lines:    []string{"hello world", "foo bar baz", "single"},
			expected: 6,
		},
		{
			name:     "lines with extra whitespace",
			lines:    []string{"  hello   world  ", "  foo  bar  ", "  "},
			expected: 4,
		},
		{
			name:     "empty lines mixed with content",
			lines:    []string{"hello", "", "world", "  ", "foo"},
			expected: 3,
		},
		{
			name:     "unicode words",
			lines:    []string{"привет мир", "こんにちは世界", "hello 世界"},
			expected: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := &WordCountProcessor{}

			for _, line := range tt.lines {
				processor.Process([]byte(line))
			}

			if got := processor.Count(); got != tt.expected {
				t.Errorf("WordCountProcessor.Count() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestLineCountProcessor(t *testing.T) {
	tests := []struct {
		name     string
		lines    []string
		expected int
	}{
		{
			name:     "no lines",
			lines:    []string{},
			expected: 0,
		},
		{
			name:     "single line",
			lines:    []string{"hello"},
			expected: 1,
		},
		{
			name:     "multiple lines",
			lines:    []string{"line1", "line2", "line3", "line4"},
			expected: 4,
		},
		{
			name:     "empty lines count",
			lines:    []string{"", "", "content", ""},
			expected: 4,
		},
		{
			name:     "mixed content and empty",
			lines:    []string{"hello", "", "world", "  ", "foo"},
			expected: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := &LineCountProcessor{}

			for _, line := range tt.lines {
				processor.Process([]byte(line))
			}

			if got := processor.Count(); got != tt.expected {
				t.Errorf("LineCountProcessor.Count() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCharacterCountProcessor(t *testing.T) {
	tests := []struct {
		name     string
		lines    []string
		expected int
	}{
		{
			name:     "no lines",
			lines:    []string{},
			expected: 0,
		},
		{
			name:     "single line",
			lines:    []string{"hello"},
			expected: 5,
		},
		{
			name:     "multiple lines",
			lines:    []string{"hi", "world", "test"},
			expected: 11,
		},
		{
			name:     "empty lines",
			lines:    []string{"", "", ""},
			expected: 0,
		},
		{
			name:     "unicode characters",
			lines:    []string{"привет", "世界", "hello世界"},
			expected: 15, // 6 + 2 + 7 runes
		},
		{
			name:     "mixed content",
			lines:    []string{"hello", "", "世界", "  ", "test"},
			expected: 13, // 5 + 0 + 2 + 2 + 4 runes
		},
		{
			name:     "special characters",
			lines:    []string{"hello\nworld", "tab\there", "emoji"},
			expected: 24, // 11 + 8 + 5 runes
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor := &CharacterCountProcessor{}

			for _, line := range tt.lines {
				processor.Process([]byte(line))
			}

			if got := processor.Count(); got != tt.expected {
				t.Errorf("CharacterCountProcessor.Count() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestProcessorReset(t *testing.T) {
	// Resettig the counter should be done through new instance of the processor creation
	t.Run("word count processor reset", func(t *testing.T) {
		processor := &WordCountProcessor{}
		processor.Process([]byte("hello world"))
		processor.Process([]byte("foo bar"))

		if processor.Count() != 4 {
			t.Errorf("Expected 4 words, got %d", processor.Count())
		}

		newProcessor := &WordCountProcessor{}
		if newProcessor.Count() != 0 {
			t.Errorf("New processor should start with 0, got %d", newProcessor.Count())
		}
	})

	t.Run("line count processor reset", func(t *testing.T) {
		processor := &LineCountProcessor{}
		processor.Process([]byte("line1"))
		processor.Process([]byte("line2"))

		if processor.Count() != 2 {
			t.Errorf("Expected 2 lines, got %d", processor.Count())
		}

		newProcessor := &LineCountProcessor{}
		if newProcessor.Count() != 0 {
			t.Errorf("New processor should start with 0, got %d", newProcessor.Count())
		}
	})

	t.Run("character count processor reset", func(t *testing.T) {
		processor := &CharacterCountProcessor{}
		processor.Process([]byte("hello"))
		processor.Process([]byte("world"))

		if processor.Count() != 10 {
			t.Errorf("Expected 10 characters, got %d", processor.Count())
		}

		newProcessor := &CharacterCountProcessor{}
		if newProcessor.Count() != 0 {
			t.Errorf("New processor should start with 0, got %d", newProcessor.Count())
		}
	})
}
