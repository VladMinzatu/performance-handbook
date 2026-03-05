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
	mu             sync.Mutex
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

	idx.mu.Lock()
	defer idx.mu.Unlock() // TODO: rw lock for more granular locking

	var isDup bool
	var bestID string
	var bestScore float32

	var vec hnsw.Vector
	vec = doc.Embedding
	neighbors := idx.graph.SearchWithDistance(vec, 1)
	if len(neighbors) > 0 {
		bestID = neighbors[0].Key
		similarity := 1 - neighbors[0].Distance
		isDup = similarity >= idx.dedupThreshold
		bestScore = similarity
		slog.Debug("Is duplicate", "isDup", isDup)

		slog.Debug("Found nearest neighbor", "id", bestID, "similarity", bestScore, "isDup", isDup)
		if !isDup {
			idx.graph.Add(hnsw.Node[string]{Key: doc.ID, Value: doc.Embedding})
		} else {
			idx.metrics.IncTotalDuplicateDocuments(ctx)
		}
	} else {
		idx.graph.Add(hnsw.Node[string]{Key: doc.ID, Value: doc.Embedding})
	}

	return DedupResult{
		ID:          doc.ID,
		IsDuplicate: isDup,
		NearestID:   bestID,
		Similarity:  bestScore,
	}, nil
}
