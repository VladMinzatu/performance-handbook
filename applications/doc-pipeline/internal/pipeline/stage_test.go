package pipeline

import (
	"context"
	"testing"
	"time"
)

type TestStageMetrics struct {
	recordedLatencies []time.Duration
}

func (t *TestStageMetrics) RecordProcessingLatency(ctx context.Context, latency time.Duration) {
	t.recordedLatencies = append(t.recordedLatencies, latency)
}

func TestStage_BasicProcessing(t *testing.T) {
	metrics := &TestStageMetrics{}
	ctx := context.Background()
	in := make(chan int, 10)

	stage := NewStage(
		"multiply",
		1,
		10,
		in,
		func(in int) (int, error) {
			return in * 2, nil
		},
		metrics,
	)

	out := stage.Run(ctx)

	testInputs := []int{1, 2, 3, 4, 5}
	for _, v := range testInputs {
		in <- v
	}
	close(in)

	var results []int
	done := make(chan bool)
	go func() {
		for result := range out {
			results = append(results, result)
			if len(results) == len(testInputs) {
				done <- true
				return
			}
		}
		done <- true
	}()

	select {
	case <-done:
		// All results received
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for results")
	}

	expected := []int{2, 4, 6, 8, 10}
	if len(results) != len(expected) {
		t.Fatalf("expected %d results, got %d", len(expected), len(results))
	}
	if len(metrics.recordedLatencies) != len(testInputs) {
		t.Errorf("expected %d latencies, got %d", len(testInputs), len(metrics.recordedLatencies))
	}

	resultMap := make(map[int]int)
	for _, r := range results {
		resultMap[r]++
	}

	for _, exp := range expected {
		if resultMap[exp] != 1 {
			t.Errorf("expected result %d to appear once, got %d", exp, resultMap[exp])
		}
	}
}

func TestStage_MultipleWorkers(t *testing.T) {
	ctx := context.Background()
	in := make(chan int, 20)
	metrics := &TestStageMetrics{}

	stage := NewStage(
		"multiply",
		3,
		20,
		in,
		func(in int) (int, error) {
			// Simulate some work
			time.Sleep(10 * time.Millisecond)
			return in * 2, nil
		},
		metrics,
	)

	out := stage.Run(ctx)

	testInputs := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	for _, v := range testInputs {
		in <- v
	}
	close(in)

	var results []int
	done := make(chan bool)
	go func() {
		for result := range out {
			results = append(results, result)
			if len(results) == len(testInputs) {
				done <- true
				return
			}
		}
		done <- true
	}()

	select {
	case <-done:
		// All results received
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for results")
	}

	if len(results) != len(testInputs) {
		t.Fatalf("expected %d results, got %d", len(testInputs), len(results))
	}

	if len(metrics.recordedLatencies) != len(testInputs) {
		t.Errorf("expected %d latencies, got %d", len(testInputs), len(metrics.recordedLatencies))
	}

	resultMap := make(map[int]int)
	for _, r := range results {
		resultMap[r]++
	}

	expected := []int{2, 4, 6, 8, 10, 12, 14, 16, 18, 20}
	for _, exp := range expected {
		if resultMap[exp] != 1 {
			t.Errorf("expected result %d to appear once, got %d", exp, resultMap[exp])
		}
	}
}

func TestStage_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	in := make(chan int, 10)
	defer close(in)

	metrics := &TestStageMetrics{}
	stage := NewStage(
		"slow",
		2,
		20,
		in,
		func(in int) (int, error) {
			time.Sleep(100 * time.Millisecond)
			return in * 2, nil
		},
		metrics,
	)

	out := stage.Run(ctx)

	in <- 1
	in <- 2

	cancel() // cancel immediately

	select {
	case <-out:
		// Worker received a value
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for worker to receive a value")
	}
	if len(metrics.recordedLatencies) != 0 {
		t.Errorf("expected %d latencies, got %d", 0, len(metrics.recordedLatencies))
	}
}

func TestStage_ErrorHandling(t *testing.T) {
	ctx := context.Background()
	in := make(chan int, 10)
	metrics := &TestStageMetrics{}

	callCount := 0
	stage := NewStage(
		"error-handling",
		1,
		20,
		in,
		func(in int) (int, error) {
			callCount++
			if in%2 == 0 {
				return 0, &testError{msg: "even number error"}
			}
			return in * 2, nil
		},
		metrics,
	)

	out := stage.Run(ctx)

	testInputs := []int{1, 2, 3, 4, 5}
	for _, v := range testInputs {
		in <- v
	}
	close(in)

	var results []int
	done := make(chan bool)
	go func() {
		for result := range out {
			results = append(results, result)
		}
		done <- true
	}()

	select {
	case <-done:
		// Processing complete
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for processing")
	}

	expectedResults := []int{2, 6, 10}
	if len(results) != len(expectedResults) {
		t.Fatalf("expected %d results, got %d", len(expectedResults), len(results))
	}

	if len(metrics.recordedLatencies) != len(expectedResults) {
		t.Errorf("expected %d latencies, got %d", len(testInputs), len(metrics.recordedLatencies))
	}

	resultMap := make(map[int]int)
	for _, r := range results {
		resultMap[r]++
	}

	for _, exp := range expectedResults {
		if resultMap[exp] != 1 {
			t.Errorf("expected result %d to appear once, got %d", exp, resultMap[exp])
		}
	}

	if callCount != len(testInputs) {
		t.Errorf("expected function to be called %d times, got %d", len(testInputs), callCount)
	}
}

func TestStage_ChannelClosure(t *testing.T) {
	ctx := context.Background()
	in := make(chan int, 10)

	metrics := &TestStageMetrics{}
	stage := NewStage(
		"closure",
		2,
		20,
		in,
		func(in int) (int, error) {
			return in * 2, nil
		},
		metrics,
	)

	out := stage.Run(ctx)

	in <- 1
	in <- 2
	close(in)

	var results []int
	done := make(chan bool)
	go func() {
		for result := range out {
			results = append(results, result)
		}
		done <- true
	}()

	select {
	case <-done:
		// Processing complete
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for processing")
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if len(metrics.recordedLatencies) != 2 {
		t.Errorf("expected %d latencies, got %d", 2, len(metrics.recordedLatencies))
	}
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
