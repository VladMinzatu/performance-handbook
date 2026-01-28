package index

import (
	"math"
	"sync"
	"testing"

	"github.com/VladMinzatu/performance-handbook/doc-pipeline/internal/embed"
)

func TestNewEmbeddingIndex(t *testing.T) {
	idx := NewEmbeddingIndex(0.8)

	if idx == nil {
		t.Fatal("NewEmbeddingIndex returned nil")
	}

	if len(idx.ids) != 0 {
		t.Errorf("expected empty ids, got length %d", len(idx.ids))
	}

	if len(idx.vecs) != 0 {
		t.Errorf("expected empty vecs, got length %d", len(idx.vecs))
	}
}

func TestDedupAndIndex_EmptyIndex(t *testing.T) {
	idx := NewEmbeddingIndex(0.8)
	doc := embed.EmbeddedDoc{
		ID:        "doc-1",
		Embedding: createEmbedding(10, []int{0}),
	}

	result, err := idx.DedupAndIndex(doc)
	if err != nil {
		t.Fatalf("DedupAndIndex failed: %v", err)
	}

	if result.ID != "doc-1" {
		t.Errorf("expected ID 'doc-1', got '%s'", result.ID)
	}

	if result.IsDuplicate {
		t.Error("expected not duplicate for empty index")
	}

	if result.NearestID != "" {
		t.Errorf("expected empty nearest ID, got '%s'", result.NearestID)
	}

	if result.Similarity != 0.0 {
		t.Errorf("expected similarity 0.0, got %f", result.Similarity)
	}

	if len(idx.ids) != 1 {
		t.Errorf("expected 1 document in index, got %d", len(idx.ids))
	}

	if idx.ids[0] != "doc-1" {
		t.Errorf("expected 'doc-1' in index, got '%s'", idx.ids[0])
	}
}

func TestDedupAndIndex_NonDuplicate(t *testing.T) {
	idx := NewEmbeddingIndex(0.8)

	doc1 := embed.EmbeddedDoc{
		ID:        "doc-1",
		Embedding: createEmbedding(10, []int{0}),
	}

	doc2 := embed.EmbeddedDoc{
		ID:        "doc-2",
		Embedding: createEmbedding(10, []int{5}),
	}

	_, err := idx.DedupAndIndex(doc1)
	if err != nil {
		t.Fatalf("DedupAndIndex failed: %v", err)
	}

	result, err := idx.DedupAndIndex(doc2)
	if err != nil {
		t.Fatalf("DedupAndIndex failed: %v", err)
	}

	if result.IsDuplicate {
		t.Error("expected not duplicate for different embeddings")
	}

	if len(idx.ids) != 2 {
		t.Errorf("expected 2 documents in index, got %d", len(idx.ids))
	}
}

func TestDedupAndIndex_Duplicate(t *testing.T) {
	idx := NewEmbeddingIndex(0.8)

	doc1 := embed.EmbeddedDoc{
		ID:        "doc-1",
		Embedding: createEmbedding(10, []int{0}),
	}

	doc2 := embed.EmbeddedDoc{
		ID:        "doc-2",
		Embedding: createEmbedding(10, []int{0}),
	}

	_, err := idx.DedupAndIndex(doc1)
	if err != nil {
		t.Fatalf("DedupAndIndex failed: %v", err)
	}

	result, err := idx.DedupAndIndex(doc2)
	if err != nil {
		t.Fatalf("DedupAndIndex failed: %v", err)
	}

	if !result.IsDuplicate {
		t.Error("expected duplicate for identical embeddings")
	}

	if result.NearestID != "doc-1" {
		t.Errorf("expected nearest ID 'doc-1', got '%s'", result.NearestID)
	}

	if result.Similarity < 0.99 {
		t.Errorf("expected high similarity for identical embeddings, got %f", result.Similarity)
	}

	if len(idx.ids) != 1 {
		t.Errorf("expected 1 document in index (duplicate not added), got %d", len(idx.ids))
	}
}

func TestDedupAndIndex_ThresholdBoundary(t *testing.T) {
	idx := NewEmbeddingIndex(0.8)

	doc1 := embed.EmbeddedDoc{
		ID:        "doc-1",
		Embedding: createEmbedding(10, []int{0}),
	}

	doc2 := embed.EmbeddedDoc{
		ID:        "doc-2",
		Embedding: createEmbedding(10, []int{0}),
	}

	_, err := idx.DedupAndIndex(doc1)
	if err != nil {
		t.Fatalf("DedupAndIndex failed: %v", err)
	}

	result, err := idx.DedupAndIndex(doc2)
	if err != nil {
		t.Fatalf("DedupAndIndex failed: %v", err)
	}

	if result.Similarity >= 0.99 {
		if !result.IsDuplicate {
			t.Error("expected duplicate when similarity >= threshold")
		}
	} else {
		if result.IsDuplicate {
			t.Error("expected not duplicate when similarity < threshold")
		}
	}
}

