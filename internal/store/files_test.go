package store

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestWriteFileAndReadFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "file.txt")

	if err := WriteFile(path, []byte("hello")); err != nil {
		t.Fatalf("write file: %v", err)
	}

	got, err := ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if got != "hello" {
		t.Fatalf("expected hello, got %q", got)
	}
}

func TestWriteFileReplacesExistingContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	if err := WriteFile(path, []byte("new")); err != nil {
		t.Fatalf("write file: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(got) != "new" {
		t.Fatalf("expected new, got %q", string(got))
	}
}

func TestAppendFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", "events.log")

	if err := AppendFile(path, []byte("one\n")); err != nil {
		t.Fatalf("append first: %v", err)
	}
	if err := AppendFile(path, []byte("two\n")); err != nil {
		t.Fatalf("append second: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(got) != "one\ntwo\n" {
		t.Fatalf("unexpected contents: %q", string(got))
	}
}

func TestAppendFileConcurrent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "concurrent.log")
	const writes = 100

	var wg sync.WaitGroup
	for i := 0; i < writes; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := AppendFile(path, []byte("x\n")); err != nil {
				t.Errorf("append file: %v", err)
			}
		}()
	}
	wg.Wait()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != writes {
		t.Fatalf("expected %d lines, got %d", writes, len(lines))
	}
}
