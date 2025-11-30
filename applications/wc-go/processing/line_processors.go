package processing

import (
	"unicode/utf8"
)

/*
All implementations of Process work on byte slices representing lines of text in the input.
But they also all avoid allocations - this will come in handy when we check the processing of large files with mmap.
*/
type LineProcessor interface {
	Process(line []byte)
	Count() int
}

type WordCountProcessor struct {
	counter int
}

func (p *WordCountProcessor) Process(line []byte) {
	inWord := false
	words := 0
	for len(line) > 0 {
		r, size := utf8.DecodeRune(line)
		if isSpace(r) {
			inWord = false
		} else if !inWord {
			inWord = true
			words++
		}
		line = line[size:]
	}
	p.counter += words
}

func (p *WordCountProcessor) Count() int {
	return p.counter
}

type LineCountProcessor struct {
	counter int
}

func (p *LineCountProcessor) Process(line []byte) {
	p.counter++
}

func (p *LineCountProcessor) Count() int {
	return p.counter
}

type CharacterCountProcessor struct {
	counter int
}

func (p *CharacterCountProcessor) Process(line []byte) {
	count := 0
	for len(line) > 0 {
		_, size := utf8.DecodeRune(line)
		line = line[size:]
		count++
	}
	p.counter += count
}

func isSpace(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\v', '\f', '\r':
		return true
	default:
		return false
	}
}

func (p *CharacterCountProcessor) Count() int {
	return p.counter
}
