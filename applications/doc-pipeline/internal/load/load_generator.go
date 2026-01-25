package load

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/VladMinzatu/performance-handbook/doc-pipeline/internal/ingest"
)

type LoadGeneratorConfig struct {
	MinTextSize int    // e.g. 1_000
	MaxTextSize int    // e.g. 20_000
	IDPrefix    string // e.g. "doc"
	RatePerSec  int    // e.g. 100
	FilePath    string // e.g. "data/shakespeare.txt"
	FileSize    int    // e.g. 5436475
}

type LoadGenerator struct {
	config  LoadGeneratorConfig
	out     chan ingest.DataLoadingConfig
	counter int
}

func NewLoadGenerator(config LoadGeneratorConfig) *LoadGenerator {
	return &LoadGenerator{
		config:  config,
		out:     make(chan ingest.DataLoadingConfig),
		counter: 0,
	}
}

func (l *LoadGenerator) Run(ctx context.Context) {
	go func() {
		defer close(l.out)

		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second / time.Duration(l.config.RatePerSec)):
				l.out <- generateRandomDataLoadingRequest(l.config, l.counter)
				l.counter++
			}
		}
	}()
}

func generateRandomDataLoadingRequest(config LoadGeneratorConfig, counter int) ingest.DataLoadingConfig {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	return ingest.DataLoadingConfig{
		ID:       fmt.Sprintf("%s-%d", config.IDPrefix, counter),
		FilePath: config.FilePath,
		Offset:   rng.Intn(config.FileSize - config.MinTextSize),
		TextSize: config.MinTextSize,
	}
}