func TestDedupAndIndex_MultipleDocuments(t *testing.T) {
	idx := NewEmbeddingIndex(0.8)

	docs := []embed.EmbeddedDoc{
		{ID: "doc-1", Embedding: createEmbedding(10, []int{0})},
		{ID: "doc-2", Embedding: createEmbedding(10, []int{1})},
		{ID: "doc-3", Embedding: createEmbedding(10, []int{2})},
		{ID: "doc-4", Embedding: createEmbedding(10, []int{3})},
		{ID: "doc-5", Embedding: createEmbedding(10, []int{4})},
	}

	for i, doc := range docs {
		result, err := idx.DedupAndIndex(doc)
		if err != nil {
			t.Fatalf("DedupAndIndex failed for doc-%d: %v", i+1, err)
		}

		if result.IsDuplicate {
			t.Errorf("doc-%d should not be duplicate", i+1)
		}

		if len(idx.ids) != i+1 {
			t.Errorf("expected %d documents in index, got %d", i+1, len(idx.ids))
		}
	}
}

func TestDedupAndIndex_FindNearest(t *testing.T) {
	idx := NewEmbeddingIndex(0.8)

	doc1 := embed.EmbeddedDoc{
		ID:        "doc-1",
		Embedding: createEmbedding(10, []int{0}),
	}

	doc2 := embed.EmbeddedDoc{
		ID:        "doc-2",
		Embedding: createEmbedding(10, []int{5}),
	}

	doc3 := embed.EmbeddedDoc{
		ID:        "doc-3",
		Embedding: createEmbedding(10, []int{0}),
	}

	_, err := idx.DedupAndIndex(doc1)
	if err != nil {
		t.Fatalf("DedupAndIndex failed: %v", err)
	}

	_, err = idx.DedupAndIndex(doc2)
	if err != nil {
		t.Fatalf("DedupAndIndex failed: %v", err)
	}

	result, err := idx.DedupAndIndex(doc3)
	if err != nil {
		t.Fatalf("DedupAndIndex failed: %v", err)
	}

	if result.NearestID != "doc-1" {
		t.Errorf("expected nearest ID 'doc-1', got '%s'", result.NearestID)
	}

	if result.Similarity < 0.99 {
		t.Errorf("expected high similarity with doc-1, got %f", result.Similarity)
	}
}

func TestDedupAndIndex_ConcurrentAccess(t *testing.T) {
	idx := NewEmbeddingIndex(0.8)
	var wg sync.WaitGroup
	numDocs := 100

	for i := 0; i < numDocs; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			doc := embed.EmbeddedDoc{
				ID:        string(rune('a' + id%26)),
				Embedding: createEmbedding(10, []int{id}),
			}
			_, err := idx.DedupAndIndex(doc)
			if err != nil {
				t.Errorf("DedupAndIndex failed: %v", err)
			}
		}(i)
	}

	wg.Wait()

	if len(idx.ids) != numDocs {
		t.Errorf("expected %d documents in index, got %d", numDocs, len(idx.ids))
	}
}

