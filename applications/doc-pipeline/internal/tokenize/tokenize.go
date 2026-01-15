package tokenize

import (
	"strings"
	"unicode"

	"github.com/VladMinzatu/performance-handbook/doc-pipeline/internal/ingest"
)

type Token struct {
	Term string
}

type TokenizedDoc struct {
	ID     string
	Tokens []Token
}

func Tokenize(doc ingest.Document) (TokenizedDoc, error) {
	if doc.Text == "" {
		return TokenizedDoc{
			ID:     doc.ID,
			Tokens: []Token{},
		}, nil
	}

	text := strings.ToLower(doc.Text)

	// split on non-letters
	fields := strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r)
	})

	tokens := make([]Token, 0, len(fields))

	for _, f := range fields {
		if len(f) == 0 {
			continue
		}

		tokens = append(tokens, Token{
			Term: f,
		})
	}

	return TokenizedDoc{
		ID:     doc.ID,
		Tokens: tokens,
	}, nil
}
