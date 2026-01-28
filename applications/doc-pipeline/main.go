package main

import (
	"context"
	"fmt"
	"sync"

	"github.com/VladMinzatu/performance-handbook/doc-pipeline/internal/ingest"
	"github.com/VladMinzatu/performance-handbook/doc-pipeline/internal/load"
	"github.com/VladMinzatu/performance-handbook/doc-pipeline/internal/pipeline"
)

func main() {
	generatorConfig := load.LoadGeneratorConfig{
		MinTextSize: 1_000,
		MaxTextSize: 20_000,
		IDPrefix:    "doc",
		RatePerSec:  100,
		FilePath:    "data/shakespeare.txt",
		FileSize:    5436475,
	}
	dataLoadingChan := make(chan ingest.DataLoadingConfig)
	generator := load.NewLoadGenerator(generatorConfig, dataLoadingChan)
	generator.Run(context.Background())

	documentChan := make(chan ingest.Document)
	dataLoadingStage := pipeline.Stage[ingest.DataLoadingConfig, ingest.Document]{
		Name:    "load",
		Workers: 10,
		In:      dataLoadingChan,
		Out:     documentChan,
		Fn:      ingest.LoadData,
	}
	dataLoadingStage.Run(context.Background(), &sync.WaitGroup{})

	for doc := range documentChan {
		fmt.Println(doc.ID)
	}

	// tokenizedDocChan := make(chan tokenize.TokenizedDoc)
	// tokenizeStage := pipeline.Stage[ingest.Document, tokenize.TokenizedDoc]{
	// 	Name:    "tokenize",
	// 	Workers: 10,
	// 	In:      documentChan,
	// 	Out:     tokenizedDocChan,
	// 	Fn:      tokenize.Tokenize,
	// }
	// tokenizeStage.Run(context.Background(), &sync.WaitGroup{})

	// embeddedDocChan := make(chan embed.EmbeddedDoc)
	// embedder := embed.NewEmbedder(1024)
	// embedStage := pipeline.Stage[tokenize.TokenizedDoc, embed.EmbeddedDoc]{
	// 	Name:    "embed",
	// 	Workers: 10,
	// 	In:      tokenizedDocChan,
	// 	Out:     embeddedDocChan,
	// 	Fn:      embedder.Embed,
	// }
	// embedStage.Run(context.Background(), &sync.WaitGroup{})

	// indexChan := make(chan index.DedupResult)
	// idx := index.NewEmbeddingIndex(0.8)
	// indexStage := pipeline.Stage[embed.EmbeddedDoc, index.DedupResult]{
	// 	Name:    "index",
	// 	Workers: 10,
	// 	In:      embeddedDocChan,
	// 	Out:     indexChan,
	// 	Fn:      idx.DedupAndIndex,
	// }
	// indexStage.Run(context.Background(), &sync.WaitGroup{})

}
