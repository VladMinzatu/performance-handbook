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

func LoadData(path string, offset int, textSize int, id string) (Document, error) {
	file, err := os.Open(path)
	if err != nil {
		return Document{}, err
	}
	defer file.Close()

	// Seek to the specified offset
	_, err = file.Seek(int64(offset), io.SeekStart)
	if err != nil {
		return Document{}, err
	}

	limitedReader := io.LimitReader(file, int64(textSize))
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return Document{}, err
	}

	return Document{
		ID:   id,
		Text: string(data),
	}, nil
}
