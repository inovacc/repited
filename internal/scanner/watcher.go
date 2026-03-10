package scanner

import (
	"context"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// scriptExtensions are file extensions that trigger a re-scan.
var scriptExtensions = map[string]struct{}{
	".sh":   {},
	".bash": {},
	".ps1":  {},
	".py":   {},
}

// Watcher monitors .scripts/ directories for file changes and triggers a callback.
type Watcher struct {
	watcher  *fsnotify.Watcher
	onChange func()
	debounce time.Duration
	mu       sync.Mutex
	dirs     map[string]struct{}
	logger   *slog.Logger
}

// NewWatcher creates a Watcher that monitors the given directories and calls
// onChange when script files are created, modified, or removed.
func NewWatcher(dirs []string, onChange func()) (*Watcher, error) {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	w := &Watcher{
		watcher:  fw,
		onChange: onChange,
		debounce: 500 * time.Millisecond,
		dirs:     make(map[string]struct{}),
		logger:   slog.Default(),
	}

	for _, dir := range dirs {
		if err := w.AddDir(dir); err != nil {
			_ = fw.Close()

			return nil, err
		}
	}

	return w, nil
}

// SetOnChange replaces the callback function invoked when script files change.
func (w *Watcher) SetOnChange(fn func()) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.onChange = fn
}

// AddDir adds a directory to the watch list if not already watched.
func (w *Watcher) AddDir(dir string) error {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if _, ok := w.dirs[abs]; ok {
		return nil
	}

	if err := w.watcher.Add(abs); err != nil {
		return err
	}

	w.dirs[abs] = struct{}{}
	w.logger.Info("watching directory", "dir", abs)

	return nil
}

// Start runs the event loop, debouncing filesystem events and calling onChange.
// It blocks until the context is cancelled.
func (w *Watcher) Start(ctx context.Context) error {
	var timer *time.Timer

	var timerMu sync.Mutex

	for {
		select {
		case <-ctx.Done():
			timerMu.Lock()
			if timer != nil {
				timer.Stop()
			}
			timerMu.Unlock()

			return ctx.Err()

		case event, ok := <-w.watcher.Events:
			if !ok {
				return nil
			}

			if !isScriptFile(event.Name) {
				continue
			}

			// Only react to create, write, remove, and rename events.
			if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove|fsnotify.Rename) == 0 {
				continue
			}

			w.logger.Info("file changed", "file", event.Name, "op", event.Op.String())

			timerMu.Lock()
			if timer != nil {
				timer.Stop()
			}

			timer = time.AfterFunc(w.debounce, func() {
				w.onChange()
			})
			timerMu.Unlock()

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return nil
			}

			w.logger.Error("watcher error", "err", err)
		}
	}
}

// Close shuts down the underlying fsnotify watcher.
func (w *Watcher) Close() error {
	return w.watcher.Close()
}

// isScriptFile checks whether the file has a script extension we care about.
func isScriptFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	_, ok := scriptExtensions[ext]

	return ok
}
