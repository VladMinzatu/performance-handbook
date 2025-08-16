package processing

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"syscall"
)

type InputProcessor struct {
	ProcessorType string
	FilePath      string
}

const (
	ProcessorTypeScanner   = "scanner"
	ProcessorTypeUpFront   = "upfront"
	ProcessorTypeBuffering = "buffering"
	ProcessorTypeMmap      = "mmap"
	ProcessorTypeMmap2     = "mmap2"
)

func (p *InputProcessor) Run(lineProcessors []LineProcessor) error {
	switch p.ProcessorType {
	case ProcessorTypeScanner:
		if p.FilePath == "" {
			return runWithScannerOnReader(os.Stdin, lineProcessors)
		}
		return runWithScannerOnFile(p.FilePath, lineProcessors)
	case ProcessorTypeUpFront:
		if p.FilePath == "" {
			return runWithUpFrontLoadingOnReader(os.Stdin, lineProcessors)
		}
		return runWithUpFrontLoadingOnFile(p.FilePath, lineProcessors)
	case ProcessorTypeBuffering:
		if p.FilePath == "" {
			return runWithBufferringOnReader(os.Stdin, lineProcessors)
		}
		return runWithBufferringOnFile(p.FilePath, lineProcessors)
	case ProcessorTypeMmap:
		if p.FilePath == "" {
			return fmt.Errorf("file path is required for mmap processor")
		}
		return runWithMmapOnFile(p.FilePath, lineProcessors, true)
	case ProcessorTypeMmap2:
		if p.FilePath == "" {
			return fmt.Errorf("file path is required for mmap2 processor")
		}
		return runWithMmapOnFile(p.FilePath, lineProcessors, false)
	default:
		return fmt.Errorf("unknown processor type: %s", p.ProcessorType)
	}
}

func runWithScannerOnReader(reader io.Reader, lineProcessors []LineProcessor) error {
	if reader == nil {
		return fmt.Errorf("reader cannot be nil")
	}
	return process(reader, lineProcessors)
}

func runWithScannerOnFile(filePath string, lineProcessors []LineProcessor) error {
	err := checkFilePath(filePath)
	if err != nil {
		return fmt.Errorf("invalid file path: %w", err)
	}

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()
	return process(file, lineProcessors)
}

func runWithUpFrontLoadingOnReader(reader io.Reader, lineProcessors []LineProcessor) error {
	if reader == nil {
		return fmt.Errorf("reader cannot be nil")
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	return processBytes(data, lineProcessors)
}

func runWithUpFrontLoadingOnFile(filePath string, lineProcessors []LineProcessor) error {
	err := checkFilePath(filePath)
	if err != nil {
		return fmt.Errorf("invalid file path: %w", err)
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	return processBytes(data, lineProcessors)
}

func runWithBufferringOnReader(reader io.Reader, lineProcessors []LineProcessor) error {
	if reader == nil {
		return fmt.Errorf("reader cannot be nil")
	}
	bufReader := bufio.NewReader(reader)
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

	return processBytes(buffer.Bytes(), lineProcessors)
}

func runWithBufferringOnFile(filePath string, lineProcessors []LineProcessor) error {
	err := checkFilePath(filePath)
	if err != nil {
		return fmt.Errorf("invalid file path: %w", err)
	}
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()
	return runWithBufferringOnReader(file, lineProcessors)
}

func runWithMmapOnFile(filePath string, lineProcessors []LineProcessor, avoidAllocations bool) error {
	err := checkFilePath(filePath)
	if err != nil {
		return fmt.Errorf("invalid file path: %w", err)
	}

	f, err := os.Open(filePath)
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

	if avoidAllocations {
		return processBytes(data, lineProcessors)
	}
	return process(bytes.NewReader(data), lineProcessors)
}

func processBytes(data []byte, lineProcessors []LineProcessor) error {
	if len(data) == 0 {
		return nil
	}

	start := 0
	for i, b := range data {
		if b == '\n' {
			end := i
			if end > start && data[end-1] == '\r' {
				end--
			}
			line := data[start:end]
			for _, processor := range lineProcessors {
				processor.Process(line)
			}
			start = i + 1
		}
	}
	// Handle last line if data doesn't end with newline
	if start < len(data) {
		line := data[start:]
		for _, processor := range lineProcessors {
			processor.Process(line)
		}
	}
	return nil
}

func process(reader io.Reader, lineProcessors []LineProcessor) error {
	scanner := bufio.NewScanner(reader)

	for scanner.Scan() {
		line := scanner.Text()
		for _, processor := range lineProcessors {
			processor.Process([]byte(line))
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}

func checkFilePath(filePath string) error {
	if filePath == "" {
		return fmt.Errorf("filePath cannot be empty")
	}
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("file does not exist: %s", filePath)
	}
	return nil
}
