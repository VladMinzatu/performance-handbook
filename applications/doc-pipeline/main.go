package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

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
	_, err := telemetry.InitMetrics()
	if err != nil {
		log.Fatal(err)
	}
	http.Handle("/metrics", promhttp.Handler())

	generatorConfig := load.LoadGeneratorConfig{
		MinTextSize: 1_000,
		MaxTextSize: 20_000,
		IDPrefix:    "doc",
		RatePerSec:  100,
		FilePath:    "data/shakespeare.txt",
		FileSize:    5436475,
	}
	generator := load.NewLoadGenerator(generatorConfig, 100)
	dataLoadingChan := generator.Run(context.Background())
	ctx := context.Background()

	dataLoadingStage := pipeline.NewStage(
		"load",
		10,
		100,
		dataLoadingChan,
		ingest.LoadData,
	)
	tokenizeStage := pipeline.NewStage(
		"tokenize",
		10,
		100,
		dataLoadingStage.Run(ctx),
		tokenize.Tokenize,
	)

	embedder := embed.NewEmbedder(1024)
	embedDocStage := pipeline.NewStage(
		"embed",
		10,
		100,
		tokenizeStage.Run(ctx),
		embedder.Embed,
	)

	indexer := index.NewEmbeddingIndex(0.8)
	indexStage := pipeline.NewStage(
		"index",
		10,
		100,
		embedDocStage.Run(ctx),
		indexer.DedupAndIndex,
	)

	out := indexStage.Run(ctx)
	for result := range out {
		fmt.Println(result)
	}
}
