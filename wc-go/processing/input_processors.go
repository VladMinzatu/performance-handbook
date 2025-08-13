package processing

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"
)

type InputProcessor interface {
	RunThrough(lineProcessors []LineProcessor) error
}

type LineScannerInputProcessor struct {
	reader io.Reader
}

func NewLineScannerInputProcessor(reader io.Reader) (*LineScannerInputProcessor, error) {
	if reader == nil {
		return nil, fmt.Errorf("reader cannot be nil")
	}
	return &LineScannerInputProcessor{reader: reader}, nil
}

func (p *LineScannerInputProcessor) RunThrough(lineProcessors []LineProcessor) error {
	return process(p.reader, lineProcessors)
}

type UpFrontLoadingInputProcessor struct {
	filePath string
	reader   io.Reader
}

func NewUpFrontLoadingInputProcessorFromFile(filePath string) (*UpFrontLoadingInputProcessor, error) {
	if filePath == "" {
		return nil, fmt.Errorf("filePath cannot be empty")
	}
	return &UpFrontLoadingInputProcessor{
		filePath: filePath,
		reader:   nil,
	}, nil
}

func NewUpFrontLoadingInputProcessorFromReader(reader io.Reader) (*UpFrontLoadingInputProcessor, error) {
	if reader == nil {
		return nil, fmt.Errorf("reader cannot be nil")
	}
	return &UpFrontLoadingInputProcessor{
		filePath: "",
		reader:   reader,
	}, nil
}

func (p *UpFrontLoadingInputProcessor) RunThrough(lineProcessors []LineProcessor) error {
	var data []byte
	var err error

	if p.reader != nil {
		data, err = io.ReadAll(p.reader)
		if err != nil {
			return err
		}
	} else {
		data, err = os.ReadFile(p.filePath)
		if err != nil {
			return err
		}
	}

	reader := strings.NewReader(string(data))
	return process(reader, lineProcessors)
}

type BufferedInputProcessor struct {
	reader io.Reader
}

func NewBufferedInputProcessor(reader io.Reader) (*BufferedInputProcessor, error) {
	if reader == nil {
		return nil, fmt.Errorf("reader cannot be nil")
	}
	return &BufferedInputProcessor{reader: reader}, nil
}

func (p *BufferedInputProcessor) RunThrough(lineProcessors []LineProcessor) error {
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
			return err
		}
	}

	text := buffer.String()
	reader := strings.NewReader(text)
	return process(reader, lineProcessors)
}

type MmapInputProcessor struct {
	filePath string
}

func NewMmapInputProcessor(filePath string) (*MmapInputProcessor, error) {
	if filePath == "" {
		return nil, fmt.Errorf("filePath cannot be empty")
	}
	return &MmapInputProcessor{filePath: filePath}, nil
}

func (p *MmapInputProcessor) RunThrough(lineProcessors []LineProcessor) error {
	f, err := os.Open(p.filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return err
	}
	size := fi.Size()
	if size == 0 {
		// file is empty, nothing to do
		return nil
	}

	// Memory map the file (read-only)
	data, err := syscall.Mmap(int(f.Fd()), 0, int(size), syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil {
		return err
	}
	defer syscall.Munmap(data)

	text := string(data)
	reader := strings.NewReader(text)
	return process(reader, lineProcessors)
}

func process(reader io.Reader, lineProcessors []LineProcessor) error {
	scanner := bufio.NewScanner(reader)

	for scanner.Scan() {
		line := scanner.Text()
		for _, processor := range lineProcessors {
			processor.Process(line)
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}
