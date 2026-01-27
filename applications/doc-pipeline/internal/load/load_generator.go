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
	Out chan ingest.DataLoadingConfig

	config  LoadGeneratorConfig
	counter int
	rng     *rand.Rand
}

func NewLoadGenerator(config LoadGeneratorConfig) *LoadGenerator {
	return &LoadGenerator{
		config:  config,
		Out:     make(chan ingest.DataLoadingConfig),
		counter: 0,
		rng:     rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (l *LoadGenerator) Run(ctx context.Context) {
	go func() {
		defer close(l.Out)

		ticker := time.NewTicker(time.Second / time.Duration(l.config.RatePerSec))
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				l.Out <- generateRandomDataLoadingRequest(l.config, l.counter, l.rng)
				l.counter++
			}
		}
	}()
}

func generateRandomDataLoadingRequest(
	config LoadGeneratorConfig,
	counter int,
	rng *rand.Rand,
) ingest.DataLoadingConfig {

	minSize := config.MinTextSize
	maxSize := config.MaxTextSize

	if minSize < 1 {
		minSize = 1
	}
	if maxSize < minSize {
		maxSize = minSize
	}
	if maxSize > config.FileSize {
		maxSize = config.FileSize
	}
	if minSize > config.FileSize {
		minSize = config.FileSize
	}

	var textSize int
	if maxSize == minSize {
		textSize = minSize
	} else {
		textSize = minSize + rng.Intn(maxSize-minSize+1)
	}

	maxOffset := config.FileSize - textSize
	if maxOffset < 0 {
		maxOffset = 0
	}

	var offset int
	if maxOffset == 0 {
		offset = 0
	} else {
		offset = rng.Intn(maxOffset + 1)
	}

	return ingest.DataLoadingConfig{
		ID:       fmt.Sprintf("%s-%d", config.IDPrefix, counter),
		FilePath: config.FilePath,
		Offset:   offset,
		TextSize: textSize,
	}
}
