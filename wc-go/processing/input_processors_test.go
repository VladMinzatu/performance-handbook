package processing

import (
	"io"
	"strings"
	"testing"
)

type errorReader struct{}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, io.ErrUnexpectedEOF
}
func TestLineScannerInputProcessor_RunThrough(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedWords int
		expectedLines int
		expectedChars int
	}{
		{
			name:          "normal text with multiple lines",
			input:         "line 1\nline 2\nline 3",
			expectedWords: 6,
			expectedLines: 3,
			expectedChars: 18,
		},
		{
			name:          "empty input",
			input:         "",
			expectedWords: 0,
			expectedLines: 0,
			expectedChars: 0,
		},
		{
			name:          "single line",
			input:         "single line",
			expectedWords: 2,
			expectedLines: 1,
			expectedChars: 11,
		},
		{
			name:          "text with empty lines",
			input:         "line 1\n\nline 3",
			expectedWords: 4,
			expectedLines: 3,
			expectedChars: 12,
		},
		{
			name:          "text ending with newline",
			input:         "line 1\nline 2\n",
			expectedWords: 4,
			expectedLines: 2,
			expectedChars: 12,
		},
		{
			name:          "text with carriage returns",
			input:         "line 1\r\nline 2\r\nline 3",
			expectedWords: 6,
			expectedLines: 3,
			expectedChars: 18,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor, err := NewLineScannerInputProcessor(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("Unexpected error creating processor: %v", err)
			}

			wordProcessor := &WordCountProcessor{}
			lineProcessor := &LineCountProcessor{}
			charProcessor := &CharacterCountProcessor{}

			lineProcessors := []LineProcessor{wordProcessor, lineProcessor, charProcessor}

			err = processor.RunThrough(lineProcessors)
			if err != nil {
				t.Errorf("RunThrough returned an error: %v", err)
			}

			if wordProcessor.Count() != tt.expectedWords {
				t.Errorf("WordCountProcessor.Count() = %d, expected %d", wordProcessor.Count(), tt.expectedWords)
			}
			if lineProcessor.Count() != tt.expectedLines {
				t.Errorf("LineCountProcessor.Count() = %d, expected %d", lineProcessor.Count(), tt.expectedLines)
			}
			if charProcessor.Count() != tt.expectedChars {
				t.Errorf("CharacterCountProcessor.Count() = %d, expected %d", charProcessor.Count(), tt.expectedChars)
			}
		})
	}
}

func TestLineScannerInputProcessor_RunThrough_WithNilReader(t *testing.T) {
	_, err := NewLineScannerInputProcessor(nil)
	if err == nil {
		t.Error("Expected error with nil reader")
	}
}

func TestLineScannerInputProcessor_RunThrough_WithErrorReader(t *testing.T) {
	processor, err := NewLineScannerInputProcessor(&errorReader{})
	if err != nil {
		t.Fatalf("Unexpected error creating processor: %v", err)
	}

	wordProcessor := &WordCountProcessor{}
	lineProcessor := &LineCountProcessor{}
	charProcessor := &CharacterCountProcessor{}

	lineProcessors := []LineProcessor{wordProcessor, lineProcessor, charProcessor}

	err = processor.RunThrough(lineProcessors)
	if err == nil {
		t.Error("Expected error with error reader")
	}
}
