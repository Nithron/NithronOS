// Package watcher provides file system watching functionality.
package watcher

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog"
)

// Event represents a file system event.
type Event struct {
	Path      string
	Op        Operation
	IsDir     bool
	Timestamp time.Time
}

// Operation represents the type of file operation.
type Operation int

const (
	OpCreate Operation = iota
	OpWrite
	OpRemove
	OpRename
	OpChmod
)

func (o Operation) String() string {
	switch o {
	case OpCreate:
		return "create"
	case OpWrite:
		return "write"
	case OpRemove:
		return "remove"
	case OpRename:
		return "rename"
	case OpChmod:
		return "chmod"
	default:
		return "unknown"
	}
}

// Watcher watches a directory for file system changes.
type Watcher struct {
	watcher         *fsnotify.Watcher
	rootPath        string
	excludePatterns []string
	events          chan Event
	errors          chan error
	logger          zerolog.Logger

	// Debouncing
	debounceTime time.Duration
	pending      map[string]*pendingEvent
	pendingMu    sync.Mutex
	
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

type pendingEvent struct {
	event Event
	timer *time.Timer
}

// Config holds watcher configuration.
type Config struct {
	RootPath        string
	ExcludePatterns []string
	DebounceTime    time.Duration
	BufferSize      int
}

// DefaultConfig returns a default configuration.
func DefaultConfig(rootPath string) Config {
	return Config{
		RootPath: rootPath,
		ExcludePatterns: []string{
			"*.tmp",
			"*.temp",
			"~$*",
			".DS_Store",
			"Thumbs.db",
			"desktop.ini",
			".git/**",
			".svn/**",
			"node_modules/**",
			"__pycache__/**",
			"*.pyc",
			".sync_*",
			"*.nstmp",
		},
		DebounceTime: 500 * time.Millisecond,
		BufferSize:   1000,
	}
}

// New creates a new file system watcher.
func New(cfg Config, logger zerolog.Logger) (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	w := &Watcher{
		watcher:         fsWatcher,
		rootPath:        cfg.RootPath,
		excludePatterns: cfg.ExcludePatterns,
		events:          make(chan Event, cfg.BufferSize),
		errors:          make(chan error, 10),
		logger:          logger.With().Str("component", "watcher").Logger(),
		debounceTime:    cfg.DebounceTime,
		pending:         make(map[string]*pendingEvent),
		ctx:             ctx,
		cancel:          cancel,
	}

	return w, nil
}

// Start starts watching the root directory.
func (w *Watcher) Start() error {
	// Add root path and all subdirectories
	if err := w.addRecursive(w.rootPath); err != nil {
		return err
	}

	// Start event processing
	w.wg.Add(1)
	go w.processEvents()

	w.logger.Info().Str("path", w.rootPath).Msg("File watcher started")
	return nil
}

// Stop stops the watcher.
func (w *Watcher) Stop() error {
	w.cancel()
	w.wg.Wait()
	
	close(w.events)
	close(w.errors)
	
	return w.watcher.Close()
}

// Events returns the events channel.
func (w *Watcher) Events() <-chan Event {
	return w.events
}

// Errors returns the errors channel.
func (w *Watcher) Errors() <-chan error {
	return w.errors
}

// addRecursive adds a directory and all subdirectories to the watcher.
func (w *Watcher) addRecursive(path string) error {
	return filepath.Walk(path, func(walkPath string, info os.FileInfo, err error) error {
		if err != nil {
			// Log but don't fail on permission errors
			if os.IsPermission(err) {
				w.logger.Warn().Str("path", walkPath).Msg("Permission denied, skipping")
				return nil
			}
			return err
		}

		// Only watch directories
		if !info.IsDir() {
			return nil
		}

		// Check excludes
		if w.shouldExclude(walkPath) {
			return filepath.SkipDir
		}

		if err := w.watcher.Add(walkPath); err != nil {
			w.logger.Warn().Err(err).Str("path", walkPath).Msg("Failed to add path to watcher")
			return nil
		}

		return nil
	})
}

// shouldExclude checks if a path should be excluded.
func (w *Watcher) shouldExclude(path string) bool {
	relPath, err := filepath.Rel(w.rootPath, path)
	if err != nil {
		return false
	}

	name := filepath.Base(path)

	for _, pattern := range w.excludePatterns {
		// Check against filename
		if matched, _ := filepath.Match(pattern, name); matched {
			return true
		}

		// Check against relative path for patterns with /**
		if strings.Contains(pattern, "**") {
			simplePattern := strings.ReplaceAll(pattern, "**", "*")
			if matched, _ := filepath.Match(simplePattern, relPath); matched {
				return true
			}
			// Also check if any path component matches
			parts := strings.Split(relPath, string(filepath.Separator))
			basePattern := strings.TrimSuffix(strings.TrimPrefix(pattern, "**/"), "/**")
			for _, part := range parts {
				if part == basePattern {
					return true
				}
			}
		}
	}

	return false
}

// processEvents processes raw fsnotify events.
func (w *Watcher) processEvents() {
	defer w.wg.Done()

	for {
		select {
		case <-w.ctx.Done():
			// Flush pending events
			w.pendingMu.Lock()
			for _, pe := range w.pending {
				pe.timer.Stop()
			}
			w.pending = nil
			w.pendingMu.Unlock()
			return

		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			w.handleEvent(event)

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			select {
			case w.errors <- err:
			default:
				w.logger.Error().Err(err).Msg("Error channel full, dropping error")
			}
		}
	}
}

// handleEvent handles a single fsnotify event.
func (w *Watcher) handleEvent(fsEvent fsnotify.Event) {
	// Check excludes
	if w.shouldExclude(fsEvent.Name) {
		return
	}

	// Determine operation
	var op Operation
	switch {
	case fsEvent.Has(fsnotify.Create):
		op = OpCreate
		// If a directory was created, add it to the watcher
		if info, err := os.Stat(fsEvent.Name); err == nil && info.IsDir() {
			w.watcher.Add(fsEvent.Name)
		}
	case fsEvent.Has(fsnotify.Write):
		op = OpWrite
	case fsEvent.Has(fsnotify.Remove):
		op = OpRemove
		// Remove from watcher (ignore error if not watched)
		w.watcher.Remove(fsEvent.Name)
	case fsEvent.Has(fsnotify.Rename):
		op = OpRename
	case fsEvent.Has(fsnotify.Chmod):
		op = OpChmod
		// Ignore chmod-only events
		return
	default:
		return
	}

	// Check if it's a directory
	isDir := false
	if info, err := os.Stat(fsEvent.Name); err == nil {
		isDir = info.IsDir()
	}

	event := Event{
		Path:      fsEvent.Name,
		Op:        op,
		IsDir:     isDir,
		Timestamp: time.Now(),
	}

	// Debounce the event
	w.debounce(event)
}

// debounce debounces events for the same path.
func (w *Watcher) debounce(event Event) {
	w.pendingMu.Lock()
	defer w.pendingMu.Unlock()

	key := event.Path

	// Cancel existing timer
	if pe, exists := w.pending[key]; exists {
		pe.timer.Stop()
		// Merge events - prefer writes over creates
		if event.Op == OpWrite && pe.event.Op == OpCreate {
			event.Op = OpCreate // Keep as create since we're writing to a new file
		}
	}

	// Set up new debounce timer
	pe := &pendingEvent{
		event: event,
	}
	pe.timer = time.AfterFunc(w.debounceTime, func() {
		w.pendingMu.Lock()
		delete(w.pending, key)
		w.pendingMu.Unlock()

		select {
		case w.events <- event:
		default:
			w.logger.Warn().Str("path", event.Path).Msg("Event channel full, dropping event")
		}
	})

	w.pending[key] = pe
}

// GetRelativePath returns the path relative to the root.
func (w *Watcher) GetRelativePath(absPath string) (string, error) {
	return filepath.Rel(w.rootPath, absPath)
}

// GetAbsolutePath returns the absolute path from a relative path.
func (w *Watcher) GetAbsolutePath(relPath string) string {
	return filepath.Join(w.rootPath, relPath)
}

