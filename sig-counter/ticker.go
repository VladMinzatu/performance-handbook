package main

import (
	"context"
	"sync/atomic"
	"time"
)

type Ticker struct {
	value uint64
}

func NewTicker() *Ticker {
	return &Ticker{
		value: 0,
	}
}

func (t *Ticker) Run(ctx context.Context) {
	tick := time.NewTicker(1 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			atomic.AddUint64(&t.value, 1)
		case <-ctx.Done():
			return
		}
	}
}

func (t *Ticker) Value() uint64 {
	return atomic.LoadUint64(&t.value)
}

func (t *Ticker) Reset() {
	atomic.StoreUint64(&t.value, 0)
}
