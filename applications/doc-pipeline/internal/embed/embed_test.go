package embed

import (
	"math"
	"testing"

	"github.com/VladMinzatu/performance-handbook/doc-pipeline/internal/tokenize"
)

func TestHash_Consistency(t *testing.T) {
	data := []byte("test")
	hash1 := Hash(data)
	hash2 := Hash(data)

	if hash1 != hash2 {
		t.Errorf("Hash should be deterministic, got %d and %d", hash1, hash2)
	}
}

func TestHash_DifferentInputs(t *testing.T) {
	hash1 := Hash([]byte("test1"))
	hash2 := Hash([]byte("test2"))

	if hash1 == hash2 {
		t.Error("Different inputs should produce different hashes")
	}
}

func TestHash_EmptyInput(t *testing.T) {
	hash := Hash([]byte{})
	if hash == 0 {
		t.Error("Empty input should produce non-zero hash")
	}
}

func TestEmbed_EmptyTokens(t *testing.T) {
	doc := tokenize.TokenizedDoc{
		ID:     "doc-1",
		Tokens: []tokenize.Token{},
	}

	result := Embed(doc, 10)

	if result.ID != "doc-1" {
		t.Errorf("expected ID 'doc-1', got '%s'", result.ID)
	}

	if len(result.Embedding) != 10 {
		t.Errorf("expected embedding length 10, got %d", len(result.Embedding))
	}

	for i, v := range result.Embedding {
		if !math.IsNaN(v) && v != 0.0 {
			t.Errorf("expected zero or NaN at index %d, got %f", i, v)
		}
	}
}

func TestEmbed_SingleToken(t *testing.T) {
	doc := tokenize.TokenizedDoc{
		ID: "doc-2",
		Tokens: []tokenize.Token{
			{Term: "hello"},
		},
	}

	result := Embed(doc, 10)

	if result.ID != "doc-2" {
		t.Errorf("expected ID 'doc-2', got '%s'", result.ID)
	}

	if len(result.Embedding) != 10 {
		t.Errorf("expected embedding length 10, got %d", len(result.Embedding))
	}

	assertNormalized(t, result.Embedding)
	assertNonZeroSum(t, result.Embedding)
}

func TestEmbed_MultipleTokens(t *testing.T) {
	doc := tokenize.TokenizedDoc{
		ID: "doc-3",
		Tokens: []tokenize.Token{
			{Term: "hello"},
			{Term: "world"},
			{Term: "test"},
		},
	}

	result := Embed(doc, 20)

	if result.ID != "doc-3" {
		t.Errorf("expected ID 'doc-3', got '%s'", result.ID)
	}

	if len(result.Embedding) != 20 {
		t.Errorf("expected embedding length 20, got %d", len(result.Embedding))
	}

	assertNormalized(t, result.Embedding)
	assertNonZeroSum(t, result.Embedding)
}

func TestEmbed_DifferentDimensions(t *testing.T) {
	doc := tokenize.TokenizedDoc{
		ID: "doc-4",
		Tokens: []tokenize.Token{
			{Term: "test"},
		},
	}

	dims := []int{5, 10, 50, 100, 256}
	for _, dim := range dims {
		result := Embed(doc, dim)
		if len(result.Embedding) != dim {
			t.Errorf("expected embedding length %d, got %d", dim, len(result.Embedding))
		}
		assertNormalized(t, result.Embedding)
	}
}

func TestEmbed_Deterministic(t *testing.T) {
	doc := tokenize.TokenizedDoc{
		ID: "doc-5",
		Tokens: []tokenize.Token{
			{Term: "hello"},
			{Term: "world"},
		},
	}

	result1 := Embed(doc, 10)
	result2 := Embed(doc, 10)

	if result1.ID != result2.ID {
		t.Errorf("expected same ID, got '%s' and '%s'", result1.ID, result2.ID)
	}

	if len(result1.Embedding) != len(result2.Embedding) {
		t.Fatal("results should have same length")
	}

	for i := range result1.Embedding {
		if math.Abs(result1.Embedding[i]-result2.Embedding[i]) > 1e-10 {
			t.Errorf("results should be identical at index %d: got %f and %f", i, result1.Embedding[i], result2.Embedding[i])
		}
	}
}

