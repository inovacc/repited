package scanner

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestWatchScriptFileTriggersCallback(t *testing.T) {
	dir := t.TempDir()

	var called atomic.Int32

	w, err := NewWatcher([]string{dir}, func() {
		called.Add(1)
	})
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}

	defer func() { _ = w.Close() }()

	ctx := t.Context()

	go func() {
		_ = w.Start(ctx)
	}()

	// Give the watcher time to start
	time.Sleep(100 * time.Millisecond)

	// Create a .sh file — should trigger callback
	if err := os.WriteFile(filepath.Join(dir, "test.sh"), []byte("#!/bin/bash\necho hello"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Wait for debounce (500ms) + margin
	time.Sleep(800 * time.Millisecond)

	if got := called.Load(); got < 1 {
		t.Errorf("expected callback to be called at least once, got %d", got)
	}
}

func TestWatchNonScriptFileIgnored(t *testing.T) {
	dir := t.TempDir()

	var called atomic.Int32

	w, err := NewWatcher([]string{dir}, func() {
		called.Add(1)
	})
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}

	defer func() { _ = w.Close() }()

	ctx := t.Context()

	go func() {
		_ = w.Start(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	// Create a .txt file — should NOT trigger callback
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("just notes"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Wait long enough that a debounced callback would have fired
	time.Sleep(800 * time.Millisecond)

	if got := called.Load(); got != 0 {
		t.Errorf("expected callback not to be called for .txt file, got %d calls", got)
	}
}

func TestWatchDebounce(t *testing.T) {
	dir := t.TempDir()

	var called atomic.Int32

	w, err := NewWatcher([]string{dir}, func() {
		called.Add(1)
	})
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}

	defer func() { _ = w.Close() }()

	ctx := t.Context()

	go func() {
		_ = w.Start(ctx)
	}()

	time.Sleep(100 * time.Millisecond)

	// Rapid-fire multiple file writes within the debounce window
	for i := range 5 {
		name := filepath.Join(dir, "rapid"+string(rune('0'+i))+".sh")

		if err := os.WriteFile(name, []byte("#!/bin/bash\necho "+string(rune('0'+i))), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		time.Sleep(50 * time.Millisecond)
	}

	// Wait for debounce to settle
	time.Sleep(800 * time.Millisecond)

	got := called.Load()
	if got != 1 {
		t.Errorf("expected exactly 1 debounced callback, got %d", got)
	}
}
