package embed

import (
	"hash/fnv"
	"math"

	"github.com/VladMinzatu/performance-handbook/doc-pipeline/internal/tokenize"
)

func Hash(data []byte) uint64 {
	// TODO: optimize to sync.Pool with h.Reset() or manual implementation to avoid allocations
	h := fnv.New64a()
	h.Write(data)
	return h.Sum64()
}

type Embedding []float32

type EmbeddedDoc struct {
	ID        string
	Embedding Embedding
}

type Embedder struct {
	dim int
}

func NewEmbedder(dim int) *Embedder {
	return &Embedder{dim: dim}
}

func (e *Embedder) Embed(doc tokenize.TokenizedDoc) (EmbeddedDoc, error) {
	vec := make([]float32, e.dim)

	for _, tok := range doc.Tokens {
		h := Hash([]byte(tok.Term))
		idx := int(h % uint64(e.dim))
		sign := float32(1.0)
		if h&1 == 1 {
			sign = float32(-1.0)
		}
		vec[idx] += sign
	}

	normalize(vec)
	return EmbeddedDoc{
		ID:        doc.ID,
		Embedding: vec,
	}, nil
}

func normalize(vec []float32) {
	sum := float32(0.0)
	for _, v := range vec {
		sum += v * v
	}
	norm := math.Sqrt(float64(sum))
	for i := range vec {
		vec[i] /= float32(norm)
	}
}
