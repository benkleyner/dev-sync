package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatcherSeesFilesCreatedInsideNewDirectory(t *testing.T) {
	root := t.TempDir()
	watcher, err := NewWatcher(root)
	if err != nil {
		t.Fatal(err)
	}
	defer watcher.Close()

	newDir := filepath.Join(root, "new-dir")
	if err := os.Mkdir(newDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := watcher.WatchCreatedDir(newDir); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(newDir, "file.txt")
	if err := os.WriteFile(target, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	deadline := time.After(2 * time.Second)
	for {
		select {
		case ev := <-watcher.Events():
			if ev.Name == target {
				return
			}
		case err := <-watcher.Errors():
			t.Fatal(err)
		case <-deadline:
			t.Fatalf("timed out waiting for event for %s", target)
		}
	}
}
