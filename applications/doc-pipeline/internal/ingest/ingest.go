package ingest

import (
	"io"
	"os"
)

type DataLoadingConfig struct {
	ID       string
	FilePath string
	Offset   int
	TextSize int
}

type Document struct {
	ID   string
	Text string
}

func LoadData(config DataLoadingConfig) (Document, error) {
	file, err := os.Open(config.FilePath)
	if err != nil {
		return Document{}, err
	}
	defer file.Close()

	_, err = file.Seek(int64(config.Offset), io.SeekStart)
	if err != nil {
		return Document{}, err
	}

	limitedReader := io.LimitReader(file, int64(config.TextSize))
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return Document{}, err
	}

	return Document{
		ID:   config.ID,
		Text: string(data),
	}, nil
}
