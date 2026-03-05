package index

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/VladMinzatu/performance-handbook/doc-pipeline/internal/embed"
	"github.com/coder/hnsw"
)

type IndexMetrics interface {
	SetDeduplicationThreshold(ctx context.Context, threshold float32)
	IncTotalProcessedDocumentsForIndexing(ctx context.Context)
	IncTotalDuplicateDocuments(ctx context.Context)
}

type DedupResult struct {
	ID          string
	IsDuplicate bool
	NearestID   string
	Similarity  float32
}

type EmbeddingIndex struct {
	mu             sync.RWMutex
	graph          *hnsw.Graph[string]
	dedupThreshold float32
	metrics        IndexMetrics
}

func NewEmbeddingIndex(dedupThreshold float32, metrics IndexMetrics) (*EmbeddingIndex, error) {
	if dedupThreshold <= 0.0 || dedupThreshold > 1.0 {
		return nil, errors.New("deduplication threshold must be between 0.0 and 1.0")
	}

	g := hnsw.NewGraph[string]()
	metrics.SetDeduplicationThreshold(context.Background(), dedupThreshold)
	return &EmbeddingIndex{
		graph:          g,
		dedupThreshold: dedupThreshold,
		metrics:        metrics,
	}, nil
}

func (idx *EmbeddingIndex) DedupAndIndex(doc embed.EmbeddedDoc) (DedupResult, error) {
	slog.Debug("Processing document for dedupping and indexing", "id", doc.ID)
	ctx := context.Background()
	idx.metrics.IncTotalProcessedDocumentsForIndexing(ctx)

	var isDup bool
	var bestID string
	var bestScore float32

	var vec hnsw.Vector
	vec = doc.Embedding
	idx.mu.RLock()
	neighbors := idx.graph.SearchWithDistance(vec, 1)
	idx.mu.RUnlock()

	if len(neighbors) > 0 {
		bestID = neighbors[0].Key
		similarity := 1 - neighbors[0].Distance
		isDup = similarity >= idx.dedupThreshold
		bestScore = similarity
		slog.Debug("Is duplicate", "isDup", isDup)

		slog.Debug("Found nearest neighbor", "id", bestID, "similarity", bestScore, "isDup", isDup)
		if !isDup {
			idx.mu.Lock()
			idx.graph.Add(hnsw.Node[string]{Key: doc.ID, Value: doc.Embedding})
			idx.mu.Unlock()
		} else {
			idx.metrics.IncTotalDuplicateDocuments(ctx)
		}
	} else {
		idx.mu.Lock()
		idx.graph.Add(hnsw.Node[string]{Key: doc.ID, Value: doc.Embedding})
		idx.mu.Unlock()
	}

	return DedupResult{
		ID:          doc.ID,
		IsDuplicate: isDup,
		NearestID:   bestID,
		Similarity:  bestScore,
	}, nil
}
