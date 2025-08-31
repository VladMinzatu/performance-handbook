package output

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"github.com/VladMinzatu/performance-handbook/log-aggregator/pkg/model"
)

type Output interface {
	Write(events <-chan model.LogEntry) error
}

type FileOutput struct {
	FilePath string
}

func NewFileOutput(filePath string) *FileOutput {
	return &FileOutput{FilePath: filePath}
}

func (fo *FileOutput) Write(events <-chan model.LogEntry) error {
	file, err := os.Create(fo.FilePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	for event := range events {
		eventJson, err := json.Marshal(event)
		if err != nil {
			fmt.Printf("failed to marshal event: %v\n", err)
			continue
		}
		_, err = writer.WriteString(string(eventJson) + "\n")
		if err != nil {
			fmt.Printf("failed to write to file: %v\n", err)
		}
	}

	return nil
}
