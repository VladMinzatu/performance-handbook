package load

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/VladMinzatu/performance-handbook/doc-pipeline/internal/ingest"
)

func TestGenerateRandomDataLoadingRequest_BoundsRespected(t *testing.T) {
	config := LoadGeneratorConfig{
		MinTextSize: 1000,
		MaxTextSize: 2000,
		IDPrefix:    "doc",
		RatePerSec:  100,
		FilePath:    "file.txt",
		FileSize:    10_000,
	}

	rng := rand.New(rand.NewSource(42))

	for i := 0; i < 1_000; i++ {
		req := generateRandomDataLoadingRequest(config, i, rng)

		if req.TextSize < config.MinTextSize || req.TextSize > config.MaxTextSize {
			t.Fatalf("TextSize out of bounds: got %d", req.TextSize)
		}

		if req.Offset < 0 {
			t.Fatalf("Offset negative: got %d", req.Offset)
		}

		if req.Offset+req.TextSize > config.FileSize {
			t.Fatalf(
				"Offset + TextSize exceeds FileSize: offset=%d size=%d file=%d",
				req.Offset, req.TextSize, config.FileSize,
			)
		}
	}
}

func TestGenerateRandomDataLoadingRequest_ExactFit(t *testing.T) {
	config := LoadGeneratorConfig{
		MinTextSize: 5000,
		MaxTextSize: 5000,
		IDPrefix:    "doc",
		FilePath:    "file.txt",
		FileSize:    5000,
	}

	rng := rand.New(rand.NewSource(1))
	req := generateRandomDataLoadingRequest(config, 0, rng)

	if req.TextSize != 5000 {
		t.Fatalf("Expected TextSize=5000, got %d", req.TextSize)
	}

	if req.Offset != 0 {
		t.Fatalf("Expected Offset=0 for exact fit, got %d", req.Offset)
	}
}

func TestGenerateRandomDataLoadingRequest_MaxSizeClampedToFileSize(t *testing.T) {
	config := LoadGeneratorConfig{
		MinTextSize: 1000,
		MaxTextSize: 20_000, // larger than file
		IDPrefix:    "doc",
		FilePath:    "file.txt",
		FileSize:    5_000,
	}

	rng := rand.New(rand.NewSource(1))
	req := generateRandomDataLoadingRequest(config, 0, rng)

	if req.TextSize < 1000 || req.TextSize > 5000 {
		t.Fatalf("TextSize not clamped correctly: got %d", req.TextSize)
	}

	if req.Offset+req.TextSize > config.FileSize {
		t.Fatalf("Offset + TextSize exceeds FileSize after clamping")
	}
}

func TestGenerateRandomDataLoadingRequest_MinGreaterThanMax(t *testing.T) {
	config := LoadGeneratorConfig{
		MinTextSize: 3000,
		MaxTextSize: 1000, // invalid
		IDPrefix:    "doc",
		FilePath:    "file.txt",
		FileSize:    10_000,
	}

	rng := rand.New(rand.NewSource(1))
	req := generateRandomDataLoadingRequest(config, 0, rng)

	if req.TextSize != 3000 {
		t.Fatalf("Expected TextSize=3000 when Min > Max, got %d", req.TextSize)
	}

	if req.Offset+req.TextSize > config.FileSize {
		t.Fatalf("Offset + TextSize exceeds FileSize")
	}
}

func TestGenerateRandomDataLoadingRequest_MinGreaterThanFileSize(t *testing.T) {
	config := LoadGeneratorConfig{
		MinTextSize: 20_000, // larger than file
		MaxTextSize: 30_000,
		IDPrefix:    "doc",
		FilePath:    "file.txt",
		FileSize:    5_000,
	}

	rng := rand.New(rand.NewSource(1))
	req := generateRandomDataLoadingRequest(config, 0, rng)

	if req.TextSize != 5_000 {
		t.Fatalf("Expected TextSize clamped to FileSize=5000, got %d", req.TextSize)
	}

	if req.Offset != 0 {
		t.Fatalf("Expected Offset=0 when TextSize == FileSize, got %d", req.Offset)
	}
}

func TestGenerateRandomDataLoadingRequest_IDFormatting(t *testing.T) {
	config := LoadGeneratorConfig{
		MinTextSize: 100,
		MaxTextSize: 200,
		IDPrefix:    "doc",
		FilePath:    "file.txt",
		FileSize:    10_000,
	}

	rng := rand.New(rand.NewSource(1))
	req := generateRandomDataLoadingRequest(config, 7, rng)

	expectedID := "doc-7"
	if req.ID != expectedID {
		t.Fatalf("Expected ID=%q, got %q", expectedID, req.ID)
	}
}

func TestLoadGenerator_RunEmitsRequests(t *testing.T) {
	config := LoadGeneratorConfig{
		MinTextSize: 100,
		MaxTextSize: 200,
		IDPrefix:    "doc",
		RatePerSec:  1000,
		FilePath:    "file.txt",
		FileSize:    10_000,
	}

	gen := NewLoadGenerator(config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	gen.Run(ctx)

	var results []ingest.DataLoadingConfig
	timeout := time.After(100 * time.Millisecond)

	for len(results) < 5 {
		select {
		case req, ok := <-gen.out:
			if !ok {
				t.Fatalf("Output channel closed unexpectedly")
			}
			results = append(results, req)
		case <-timeout:
			t.Fatalf("Timed out waiting for generator output")
		}
	}

	for i, req := range results {
		expectedID := fmt.Sprintf("%s-%d", config.IDPrefix, i)
		if req.ID != expectedID {
			t.Fatalf("Expected ID=%q, got %q", expectedID, req.ID)
		}

		if req.TextSize < config.MinTextSize || req.TextSize > config.MaxTextSize {
			t.Fatalf("TextSize out of bounds: %d", req.TextSize)
		}

		if req.Offset+req.TextSize > config.FileSize {
			t.Fatalf("Offset + TextSize exceeds FileSize")
		}
	}
}