func TestEmbed_DifferentTokensProduceDifferentEmbeddings(t *testing.T) {
	doc1 := tokenize.TokenizedDoc{
		ID: "doc-6",
		Tokens: []tokenize.Token{
			{Term: "hello"},
		},
	}

	doc2 := tokenize.TokenizedDoc{
		ID: "doc-7",
		Tokens: []tokenize.Token{
			{Term: "world"},
		},
	}

	result1 := Embed(doc1, 10)
	result2 := Embed(doc2, 10)

	assertNormalized(t, result1.Embedding)
	assertNormalized(t, result2.Embedding)

	identical := true
	for i := range result1.Embedding {
		if math.Abs(result1.Embedding[i]-result2.Embedding[i]) > 1e-10 {
			identical = false
			break
		}
	}

	if identical {
		t.Error("different tokens should produce different embeddings")
	}
}

func TestEmbed_TokenCollision(t *testing.T) {
	doc := tokenize.TokenizedDoc{
		ID: "doc-8",
		Tokens: []tokenize.Token{
			{Term: "a"},
			{Term: "b"},
			{Term: "c"},
		},
	}

	result := Embed(doc, 2)

	if len(result.Embedding) != 2 {
		t.Errorf("expected embedding length 2, got %d", len(result.Embedding))
	}

	assertNormalized(t, result.Embedding)
}

func TestEmbed_LargeDimension(t *testing.T) {
	doc := tokenize.TokenizedDoc{
		ID: "doc-9",
		Tokens: []tokenize.Token{
			{Term: "test"},
		},
	}

	result := Embed(doc, 1000)

	if len(result.Embedding) != 1000 {
		t.Errorf("expected embedding length 1000, got %d", len(result.Embedding))
	}

	assertNormalized(t, result.Embedding)
}

func TestEmbed_ManyTokens(t *testing.T) {
	tokens := make([]tokenize.Token, 100)
	for i := range tokens {
		tokens[i] = tokenize.Token{Term: string(rune('a' + i%26))}
	}

	doc := tokenize.TokenizedDoc{
		ID:     "doc-10",
		Tokens: tokens,
	}

	result := Embed(doc, 50)

	if len(result.Embedding) != 50 {
		t.Errorf("expected embedding length 50, got %d", len(result.Embedding))
	}

	assertNormalized(t, result.Embedding)
}

func TestEmbed_RepeatedTokens(t *testing.T) {
	doc := tokenize.TokenizedDoc{
		ID: "doc-11",
		Tokens: []tokenize.Token{
			{Term: "hello"},
			{Term: "hello"},
			{Term: "hello"},
		},
	}

	result := Embed(doc, 10)

	if len(result.Embedding) != 10 {
		t.Errorf("expected embedding length 10, got %d", len(result.Embedding))
	}

	assertNormalized(t, result.Embedding)
}

func TestEmbed_UnicodeTokens(t *testing.T) {
	doc := tokenize.TokenizedDoc{
		ID: "doc-12",
		Tokens: []tokenize.Token{
			{Term: "café"},
			{Term: "résumé"},
			{Term: "naïve"},
		},
	}

	result := Embed(doc, 20)

	if len(result.Embedding) != 20 {
		t.Errorf("expected embedding length 20, got %d", len(result.Embedding))
	}

	assertNormalized(t, result.Embedding)
}

func assertNormalized(t *testing.T, vec Embedding) {
	t.Helper()

	if len(vec) == 0 {
		return
	}

	sum := 0.0
	for _, v := range vec {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			continue
		}
		sum += v * v
	}

	norm := math.Sqrt(sum)

	if norm == 0 {
		return
	}

	if math.Abs(norm-1.0) > 1e-10 {
		t.Errorf("vector should be normalized (norm=1.0), got norm=%f", norm)
	}
}

func assertNonZeroSum(t *testing.T, vec Embedding) {
	t.Helper()

	hasNonZero := false
	for _, v := range vec {
		if math.Abs(v) > 1e-10 {
			hasNonZero = true
			break
		}
	}

	if !hasNonZero {
		t.Error("embedding should have at least one non-zero value")
	}
}
