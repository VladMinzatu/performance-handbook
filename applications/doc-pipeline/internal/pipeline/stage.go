package pipeline

import (
	"context"
	"sync"
)

const DefaultBufferSize = 100

type Stage[I any, O any] struct {
	Name       string
	Workers    int
	BufferSize int

	in <-chan I
	fn func(I) (O, error) // TODO: could add context.Context as a parameter, but is it necessary or worth it?
}

func NewStage[I any, O any](
	name string,
	workers int,
	bufferSize int,
	in <-chan I,
	fn func(I) (O, error),
) *Stage[I, O] {
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
	}
}

func (s *Stage[I, O]) Run(ctx context.Context) <-chan O {
	out := make(chan O, s.BufferSize)

	var wg sync.WaitGroup
	wg.Add(s.Workers)

	for i := 0; i < s.Workers; i++ {
		go func(workerID int) {
			defer wg.Done()

			for {
				select {
				case <-ctx.Done():
					return

				case in, ok := <-s.in:
					if !ok {
						return
					}

					outVal, err := s.fn(in)
					if err != nil {
						// TODO: record error
						continue
					}

					select {
					case out <- outVal:
						// TODO: record latency metric
					case <-ctx.Done():
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
