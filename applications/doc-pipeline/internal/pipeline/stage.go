package pipeline

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

const DefaultBufferSize = 100

type StageMetrics interface {
	RecordProcessingLatency(ctx context.Context, latency time.Duration, stageName string)
	IncStageTotalProcessedItems(ctx context.Context, stageName string)
	IncStageErrors(ctx context.Context, stageName string)
}

type Stage[I any, O any] struct {
	Name       string
	Workers    int
	BufferSize int

	in <-chan I
	fn func(I) (O, error)

	metrics StageMetrics
}

func NewStage[I any, O any](
	name string,
	workers int,
	bufferSize int,
	in <-chan I,
	fn func(I) (O, error),
	metrics StageMetrics,
) *Stage[I, O] {
	slog.Info("creating stage", "name", name, "workers", workers, "bufferSize", bufferSize)

	if workers <= 0 {
		workers = 1
	}
	if bufferSize <= 0 {
		bufferSize = DefaultBufferSize
	}

	return &Stage[I, O]{
		Name:       name,
		Workers:    workers,
		BufferSize: bufferSize,
		in:         in,
		fn:         fn,
		metrics:    metrics,
	}
}

func (s *Stage[I, O]) Run(ctx context.Context) <-chan O {
	out := make(chan O, s.BufferSize)
	slog.Info("starting stage run", "name", s.Name, "workers", s.Workers, "bufferSize", s.BufferSize)

	var wg sync.WaitGroup
	wg.Add(s.Workers)

	for i := 0; i < s.Workers; i++ {
		go func(workerID int) {
			defer wg.Done()

			for {
				select {
				case <-ctx.Done():
					slog.Info("stage run cancelled (context done)", "name", s.Name, "workerID", workerID)
					return

				case in, ok := <-s.in:
					if !ok {
						slog.Info("stage run completed (input channel closed)", "name", s.Name, "workerID", workerID)
						return
					}

					s.metrics.IncStageTotalProcessedItems(ctx, s.Name)
					startTime := time.Now()
					outVal, err := s.fn(in)
					if err != nil {
						slog.Error("error in stage - skipping", "stage", s.Name, "input", in, "error", err)
						s.metrics.IncStageErrors(ctx, s.Name)
						continue
					}

					select {
					case out <- outVal:
						s.metrics.RecordProcessingLatency(ctx, time.Since(startTime), s.Name)
					case <-ctx.Done():
						slog.Info("stage run cancelled (context done)", "name", s.Name, "workerID", workerID)
						return
					}
				}
			}
		}(i)
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}
