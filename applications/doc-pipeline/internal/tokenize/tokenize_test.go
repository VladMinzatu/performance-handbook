package tokenize

import (
	"reflect"
	"testing"

	"github.com/VladMinzatu/performance-handbook/doc-pipeline/internal/ingest"
)

func TestTokenize_EmptyText(t *testing.T) {
	doc := ingest.Document{
		ID:   "doc-1",
		Text: "",
	}

	result, err := Tokenize(doc)
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}

	assertTokenizedDoc(t, result, "doc-1", []Token{})
}

func TestTokenize_SimpleWords(t *testing.T) {
	doc := ingest.Document{
		ID:   "doc-2",
		Text: "hello world",
	}

	result, err := Tokenize(doc)
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}

	expected := []Token{
		{Term: "hello"},
		{Term: "world"},
	}
	assertTokenizedDoc(t, result, "doc-2", expected)
}

func TestTokenize_MixedCase(t *testing.T) {
	doc := ingest.Document{
		ID:   "doc-3",
		Text: "Hello WORLD Test",
	}

	result, err := Tokenize(doc)
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}

	expected := []Token{
		{Term: "hello"},
		{Term: "world"},
		{Term: "test"},
	}
	assertTokenizedDoc(t, result, "doc-3", expected)
}

func TestTokenize_WithPunctuation(t *testing.T) {
	doc := ingest.Document{
		ID:   "doc-4",
		Text: "Hello, world! How are you?",
	}

	result, err := Tokenize(doc)
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}

	expected := []Token{
		{Term: "hello"},
		{Term: "world"},
		{Term: "how"},
		{Term: "are"},
		{Term: "you"},
	}
	assertTokenizedDoc(t, result, "doc-4", expected)
}

func TestTokenize_WithNumbers(t *testing.T) {
	doc := ingest.Document{
		ID:   "doc-5",
		Text: "Version 2.0 is better than 1.0",
	}

	result, err := Tokenize(doc)
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}

	expected := []Token{
		{Term: "version"},
		{Term: "is"},
		{Term: "better"},
		{Term: "than"},
	}
	assertTokenizedDoc(t, result, "doc-5", expected)
}

func TestTokenize_MultipleSpaces(t *testing.T) {
	doc := ingest.Document{
		ID:   "doc-6",
		Text: "word    word\tword\nword",
	}

	result, err := Tokenize(doc)
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}

	expected := []Token{
		{Term: "word"},
		{Term: "word"},
		{Term: "word"},
		{Term: "word"},
	}
	assertTokenizedDoc(t, result, "doc-6", expected)
}

func TestTokenize_WithSpecialCharacters(t *testing.T) {
	doc := ingest.Document{
		ID:   "doc-7",
		Text: "test@example.com & more#text",
	}

	result, err := Tokenize(doc)
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}

	expected := []Token{
		{Term: "test"},
		{Term: "example"},
		{Term: "com"},
		{Term: "more"},
		{Term: "text"},
	}
	assertTokenizedDoc(t, result, "doc-7", expected)
}

func TestTokenize_OnlyPunctuation(t *testing.T) {
	doc := ingest.Document{
		ID:   "doc-8",
		Text: "!!! @@@ ### $$$",
	}

	result, err := Tokenize(doc)
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}

	assertTokenizedDoc(t, result, "doc-8", []Token{})
}

func TestTokenize_OnlyNumbers(t *testing.T) {
	doc := ingest.Document{
		ID:   "doc-9",
		Text: "123 456 789",
	}

	result, err := Tokenize(doc)
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}

	assertTokenizedDoc(t, result, "doc-9", []Token{})
}

func TestTokenize_SingleWord(t *testing.T) {
	doc := ingest.Document{
		ID:   "doc-10",
		Text: "Hello",
	}

	result, err := Tokenize(doc)
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}

	expected := []Token{
		{Term: "hello"},
	}
	assertTokenizedDoc(t, result, "doc-10", expected)
}

func TestTokenize_UnicodeLetters(t *testing.T) {
	doc := ingest.Document{
		ID:   "doc-11",
		Text: "Café résumé naïve",
	}

	result, err := Tokenize(doc)
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}

	expected := []Token{
		{Term: "café"},
		{Term: "résumé"},
		{Term: "naïve"},
	}
	assertTokenizedDoc(t, result, "doc-11", expected)
}

func TestTokenize_MixedUnicodeAndPunctuation(t *testing.T) {
	doc := ingest.Document{
		ID:   "doc-12",
		Text: "Hello, café! How's it going?",
	}

	result, err := Tokenize(doc)
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}

	expected := []Token{
		{Term: "hello"},
		{Term: "café"},
		{Term: "how"},
		{Term: "s"},
		{Term: "it"},
		{Term: "going"},
	}
	assertTokenizedDoc(t, result, "doc-12", expected)
}

func TestTokenize_LeadingTrailingWhitespace(t *testing.T) {
	doc := ingest.Document{
		ID:   "doc-13",
		Text: "   hello world   ",
	}

	result, err := Tokenize(doc)
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}

	expected := []Token{
		{Term: "hello"},
		{Term: "world"},
	}
	assertTokenizedDoc(t, result, "doc-13", expected)
}

func TestTokenize_HyphenatedWords(t *testing.T) {
	doc := ingest.Document{
		ID:   "doc-14",
		Text: "well-known state-of-the-art",
	}

	result, err := Tokenize(doc)
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}

	expected := []Token{
		{Term: "well"},
		{Term: "known"},
		{Term: "state"},
		{Term: "of"},
		{Term: "the"},
		{Term: "art"},
	}
	assertTokenizedDoc(t, result, "doc-14", expected)
}

func assertTokenizedDoc(t *testing.T, result TokenizedDoc, expectedID string, expectedTokens []Token) {
	t.Helper()

	if result.ID != expectedID {
		t.Errorf("expected ID '%s', got '%s'", expectedID, result.ID)
	}

	if !reflect.DeepEqual(result.Tokens, expectedTokens) {
		t.Errorf("expected tokens %v, got %v", expectedTokens, result.Tokens)
	}
}
