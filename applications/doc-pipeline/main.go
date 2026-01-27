package main

import (
	"context"
	"sync"

	"github.com/VladMinzatu/performance-handbook/doc-pipeline/internal/embed"
	"github.com/VladMinzatu/performance-handbook/doc-pipeline/internal/index"
	"github.com/VladMinzatu/performance-handbook/doc-pipeline/internal/ingest"
	"github.com/VladMinzatu/performance-handbook/doc-pipeline/internal/load"
	"github.com/VladMinzatu/performance-handbook/doc-pipeline/internal/pipeline"
	"github.com/VladMinzatu/performance-handbook/doc-pipeline/internal/tokenize"
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
	generator := load.NewLoadGenerator(generatorConfig)
	generator.Run(context.Background())

	dataLoadingChan := make(chan ingest.DataLoadingConfig)
	documentChan := make(chan ingest.Document)

	dataLoadingStage := pipeline.Stage[ingest.DataLoadingConfig, ingest.Document]{
		Name:    "load",
		Workers: 10,
		In:      dataLoadingChan,
		Out:     documentChan,
		Fn:      ingest.LoadData,
	}
	dataLoadingStage.Run(context.Background(), &sync.WaitGroup{})

	tokenizedDocChan := make(chan tokenize.TokenizedDoc)
	tokenizeStage := pipeline.Stage[ingest.Document, tokenize.TokenizedDoc]{
		Name:    "tokenize",
		Workers: 10,
		In:      documentChan,
		Out:     tokenizedDocChan,
		Fn:      tokenize.Tokenize,
	}
	tokenizeStage.Run(context.Background(), &sync.WaitGroup{})

	embeddedDocChan := make(chan embed.EmbeddedDoc)
	embedStage := pipeline.Stage[tokenize.TokenizedDoc, embed.EmbeddedDoc]{
		Name:    "embed",
		Workers: 10,
		In:      tokenizedDocChan,
		Out:     embeddedDocChan,
		Fn:      embed.Embed,
	}
	embedStage.Run(context.Background(), &sync.WaitGroup{})

	indexChan := make(chan index.DedupResult)
	indexStage := pipeline.Stage[embed.EmbeddedDoc, index.DedupResult]{
		Name:    "index",
		Workers: 10,
		In:      embeddedDocChan,
		Out:     indexChan,
		Fn:      index.DedupAndIndex,
	}
	indexStage.Run(context.Background(), &sync.WaitGroup{})

}
