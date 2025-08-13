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

type processorTest struct {
	name          string
	input         string
	expectedWords int
	expectedLines int
	expectedChars int
}

var commonTestCases = []processorTest{
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

func runProcessorTests(t *testing.T, processorName string, createProcessor func(string) (InputProcessor, error)) {
	for _, tt := range commonTestCases {
		t.Run(tt.name, func(t *testing.T) {
			processor, err := createProcessor(tt.input)
			if err != nil {
				t.Fatalf("Failed to create %s: %v", processorName, err)
			}

			wordProcessor := &WordCountProcessor{}
			lineProcessor := &LineCountProcessor{}
			charProcessor := &CharacterCountProcessor{}

			lineProcessors := []LineProcessor{wordProcessor, lineProcessor, charProcessor}

			err = processor.RunThrough(lineProcessors)
			if err != nil {
				t.Fatalf("%s.RunThrough() failed: %v", processorName, err)
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

func TestLineScannerInputProcessor_CommonCases(t *testing.T) {
	createProcessor := func(input string) (InputProcessor, error) {
		return NewLineScannerInputProcessor(strings.NewReader(input))
	}
	runProcessorTests(t, "LineScannerInputProcessor", createProcessor)
}

func TestUpFrontLoadingInputProcessor_CommonCases(t *testing.T) {
	createProcessor := func(input string) (InputProcessor, error) {
		return NewUpFrontLoadingInputProcessorFromReader(strings.NewReader(input))
	}
	runProcessorTests(t, "UpFrontLoadingInputProcessor", createProcessor)
}

func TestBufferedInputProcessor_CommonCases(t *testing.T) {
	createProcessor := func(input string) (InputProcessor, error) {
		return NewBufferedInputProcessor(strings.NewReader(input))
	}
	runProcessorTests(t, "BufferedInputProcessor", createProcessor)
}

func TestInputProcessors_ErrorHandling(t *testing.T) {
	t.Run("LineScannerInputProcessor with nil reader", func(t *testing.T) {
		_, err := NewLineScannerInputProcessor(nil)
		if err == nil {
			t.Error("Expected error but got none")
		}
	})

	t.Run("UpFrontLoadingInputProcessor with nil reader", func(t *testing.T) {
		_, err := NewUpFrontLoadingInputProcessorFromReader(nil)
		if err == nil {
			t.Error("Expected error but got none")
		}
	})

	t.Run("UpFrontLoadingInputProcessor with empty file path", func(t *testing.T) {
		_, err := NewUpFrontLoadingInputProcessorFromFile("")
		if err == nil {
			t.Error("Expected error but got none")
		}
	})

	t.Run("BufferedInputProcessor with nil reader", func(t *testing.T) {
		_, err := NewBufferedInputProcessor(nil)
		if err == nil {
			t.Error("Expected error but got none")
		}
	})

	t.Run("MmapInputProcessor with empty file path", func(t *testing.T) {
		_, err := NewMmapInputProcessor("")
		if err == nil {
			t.Error("Expected error but got none")
		}
	})
}

func TestInputProcessors_WithErrorReader(t *testing.T) {
	errorReader := &errorReader{}

	tests := []struct {
		name       string
		createFunc func() (InputProcessor, error)
	}{
		{
			name: "LineScannerInputProcessor with error reader",
			createFunc: func() (InputProcessor, error) {
				return NewLineScannerInputProcessor(errorReader)
			},
		},
		{
			name: "UpFrontLoadingInputProcessor with error reader",
			createFunc: func() (InputProcessor, error) {
				return NewUpFrontLoadingInputProcessorFromReader(errorReader)
			},
		},
		{
			name: "BufferedInputProcessor with error reader",
			createFunc: func() (InputProcessor, error) {
				return NewBufferedInputProcessor(errorReader)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processor, err := tt.createFunc()
			if err != nil {
				t.Fatalf("Failed to create processor: %v", err)
			}

			wordProcessor := &WordCountProcessor{}
			lineProcessor := &LineCountProcessor{}
			charProcessor := &CharacterCountProcessor{}

			lineProcessors := []LineProcessor{wordProcessor, lineProcessor, charProcessor}

			err = processor.RunThrough(lineProcessors)
			if err == nil {
				t.Error("Expected error with error reader")
			}
		})
	}
}
