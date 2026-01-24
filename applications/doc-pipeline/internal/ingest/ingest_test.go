package ingest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadData_Success(t *testing.T) {
	content := "This is test content for the document pipeline"
	filePath := createTestFile(t, "test.txt", content)

	doc, err := LoadData(filePath, 0, len(content), "test-id-1")
	if err != nil {
		t.Fatalf("LoadData failed: %v", err)
	}

	assertDocument(t, doc, "test-id-1", content)
}

func TestLoadData_PartialRead(t *testing.T) {
	fullContent := "This is a very long test content that exceeds the text size limit we want to read"
	filePath := createTestFile(t, "test.txt", fullContent)

	textSize := 20
	doc, err := LoadData(filePath, 0, textSize, "test-id-2")
	if err != nil {
		t.Fatalf("LoadData failed: %v", err)
	}

	expectedText := fullContent[:textSize]
	assertDocument(t, doc, "test-id-2", expectedText)

	if len(doc.Text) != textSize {
		t.Errorf("expected text length %d, got %d", textSize, len(doc.Text))
	}
}

func TestLoadData_EmptyFile(t *testing.T) {
	filePath := createEmptyFile(t, "empty.txt")

	doc, err := LoadData(filePath, 0, 100, "test-id-3")
	if err != nil {
		t.Fatalf("LoadData failed: %v", err)
	}

	assertDocument(t, doc, "test-id-3", "")
}

func TestLoadData_TextSizeLargerThanFile(t *testing.T) {
	content := "small content"
	filePath := createTestFile(t, "small.txt", content)

	doc, err := LoadData(filePath, 0, 1000, "test-id-4")
	if err != nil {
		t.Fatalf("LoadData failed: %v", err)
	}

	assertDocument(t, doc, "test-id-4", content)

	if len(doc.Text) != len(content) {
		t.Errorf("expected text length %d, got %d", len(content), len(doc.Text))
	}
}

func TestLoadData_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	nonExistentPath := filepath.Join(tmpDir, "nonexistent.txt")

	_, err := LoadData(nonExistentPath, 0, 100, "test-id-5")
	if err == nil {
		t.Fatal("expected error for non-existent file, got nil")
	}
}

func TestLoadData_ZeroTextSize(t *testing.T) {
	content := "some content"
	filePath := createTestFile(t, "test.txt", content)

	doc, err := LoadData(filePath, 0, 0, "test-id-6")
	if err != nil {
		t.Fatalf("LoadData failed: %v", err)
	}

	assertDocument(t, doc, "test-id-6", "")
}

func TestLoadData_WithOffset(t *testing.T) {
	fullContent := "This is a very long test content that exceeds the text size limit we want to read"
	filePath := createTestFile(t, "test.txt", fullContent)

	offset := 20
	textSize := 15
	doc, err := LoadData(filePath, offset, textSize, "test-id-7")
	if err != nil {
		t.Fatalf("LoadData failed: %v", err)
	}

	expectedText := fullContent[offset : offset+textSize]
	assertDocument(t, doc, "test-id-7", expectedText)

	if len(doc.Text) != textSize {
		t.Errorf("expected text length %d, got %d", textSize, len(doc.Text))
	}
}

func TestLoadData_OffsetBeyondFileSize(t *testing.T) {
	content := "small content"
	filePath := createTestFile(t, "small.txt", content)

	offset := len(content) + 100
	doc, err := LoadData(filePath, offset, 100, "test-id-8")
	if err != nil {
		t.Fatalf("LoadData failed: %v", err)
	}

	assertDocument(t, doc, "test-id-8", "")
}

// Helpers
func createTestFile(t *testing.T, filename string, content string) string {
	t.Helper()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, filename)
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	return filePath
}

func createEmptyFile(t *testing.T, filename string) string {
	t.Helper()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, filename)
	file, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("failed to create empty file: %v", err)
	}
	file.Close()
	return filePath
}

func assertDocument(t *testing.T, doc Document, expectedID, expectedText string) {
	t.Helper()
	if doc.ID != expectedID {
		t.Errorf("expected ID '%s', got '%s'", expectedID, doc.ID)
	}
	if doc.Text != expectedText {
		t.Errorf("expected text '%s', got '%s'", expectedText, doc.Text)
	}
}
