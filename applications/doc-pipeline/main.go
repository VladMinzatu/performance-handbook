package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	_ "net/http/pprof"

	"github.com/VladMinzatu/performance-handbook/doc-pipeline/internal/embed"
	"github.com/VladMinzatu/performance-handbook/doc-pipeline/internal/index"
	"github.com/VladMinzatu/performance-handbook/doc-pipeline/internal/ingest"
	"github.com/VladMinzatu/performance-handbook/doc-pipeline/internal/load"
	"github.com/VladMinzatu/performance-handbook/doc-pipeline/internal/pipeline"
	"github.com/VladMinzatu/performance-handbook/doc-pipeline/internal/telemetry"
	"github.com/VladMinzatu/performance-handbook/doc-pipeline/internal/tokenize"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	telemetryMetrics, err := telemetry.InitMetrics()
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	go func() {
		slog.Info("metrics available at :8080/metrics")
		log.Fatal(http.ListenAndServe(":8080", mux))
	}()

	go func() {
		log.Println("pprof listening on :6060")
		log.Println(http.ListenAndServe(":6060", nil))
	}()

	generatorConfig := load.LoadGeneratorConfig{
		MinTextSize: 1_000,
		MaxTextSize: 20_000,
		IDPrefix:    "doc",
		RatePerSec:  1000,
		FilePath:    "data/shakespeare.txt",
		FileSize:    5436475,
	}
	generator := load.NewLoadGenerator(generatorConfig, 100, telemetryMetrics)
	dataLoadingChan := generator.Run(context.Background())
	ctx := context.Background()

	dataLoadingStage := pipeline.NewStage(
		"load",
		10,
		100,
		dataLoadingChan,
		ingest.LoadData,
		telemetryMetrics,
	)
	tokenizeStage := pipeline.NewStage(
		"tokenize",
		10,
		100,
		dataLoadingStage.Run(ctx),
		tokenize.Tokenize,
		telemetryMetrics,
	)

	embedder := embed.NewEmbedder(1024)
	embedDocStage := pipeline.NewStage(
		"embed",
		10,
		100,
		tokenizeStage.Run(ctx),
		embedder.Embed,
		telemetryMetrics,
	)

	indexer, err := index.NewEmbeddingIndex(0.8, telemetryMetrics)
	if err != nil {
		log.Fatal(err)
	}
	indexStage := pipeline.NewStage(
		"index",
		10,
		100,
		embedDocStage.Run(ctx),
		indexer.DedupAndIndex,
		telemetryMetrics,
	)

	out := indexStage.Run(ctx)
	for result := range out {
		fmt.Println(result)
	}
}
