package load

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/VladMinzatu/performance-handbook/doc-pipeline/internal/ingest"
)

const DefaultBufferSize = 100

type LoadGeneratorMetrics interface {
	IncDataLoadingRequests(ctx context.Context, n int64)
	RecordDataLoadingRequestTextSize(ctx context.Context, textSize int64)
}

type LoadGeneratorConfig struct {
	MinTextSize int    // e.g. 1_000
	MaxTextSize int    // e.g. 20_000
	IDPrefix    string // e.g. "doc"
	RatePerSec  int    // e.g. 100
	FilePath    string // e.g. "data/shakespeare.txt"
	FileSize    int    // e.g. 5436475
}

type LoadGenerator struct {
	config     LoadGeneratorConfig
	bufferSize int
	counter    int
	rng        *rand.Rand
	metrics    LoadGeneratorMetrics
}

func NewLoadGenerator(config LoadGeneratorConfig, bufferSize int, metrics LoadGeneratorMetrics) *LoadGenerator {
	if bufferSize <= 0 {
		bufferSize = DefaultBufferSize
	}

	return &LoadGenerator{
		config:     config,
		bufferSize: bufferSize,
		counter:    0,
		rng:        rand.New(rand.NewSource(time.Now().UnixNano())),
		metrics:    metrics,
	}
}

func (l *LoadGenerator) Run(ctx context.Context) <-chan ingest.DataLoadingConfig {
	out := make(chan ingest.DataLoadingConfig, 100)

	go func() {
		defer close(out)

		ticker := time.NewTicker(time.Second / time.Duration(l.config.RatePerSec))
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				req := generateRandomDataLoadingRequest(l.config, l.counter, l.rng)
				l.metrics.IncDataLoadingRequests(ctx, 1)
				l.metrics.RecordDataLoadingRequestTextSize(ctx, int64(req.TextSize))

				out <- req
				l.counter++
			}
		}
	}()

	return out
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
