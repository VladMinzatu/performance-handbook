package ingest

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"os"
	"time"
)

const (
	FileSize = 5436475
	FilePath = "data/shakespeare.txt"
)

type LoadGenerationConfig struct {
	MinTextSize int    // e.g. 1_000
	MaxTextSize int    // e.g. 20_000
	IDPrefix    string // e.g. "doc"
	RatePerSec  int    // e.g. 100
}

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

func GenerateLoadConfigs(ctx context.Context, config LoadGenerationConfig) (<-chan DataLoadingConfig, error) {
	configs := make(chan DataLoadingConfig)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	counter := 0

	interval := time.Second / time.Duration(config.RatePerSec)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	go func() {
		defer close(configs)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				textSize := config.MinTextSize
				if config.MaxTextSize > config.MinTextSize {
					textSize = config.MinTextSize + rng.Intn(config.MaxTextSize-config.MinTextSize+1)
				}

				maxOffset := FileSize - textSize
				offset := 0
				if maxOffset > 0 {
					offset = rng.Intn(maxOffset + 1)
				}

				counter++
				id := fmt.Sprintf("%s-%d", config.IDPrefix, counter)

				select {
				case configs <- DataLoadingConfig{
					ID:       id,
					FilePath: FilePath,
					Offset:   offset,
					TextSize: textSize,
				}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return configs, nil
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
