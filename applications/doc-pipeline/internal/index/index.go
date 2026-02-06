package index

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/VladMinzatu/performance-handbook/doc-pipeline/internal/embed"
)

const (
	DefaultCapacity = 1024
)

type IndexMetrics interface {
	SetDeduplicationThreshold(ctx context.Context, threshold float64)
	IncTotalProcessedDocumentsForIndexing(ctx context.Context)
	IncTotalDuplicateDocuments(ctx context.Context)
}

type DedupResult struct {
	ID          string
	IsDuplicate bool
	NearestID   string
	Similarity  float64
}

type EmbeddingIndex struct {
	mu             sync.RWMutex
	ids            []string
	vecs           [][]float64
	dedupThreshold float64
	metrics        IndexMetrics
}

func NewEmbeddingIndex(dedupThreshold float64, metrics IndexMetrics) (*EmbeddingIndex, error) {
	if dedupThreshold <= 0.0 || dedupThreshold > 1.0 {
		return nil, errors.New("deduplication threshold must be between 0.0 and 1.0")
	}

	metrics.SetDeduplicationThreshold(context.Background(), dedupThreshold)
	return &EmbeddingIndex{
		ids:            make([]string, 0, DefaultCapacity),
		vecs:           make([][]float64, 0, DefaultCapacity),
		dedupThreshold: dedupThreshold,
		metrics:        metrics,
	}, nil
}

func (idx *EmbeddingIndex) DedupAndIndex(doc embed.EmbeddedDoc) (DedupResult, error) {
	slog.Debug("Processing document for dedupping and indexing", "id", doc.ID)
	idx.metrics.IncTotalProcessedDocumentsForIndexing(context.Background())
	// snapshot
	idx.mu.RLock()
	n := len(idx.ids)
	ids := make([]string, n)
	vecs := make([][]float64, n)
	copy(ids, idx.ids)
	copy(vecs, idx.vecs)
	idx.mu.RUnlock()

	// find nearest neighbor
	var bestID string
	var bestScore float64

	for i, v := range vecs {
		s := cosine(doc.Embedding, v)
		if s > bestScore {
			bestScore = s
			bestID = ids[i]
		}
	}

	slog.Debug("Found nearest neighbor", "id", bestID, "similarity", bestScore)
	isDup := bestScore >= idx.dedupThreshold
	slog.Debug("Is duplicate", "isDup", isDup)

	if !isDup {
		idx.mu.Lock()
		idx.ids = append(idx.ids, doc.ID)
		idx.vecs = append(idx.vecs, doc.Embedding)
		idx.mu.Unlock()
	} else {
		idx.metrics.IncTotalDuplicateDocuments(context.Background())
	}

	return DedupResult{
		ID:          doc.ID,
		IsDuplicate: isDup,
		NearestID:   bestID,
		Similarity:  bestScore,
	}, nil
}

func cosine(a, b []float64) float64 {
	var sum float64
	for i := range a {
		sum += a[i] * b[i]
	}
	return sum
}
