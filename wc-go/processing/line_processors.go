package processing

import (
	"strings"
	"unicode/utf8"
)

type LineProcessor interface {
	Process(line string)
	Count() int
}

type WordCountProcessor struct {
	counter int
}

func (p *WordCountProcessor) Process(line string) {
	words := strings.Fields(line)
	p.counter += len(words)
}

func (p *WordCountProcessor) Count() int {
	return p.counter
}

type LineCountProcessor struct {
	counter int
}

func (p *LineCountProcessor) Process(line string) {
	p.counter++
}

func (p *LineCountProcessor) Count() int {
	return p.counter
}

type CharacterCountProcessor struct {
	counter int
}

func (p *CharacterCountProcessor) Process(line string) {
	p.counter += utf8.RuneCountInString(line)
}

func (p *CharacterCountProcessor) Count() int {
	return p.counter
}
