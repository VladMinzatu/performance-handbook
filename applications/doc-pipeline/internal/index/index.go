package index

import (
	"sync"

	"github.com/VladMinzatu/performance-handbook/doc-pipeline/internal/embed"
)

const (
	DefaultCapacity = 1024
)

type DedupResult struct {
	ID          string
	IsDuplicate bool
	NearestID   string
	Similarity  float64
}

type EmbeddingIndex struct {
	mu   sync.RWMutex
	ids  []string
	vecs [][]float64
}

func NewEmbeddingIndex() *EmbeddingIndex {
	return &EmbeddingIndex{
		ids:  make([]string, 0, DefaultCapacity),
		vecs: make([][]float64, 0, DefaultCapacity),
	}
}

func DedupAndIndex(idx *EmbeddingIndex, doc embed.EmbeddedDoc, threshold float64) (DedupResult, error) {
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

	isDup := bestScore >= threshold

	if !isDup {
		idx.mu.Lock()
		idx.ids = append(idx.ids, doc.ID)
		idx.vecs = append(idx.vecs, doc.Embedding)
		idx.mu.Unlock()
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
