package ingest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadData_Success(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	content := "This is test content for the document pipeline"
	
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	doc, err := LoadData(filePath, len(content), "test-id-1")
	if err != nil {
		t.Fatalf("LoadData failed: %v", err)
	}

	if doc.ID != "test-id-1" {
		t.Errorf("expected ID 'test-id-1', got '%s'", doc.ID)
	}

	if doc.Text != content {
		t.Errorf("expected text '%s', got '%s'", content, doc.Text)
	}
}

func TestLoadData_PartialRead(t *testing.T) {
	// Create a temporary file with more content than we'll read
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	fullContent := "This is a very long test content that exceeds the text size limit we want to read"
	
	err := os.WriteFile(filePath, []byte(fullContent), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Read only first 20 bytes
	textSize := 20
	doc, err := LoadData(filePath, textSize, "test-id-2")
	if err != nil {
		t.Fatalf("LoadData failed: %v", err)
	}

	if doc.ID != "test-id-2" {
		t.Errorf("expected ID 'test-id-2', got '%s'", doc.ID)
	}

	expectedText := fullContent[:textSize]
	if doc.Text != expectedText {
		t.Errorf("expected text '%s', got '%s'", expectedText, doc.Text)
	}

	if len(doc.Text) != textSize {
		t.Errorf("expected text length %d, got %d", textSize, len(doc.Text))
	}
}

func TestLoadData_EmptyFile(t *testing.T) {
	// Create an empty temporary file
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "empty.txt")
	
	file, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("failed to create empty file: %v", err)
	}
	file.Close()

	doc, err := LoadData(filePath, 100, "test-id-3")
	if err != nil {
		t.Fatalf("LoadData failed: %v", err)
	}

	if doc.ID != "test-id-3" {
		t.Errorf("expected ID 'test-id-3', got '%s'", doc.ID)
	}

	if doc.Text != "" {
		t.Errorf("expected empty text, got '%s'", doc.Text)
	}
}

func TestLoadData_TextSizeLargerThanFile(t *testing.T) {
	// Create a file with less content than textSize
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "small.txt")
	content := "small content"
	
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Request more bytes than file contains
	textSize := 1000
	doc, err := LoadData(filePath, textSize, "test-id-4")
	if err != nil {
		t.Fatalf("LoadData failed: %v", err)
	}

	if doc.ID != "test-id-4" {
		t.Errorf("expected ID 'test-id-4', got '%s'", doc.ID)
	}

	// Should only read what's available
	if doc.Text != content {
		t.Errorf("expected text '%s', got '%s'", content, doc.Text)
	}

	if len(doc.Text) != len(content) {
		t.Errorf("expected text length %d, got %d", len(content), len(doc.Text))
	}
}

func TestLoadData_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	nonExistentPath := filepath.Join(tmpDir, "nonexistent.txt")

	_, err := LoadData(nonExistentPath, 100, "test-id-5")
	if err == nil {
		t.Fatal("expected error for non-existent file, got nil")
	}
}

func TestLoadData_ZeroTextSize(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	content := "some content"
	
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	doc, err := LoadData(filePath, 0, "test-id-6")
	if err != nil {
		t.Fatalf("LoadData failed: %v", err)
	}

	if doc.ID != "test-id-6" {
		t.Errorf("expected ID 'test-id-6', got '%s'", doc.ID)
	}

	if doc.Text != "" {
		t.Errorf("expected empty text for zero textSize, got '%s'", doc.Text)
	}
}
