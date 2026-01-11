package ingest

import (
	"io"
	"os"
)

type IngestConfig struct {
	DocsPerSec int
	TextSize   int
	Path       string
}

type Document struct {
	ID   string
	Text string
}

func LoadData(config IngestConfig, id string) (Document, error) {
	file, err := os.Open(config.Path)
	if err != nil {
		return Document{}, err
	}
	defer file.Close()

	limitedReader := io.LimitReader(file, int64(config.TextSize))
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return Document{}, err
	}

	return Document{
		ID:   id,
		Text: string(data),
	}, nil
}