func TestDedupAndIndex_ConcurrentDuplicateDetection(t *testing.T) {
	idx := NewEmbeddingIndex(0.8)

	doc1 := embed.EmbeddedDoc{
		ID:        "doc-1",
		Embedding: createEmbedding(10, []int{0}),
	}

	_, err := idx.DedupAndIndex(doc1)
	if err != nil {
		t.Fatalf("DedupAndIndex failed: %v", err)
	}

	var wg sync.WaitGroup
	numConcurrent := 10
	duplicates := 0
	var mu sync.Mutex

	for i := 0; i < numConcurrent; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			doc := embed.EmbeddedDoc{
				ID:        "duplicate",
				Embedding: createEmbedding(10, []int{0}),
			}
			result, err := idx.DedupAndIndex(doc)
			if err != nil {
				t.Errorf("DedupAndIndex failed: %v", err)
				return
			}
			if result.IsDuplicate {
				mu.Lock()
				duplicates++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	if duplicates == 0 {
		t.Error("expected at least one duplicate detection")
	}

	if len(idx.ids) > 2 {
		t.Errorf("expected at most 2 documents in index (original + maybe one duplicate), got %d", len(idx.ids))
	}
}

func TestDedupAndIndex_VerySimilar(t *testing.T) {
	idx := NewEmbeddingIndex(0.8)

	doc1 := embed.EmbeddedDoc{
		ID:        "doc-1",
		Embedding: createEmbedding(100, []int{0, 1, 2}),
	}

	doc2 := embed.EmbeddedDoc{
		ID:        "doc-2",
		Embedding: createEmbedding(100, []int{0, 1, 2, 3}),
	}

	_, err := idx.DedupAndIndex(doc1)
	if err != nil {
		t.Fatalf("DedupAndIndex failed: %v", err)
	}

	result, err := idx.DedupAndIndex(doc2)
	if err != nil {
		t.Fatalf("DedupAndIndex failed: %v", err)
	}

	if result.Similarity < 0.0 || result.Similarity > 1.0 {
		t.Errorf("similarity should be between 0 and 1, got %f", result.Similarity)
	}
}

func TestDedupAndIndex_VeryDifferent(t *testing.T) {
	idx := NewEmbeddingIndex(0.8)

	doc1 := embed.EmbeddedDoc{
		ID:        "doc-1",
		Embedding: createEmbedding(10, []int{0}),
	}

	doc2 := embed.EmbeddedDoc{
		ID:        "doc-2",
		Embedding: createEmbedding(10, []int{9}),
	}

	_, err := idx.DedupAndIndex(doc1)
	if err != nil {
		t.Fatalf("DedupAndIndex failed: %v", err)
	}

	result, err := idx.DedupAndIndex(doc2)
	if err != nil {
		t.Fatalf("DedupAndIndex failed: %v", err)
	}

	if result.IsDuplicate {
		t.Error("expected not duplicate for very different embeddings")
	}

	if result.Similarity >= 0.8 {
		t.Errorf("expected low similarity, got %f", result.Similarity)
	}
}

func TestDedupAndIndex_ZeroThreshold(t *testing.T) {
	idx := NewEmbeddingIndex(0.0)

	doc1 := embed.EmbeddedDoc{
		ID:        "doc-1",
		Embedding: createEmbedding(10, []int{0}),
	}

	doc2 := embed.EmbeddedDoc{
		ID:        "doc-2",
		Embedding: createEmbedding(10, []int{9}),
	}

	_, err := idx.DedupAndIndex(doc1)
	if err != nil {
		t.Fatalf("DedupAndIndex failed: %v", err)
	}

	result, err := idx.DedupAndIndex(doc2)
	if err != nil {
		t.Fatalf("DedupAndIndex failed: %v", err)
	}

	if result.Similarity >= 0.0 {
		if !result.IsDuplicate {
			t.Error("expected duplicate when threshold is 0.0 and similarity >= 0")
		}
	}
}

func TestDedupAndIndex_OneThreshold(t *testing.T) {
	idx := NewEmbeddingIndex(1.0)

	doc1 := embed.EmbeddedDoc{
		ID:        "doc-1",
		Embedding: createEmbedding(10, []int{0}),
	}

	doc2 := embed.EmbeddedDoc{
		ID:        "doc-2",
		Embedding: createEmbedding(10, []int{0}),
	}

	_, err := idx.DedupAndIndex(doc1)
	if err != nil {
		t.Fatalf("DedupAndIndex failed: %v", err)
	}

	result, err := idx.DedupAndIndex(doc2)
	if err != nil {
		t.Fatalf("DedupAndIndex failed: %v", err)
	}

	if result.Similarity >= 1.0 {
		if !result.IsDuplicate {
			t.Error("expected duplicate when similarity >= 1.0")
		}
	} else {
		if result.IsDuplicate {
			t.Error("expected not duplicate when similarity < 1.0")
		}
	}
}

func TestDedupAndIndex_LargeEmbedding(t *testing.T) {
	idx := NewEmbeddingIndex(0.8)

	doc := embed.EmbeddedDoc{
		ID:        "doc-1",
		Embedding: createEmbedding(1000, []int{0, 100, 500, 999}),
	}

	result, err := idx.DedupAndIndex(doc)
	if err != nil {
		t.Fatalf("DedupAndIndex failed: %v", err)
	}

	if result.IsDuplicate {
		t.Error("expected not duplicate for first document")
	}

	if len(idx.ids) != 1 {
		t.Errorf("expected 1 document in index, got %d", len(idx.ids))
	}
}

func createEmbedding(dim int, nonZeroIndices []int) embed.Embedding {
	vec := make([]float64, dim)
	for _, idx := range nonZeroIndices {
		if idx < dim {
			vec[idx] = 1.0
		}
	}

	sum := 0.0
	for _, v := range vec {
		sum += v * v
	}
	norm := math.Sqrt(sum)
	if norm > 0 {
		for i := range vec {
			vec[i] /= norm
		}
	}

	return vec
}
