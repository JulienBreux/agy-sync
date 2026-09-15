package watcher

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/julienbreux/agy-sync/internal/logger"
)

// WatchEvent represents a detected change in an Antigravity conversation directory.
type WatchEvent struct {
	ConversationID string
	FilePath       string
	IsTranscript   bool
}

// Watcher monitors the Antigravity brain directory for file modifications and new sessions.
type Watcher struct {
	brainDir    string
	debounce    time.Duration
	fsw         *fsnotify.Watcher
	watchedDirs map[string]bool
	mu          sync.RWMutex
}

// NewWatcher initializes a filesystem watcher for the brain directory.
func NewWatcher(brainDir string, debounce time.Duration) (*Watcher, error) {
	if debounce <= 0 {
		debounce = 200 * time.Millisecond
	}

	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create fsnotify watcher: %w", err)
	}

	return &Watcher{
		brainDir:    filepath.Clean(brainDir),
		debounce:    debounce,
		fsw:         fsw,
		watchedDirs: make(map[string]bool),
	}, nil
}

// WatchDirectory registers an additional directory to monitor.
func (w *Watcher) WatchDirectory(dir string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	cleaned := filepath.Clean(dir)
	if w.watchedDirs[cleaned] {
		return nil
	}

	if err := w.fsw.Add(cleaned); err != nil {
		return fmt.Errorf("failed adding directory %s to watcher: %w", cleaned, err)
	}
	w.watchedDirs[cleaned] = true
	return nil
}

// InspectPath parses an absolute file path and determines the conversation ID and whether it's a transcript.
func InspectPath(brainDir, path string) (conversationID string, isTranscript bool) {
	rel, err := filepath.Rel(filepath.Clean(brainDir), filepath.Clean(path))
	if err != nil || strings.HasPrefix(rel, "..") || rel == "." {
		return "", false
	}

	parts := strings.Split(rel, string(filepath.Separator))
	if len(parts) == 0 || parts[0] == "" {
		return "", false
	}

	convID := parts[0]
	isTr := filepath.Base(path) == "transcript.jsonl"
	return convID, isTr
}

// Start initiates the watcher loop in the background and returns channels for events and errors.
func (w *Watcher) Start(ctx context.Context) (<-chan WatchEvent, <-chan error, error) {
	// Recursively register all existing subdirectories
	if err := w.registerExistingDirs(); err != nil {
		return nil, nil, err
	}

	outEvents := make(chan WatchEvent, 100)
	outErrors := make(chan error, 50)

	go w.eventLoop(ctx, outEvents, outErrors)

	return outEvents, outErrors, nil
}

func (w *Watcher) registerExistingDirs() error {
	if _, err := os.Stat(w.brainDir); os.IsNotExist(err) {
		if err := os.MkdirAll(w.brainDir, 0o755); err != nil {
			return fmt.Errorf("failed creating brain directory: %w", err)
		}
	}

	return filepath.WalkDir(w.brainDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			_ = w.WatchDirectory(path)
		}
		return nil
	})
}

func (w *Watcher) eventLoop(ctx context.Context, outEvents chan<- WatchEvent, outErrors chan<- error) {
	log := logger.FromContext(ctx)
	defer close(outEvents)
	defer close(outErrors)

	timers := make(map[string]*time.Timer)
	var timerMu sync.Mutex

	for {
		select {
		case <-ctx.Done():
			log.DebugContext(ctx, "Filesystem watcher stopped via context cancellation")
			return

		case err, ok := <-w.fsw.Errors:
			if !ok {
				return
			}
			log.ErrorContext(ctx, "Filesystem watcher error received", "error", err)
			outErrors <- err

		case event, ok := <-w.fsw.Events:
			if !ok {
				return
			}

			// If new directory created, register it
			if event.Has(fsnotify.Create) {
				fi, err := os.Stat(event.Name)
				if err == nil && fi.IsDir() {
					log.DebugContext(ctx, "Registering newly created directory to watcher", "dir", event.Name)
					_ = w.WatchDirectory(event.Name)
					continue
				}
			}

			if !event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) {
				continue
			}

			convID, isTranscript := InspectPath(w.brainDir, event.Name)
			if convID == "" {
				continue
			}

			// Debounce notifications for the same file
			timerMu.Lock()
			if t, exists := timers[event.Name]; exists {
				t.Stop()
			}

			filePath := event.Name
			timers[event.Name] = time.AfterFunc(w.debounce, func() {
				timerMu.Lock()
				delete(timers, filePath)
				timerMu.Unlock()

				select {
				case <-ctx.Done():
				case outEvents <- WatchEvent{
					ConversationID: convID,
					FilePath:       filePath,
					IsTranscript:   isTranscript,
				}:
				}
			})
			timerMu.Unlock()
		}
	}
}

// Close closes the underlying filesystem watcher.
func (w *Watcher) Close() error {
	return w.fsw.Close()
}
