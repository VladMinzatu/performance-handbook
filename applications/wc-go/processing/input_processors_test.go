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

var processorFuncs = []struct {
	name     string
	procFunc func(reader io.Reader, lineProcessors []LineProcessor) error
}{
	{
		name:     "runWithScannerOnReader",
		procFunc: runWithScannerOnReader,
	},
	{
		name:     "runWithUpFrontLoadingOnReader",
		procFunc: runWithUpFrontLoadingOnReader,
	},
	{
		name:     "runWithBufferringOnReader",
		procFunc: runWithBufferringOnReader,
	},
}

var readerFuncs = []struct {
	name string
	fn   func(io.Reader, []LineProcessor) error
}{
	{"runWithScannerOnReader", runWithScannerOnReader},
	{"runWithUpFrontLoadingOnReader", runWithUpFrontLoadingOnReader},
	{"runWithBufferringOnReader", runWithBufferringOnReader},
}

var fileFuncs = []struct {
	name string
	fn   func(string, []LineProcessor) error
}{
	{"runWithScannerOnReader", runWithScannerOnFile},
	{"runWithUpFrontLoadingOnReader", runWithUpFrontLoadingOnFile},
	{"runWithBufferringOnReader", runWithBufferringOnFile},
}

func TestInputProcessors_CommonCases(t *testing.T) {
	for _, tt := range commonTestCases {
		t.Run(tt.name, func(t *testing.T) {
			for _, proc := range processorFuncs {
				wordProcessor := &WordCountProcessor{}
				lineProcessor := &LineCountProcessor{}
				charProcessor := &CharacterCountProcessor{}

				lineProcessors := []LineProcessor{wordProcessor, lineProcessor, charProcessor}

				err := proc.procFunc(strings.NewReader(tt.input), lineProcessors)
				if err != nil {
					t.Fatalf("%s.Run failed: %v", proc.name, err)
				}

				if wordProcessor.Count() != tt.expectedWords {
					t.Errorf("[%s] WordCountProcessor.Count() = %d, expected %d", proc.name, wordProcessor.Count(), tt.expectedWords)
				}
				if lineProcessor.Count() != tt.expectedLines {
					t.Errorf("[%s] LineCountProcessor.Count() = %d, expected %d", proc.name, lineProcessor.Count(), tt.expectedLines)
				}
				if charProcessor.Count() != tt.expectedChars {
					t.Errorf("[%s] CharacterCountProcessor.Count() = %d, expected %d", proc.name, charProcessor.Count(), tt.expectedChars)
				}
			}
		})
	}
}

func TestInputProcessors_ErrorHandling_NilReader(t *testing.T) {
	for _, procFunc := range readerFuncs {
		err := procFunc.fn(nil, []LineProcessor{})
		if err == nil {
			t.Errorf("%s: Expected error but got none", procFunc.name)
		}
	}
}

func TestInputProcessors_ErrorHandling_ErrorReader(t *testing.T) {
	errorReader := &errorReader{}
	for _, procFunc := range readerFuncs {
		err := procFunc.fn(errorReader, []LineProcessor{})
		if err == nil {
			t.Errorf("%s: Expected error but got none", procFunc.name)
		}
	}
}

func TestInputProcessors_ErrorHandling_EmptyFilePath(t *testing.T) {
	for _, procFunc := range fileFuncs {
		err := procFunc.fn("", []LineProcessor{})
		if err == nil {
			t.Errorf("%s: Expected error but got none", procFunc.name)
		}
	}
}
