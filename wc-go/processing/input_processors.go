package processing

import (
	"bufio"
	"bytes"
	"io"
	"log"
	"os"
	"strings"
	"syscall"
)

type InputProcessor interface {
	RunThrough(lineProcessors []LineProcessor)
}

type LineScannerInputProcessor struct {
	reader io.Reader
}

func NewLineScannerInputProcessor(reader io.Reader) *LineScannerInputProcessor {
	return &LineScannerInputProcessor{reader: reader}
}

func (p *LineScannerInputProcessor) RunThrough(lineProcessors []LineProcessor) {
	scanner := bufio.NewScanner(p.reader)
	for scanner.Scan() {
		line := scanner.Text()
		for _, processor := range lineProcessors {
			processor.Process(line)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("Error reading input: %v", err)
		os.Exit(1)
	}
}

type UpFrontLoadingInputProcessor struct {
	filePath string
	reader   io.Reader
}

func NewUpFrontLoadingInputProcessorFromFile(filePath string) *UpFrontLoadingInputProcessor {
	return &UpFrontLoadingInputProcessor{
		filePath: filePath,
		reader:   nil,
	}
}

func NewUpFrontLoadingInputProcessorFromReader(reader io.Reader) *UpFrontLoadingInputProcessor {
	return &UpFrontLoadingInputProcessor{
		filePath: "",
		reader:   reader,
	}
}

func (p *UpFrontLoadingInputProcessor) RunThrough(lineProcessors []LineProcessor) {
	var data []byte
	var err error

	if p.reader != nil {
		data, err = io.ReadAll(p.reader)
		if err != nil {
			log.Fatalf("Error reading input: %v", err)
			os.Exit(1)
		}
	} else {
		data, err = os.ReadFile(p.filePath)
		if err != nil {
			log.Fatalf("Error reading input: %v", err)
		}
	}
	text := string(data)
	splitAndProcess(text, lineProcessors)
}

type BufferedInputProcessor struct {
	reader io.Reader
}

func NewBufferedInputProcessor(reader io.Reader) *BufferedInputProcessor {
	return &BufferedInputProcessor{reader: reader}
}

func (p *BufferedInputProcessor) RunThrough(lineProcessors []LineProcessor) {
	bufReader := bufio.NewReader(p.reader)
	var buffer bytes.Buffer

	chunk := make([]byte, 4096) // 4KB chunks
	for {
		n, err := bufReader.Read(chunk)
		if n > 0 {
			buffer.Write(chunk[:n])
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("Error reading input: %v", err)
			os.Exit(1)
		}
	}

	text := buffer.String()
	splitAndProcess(text, lineProcessors)
}

type MmapInputProcessor struct {
	filePath string
}

func NewMmapInputProcessor(filePath string) *MmapInputProcessor {
	return &MmapInputProcessor{filePath: filePath}
}

func (p *MmapInputProcessor) RunThrough(lineProcessors []LineProcessor) {
	f, err := os.Open(p.filePath)
	if err != nil {
		log.Fatalf("Error opening file: %v", err)
		os.Exit(1)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		log.Fatalf("Error statting file: %v", err)
		os.Exit(1)
	}
	size := fi.Size()
	if size == 0 {
		// file is empty, nothing to do
		return
	}

	// Memory map the file (read-only)
	data, err := syscall.Mmap(int(f.Fd()), 0, int(size), syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil {
		log.Fatalf("Error mapping file: %v", err)
		os.Exit(1)
	}
	defer syscall.Munmap(data)

	text := string(data)
	splitAndProcess(text, lineProcessors)
}

func splitAndProcess(text string, lineProcessors []LineProcessor) {
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		for _, processor := range lineProcessors {
			processor.Process(line)
		}
	}
}
