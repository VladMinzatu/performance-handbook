package main

import (
	"context"
	"crypto/sha1"
	"fmt"
	"net/http"
	"time"

	"github.com/grafana/pyroscope-go"
)

func busyWork(iterations int) {
	var b []byte
	for i := 0; i < iterations; i++ {
		h := sha1.New()
		h.Write([]byte(fmt.Sprintf("work-%d-%d", i, time.Now().UnixNano())))
		b = h.Sum(b)

		if i%1000 == 0 {
			time.Sleep(1 * time.Millisecond)
		}
	}
	_ = b
}

func handlerFast(w http.ResponseWriter, r *http.Request) {
	pyroscope.TagWrapper(context.Background(), pyroscope.Labels("handler", "fast"), func(ctx context.Context) {
		busyWork(2000)
	})
	fmt.Fprintln(w, "fast done")
}

func handlerSlow(w http.ResponseWriter, r *http.Request) {
	pyroscope.TagWrapper(context.Background(), pyroscope.Labels("handler", "slow"), func(ctx context.Context) {
		busyWork(12000)
	})
	fmt.Fprintln(w, "slow done")
}

func handlerMixed(w http.ResponseWriter, r *http.Request) {
	pyroscope.TagWrapper(context.Background(), pyroscope.Labels("handler", "mixed"), func(ctx context.Context) {
		busyWork(6000)
	})
	fmt.Fprintln(w, "mixed done")
}

func main() {
	pyroscope.Start(pyroscope.Config{
		ApplicationName: "example.simple-server",
		ServerAddress:   "http://localhost:4040",
		Logger:          pyroscope.StandardLogger,
		ProfileTypes: []pyroscope.ProfileType{
			pyroscope.ProfileCPU,
			pyroscope.ProfileAllocObjects,
			pyroscope.ProfileAllocSpace,
			pyroscope.ProfileGoroutines,
		},
	})

	http.HandleFunc("/fast", handlerFast)
	http.HandleFunc("/slow", handlerSlow)
	http.HandleFunc("/mixed", handlerMixed)

	fmt.Println("listening on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		panic(err)
	}
}
