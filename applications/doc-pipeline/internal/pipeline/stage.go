package pipeline

import (
	"context"
	"sync"
)

type Stage[I any, O any] struct {
	Name    string
	Workers int

	in <-chan I
	fn func(I) (O, error) // TODO: could add context.Context as a parameter, but is it necessary or worth it?
}

func NewStage[I any, O any](
	name string,
	workers int,
	in <-chan I,
	fn func(I) (O, error),
) *Stage[I, O] {
	if workers <= 0 {
		workers = 1
	}

	return &Stage[I, O]{
		Name:    name,
		Workers: workers,
		in:      in,
		fn:      fn,
	}
}

func (s *Stage[I, O]) Run(ctx context.Context, wg *sync.WaitGroup) <-chan O {
	out := make(chan O)

	for i := 0; i < s.Workers; i++ {
		wg.Add(1)
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
