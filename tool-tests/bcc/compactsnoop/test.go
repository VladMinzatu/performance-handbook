package main

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

func worker(id int, allocSize int, iters int, wg *sync.WaitGroup) {
	defer wg.Done()

	for i := 0; i < iters; i++ {
		fmt.Printf("[Worker %d] Iteration %d: allocating %d MB\n", id, i+1, allocSize/(1024*1024))
		buf := make([]byte, allocSize)

		// Touch each page so memory is really allocated
		for j := 0; j < len(buf); j += 4096 {
			buf[j] = 1
		}

		// Drop reference and hint GC
		buf = nil
		runtime.GC()

		time.Sleep(500 * time.Millisecond)
	}
}

func main() {
	const allocSize = 1000 * 1024 * 1024 // 1GB per allocation
	const iterations = 15
	const workers = 6

	var wg sync.WaitGroup
	fmt.Printf("Starting %d workers, each allocating %d MB × %d iterations\n",
		workers, allocSize/(1024*1024), iterations)

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go worker(w, allocSize, iterations, &wg)
	}

	wg.Wait()
	fmt.Println("All workers finished.")
}

