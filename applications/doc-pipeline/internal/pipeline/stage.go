package pipeline

import (
	"context"
	"sync"
)

type Stage[I any, O any] struct {
	Name    string
	Workers int

	In  <-chan I
	Out chan<- O

	Fn func(context.Context, I) (O, error)
}

func (s *Stage[I, O]) Run(ctx context.Context, wg *sync.WaitGroup) {
	for i := 0; i < s.Workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for {
				select {
				case <-ctx.Done():
					return

				case in, ok := <-s.In:
					if !ok {
						return
					}

					out, err := s.Fn(ctx, in)
					if err != nil {
						// TODO: record error
						continue
					}

					select {
					case s.Out <- out:
						// TODO: record latency metric
					case <-ctx.Done():
						return
					}
				}
			}
		}(i)
	}
}
