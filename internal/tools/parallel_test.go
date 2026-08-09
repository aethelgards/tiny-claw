package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// TestParallelEditsNoLostUpdate runs many edit_file calls concurrently against
// the same file, each replacing a distinct unique line. Without the exclusive
// lock around edit_file's read-modify-write cycle, two edits would both read
// the original content and the later write would silently drop the earlier
// edit; with the lock every edit must land.
func TestParallelEditsNoLostUpdate(t *testing.T) {
	workDir := t.TempDir()

	const n = 8
	var lines []string
	for i := 0; i < n; i++ {
		lines = append(lines, fmt.Sprintf("line-%d", i))
	}
	writeFile(t, filepath.Join(workDir, "f.txt"), strings.Join(lines, "\n")+"\n")

	var wg sync.WaitGroup
	errCh := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if _, err := executeEditFile(t, workDir, "f.txt",
				fmt.Sprintf("line-%d", i), fmt.Sprintf("edited-%d", i)); err != nil {
				errCh <- fmt.Errorf("edit %d: %w", i, err)
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Error(err)
	}

	data, err := os.ReadFile(filepath.Join(workDir, "f.txt"))
	if err != nil {
		t.Fatalf("read back file: %v", err)
	}
	for i := 0; i < n; i++ {
		if !strings.Contains(string(data), fmt.Sprintf("edited-%d", i)) {
			t.Errorf("edit %d lost: file missing %q", i, fmt.Sprintf("edited-%d", i))
		}
	}
}

// TestParallelReadWriteMix runs readers, a writer and editors concurrently on
// the same workspace to exercise the shared read/write lock under -race: reads
// may overlap, but must never observe a torn write.
func TestParallelReadWriteMix(t *testing.T) {
	workDir := t.TempDir()
	writeFile(t, filepath.Join(workDir, "a.txt"), "original\n")

	var wg sync.WaitGroup
	errCh := make(chan error, 24)

	// Concurrent readers of the same file.
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := executeReadFile(t, workDir, "a.txt")
			if err != nil {
				errCh <- fmt.Errorf("read: %w", err)
				return
			}
			if !strings.Contains(got, "original") && !strings.Contains(got, "replaced") {
				errCh <- fmt.Errorf("read returned unexpected content %q", got)
			}
		}()
	}

	// A writer racing the readers.
	wg.Add(1)
	go func() {
		defer wg.Done()
		if _, err := executeWriteFile(t, workDir, "b.txt", "fresh\n"); err != nil {
			errCh <- fmt.Errorf("write: %w", err)
		}
	}()

	// Editors mutating the read file while it is being read.
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := executeEditFile(t, workDir, "a.txt", "original", "replaced"); err != nil &&
				!strings.Contains(err.Error(), "未找到") {
				errCh <- fmt.Errorf("edit: %w", err)
			}
		}()
	}

	// A bash command touching the workspace concurrently.
	wg.Add(1)
	go func() {
		defer wg.Done()
		if _, err := executeBash(t, workDir, "printf 'shell\\n' > c.txt"); err != nil {
			errCh <- fmt.Errorf("bash: %w", err)
		}
	}()

	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Error(err)
	}
}

// TestParallelWritesDisjointPaths runs write_file calls concurrently against
// different files. The per-path lock must not serialize them, and every
// write must land with its own content (no cross-file interference).
func TestParallelWritesDisjointPaths(t *testing.T) {
	workDir := t.TempDir()

	const n = 8
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := fmt.Sprintf("f%d.txt", i)
			if _, err := executeWriteFile(t, workDir, name, fmt.Sprintf("content-%d\n", i)); err != nil {
				t.Errorf("write %s failed: %v", name, err)
			}
		}(i)
	}
	wg.Wait()

	for i := 0; i < n; i++ {
		name := fmt.Sprintf("f%d.txt", i)
		want := fmt.Sprintf("content-%d\n", i)
		got, err := executeReadFile(t, workDir, name)
		if err != nil {
			t.Fatalf("read %s failed: %v", name, err)
		}
		if got != want {
			t.Errorf("file %s = %q, want %q", name, got, want)
		}
	}
}
