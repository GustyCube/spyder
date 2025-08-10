package ui

import (
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// InteractiveLogger wraps zap logger with progress support
type InteractiveLogger struct {
	logger      *zap.SugaredLogger
	mu          sync.Mutex
	lastLine    string
	isProgress  bool
	output      io.Writer
	progressBar *ProgressBar
	stats       *Stats
	spinner     *Spinner
	showProgress bool
}

// NewInteractiveLogger creates a new interactive logger
func NewInteractiveLogger(logger *zap.SugaredLogger, showProgress bool) *InteractiveLogger {
	return &InteractiveLogger{
		logger:       logger,
		output:       os.Stdout,
		stats:        NewStats(),
		showProgress: showProgress && isTerminal(os.Stdout),
	}
}

// isTerminal checks if the output is a terminal
func isTerminal(f *os.File) bool {
	// Simple check - in a real implementation you'd use a proper terminal detection library
	stat, err := f.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}

// SetProgress updates the progress display
func (il *InteractiveLogger) SetProgress(message string) {
	if !il.showProgress {
		return
	}

	il.mu.Lock()
	defer il.mu.Unlock()

	// Clear previous line if it was progress
	if il.isProgress && il.lastLine != "" {
		il.clearLine()
	}

	// Write new progress line
	il.output.Write([]byte(message + "\r"))
	il.lastLine = message
	il.isProgress = true
}

// LogInfo logs an info message, clearing progress if needed
func (il *InteractiveLogger) LogInfo(message string, args ...interface{}) {
	il.clearProgressAndLog(func() {
		il.logger.Infow(message, args...)
	})
}

// LogWarn logs a warning message
func (il *InteractiveLogger) LogWarn(message string, args ...interface{}) {
	il.clearProgressAndLog(func() {
		il.logger.Warnw(message, args...)
	})
}

// LogError logs an error message
func (il *InteractiveLogger) LogError(message string, args ...interface{}) {
	il.clearProgressAndLog(func() {
		il.logger.Errorw(message, args...)
	})
}

// LogFatal logs a fatal message
func (il *InteractiveLogger) LogFatal(message string, args ...interface{}) {
	il.clearProgressAndLog(func() {
		il.logger.Fatalw(message, args...)
	})
}

// clearProgressAndLog clears progress and executes log function
func (il *InteractiveLogger) clearProgressAndLog(logFn func()) {
	il.mu.Lock()
	defer il.mu.Unlock()

	if il.showProgress && il.isProgress {
		il.clearLine()
		il.output.Write([]byte("\n"))
	}

	logFn()

	il.isProgress = false
	il.lastLine = ""
}

// clearLine clears the current line
func (il *InteractiveLogger) clearLine() {
	if il.lastLine != "" {
		spaces := strings.Repeat(" ", len(il.lastLine))
		il.output.Write([]byte("\r" + spaces + "\r"))
	}
}

// StartSpinner starts a spinner with a message
func (il *InteractiveLogger) StartSpinner(message string) {
	if !il.showProgress {
		return
	}

	il.mu.Lock()
	defer il.mu.Unlock()

	il.spinner = NewSpinner(message)
	il.spinner.Start()

	// Start spinner update loop
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if il.spinner == nil || !il.spinner.IsActive() {
					return
				}
				il.SetProgress(il.spinner.String())
			}
		}
	}()
}

// StopSpinner stops the current spinner
func (il *InteractiveLogger) StopSpinner() {
	il.mu.Lock()
	defer il.mu.Unlock()

	if il.spinner != nil {
		il.spinner.Stop()
		il.spinner = nil
	}

	if il.showProgress && il.isProgress {
		il.clearLine()
		il.isProgress = false
		il.lastLine = ""
	}
}

// UpdateProgress updates processing statistics and displays progress
func (il *InteractiveLogger) UpdateProgress(processed, successful, failed, edges int64) {
	if !il.showProgress {
		return
	}

	il.stats.mu.Lock()
	il.stats.processed = processed
	il.stats.successful = successful
	il.stats.failed = failed
	il.stats.edges = edges
	il.stats.mu.Unlock()

	if il.stats.ShouldLog() {
		message := il.stats.LogAndReset()
		il.SetProgress(message)
	}

	// Update progress bar if available
	if il.stats.progressBar != nil {
		il.SetProgress(il.stats.GetProgressBar())
	}
}

// SetTotal sets the total number of items for progress tracking
func (il *InteractiveLogger) SetTotal(total int64) {
	if il.showProgress {
		il.stats.SetTotal(total)
	}
}

// Finish completes progress tracking and shows summary
func (il *InteractiveLogger) Finish() {
	if !il.showProgress {
		return
	}

	il.mu.Lock()
	defer il.mu.Unlock()

	il.stats.Finish()

	// Clear progress and show final summary
	if il.isProgress {
		il.clearLine()
		il.output.Write([]byte("\n"))
	}

	summary := il.stats.Summary()
	il.logger.Info(summary)

	il.isProgress = false
	il.lastLine = ""
}

// GetStats returns the current statistics
func (il *InteractiveLogger) GetStats() *Stats {
	return il.stats
}

// EnableColors enables colored output (placeholder for future implementation)
func (il *InteractiveLogger) EnableColors(enabled bool) {
	// Future implementation for colored output
}

// Sync syncs the underlying logger
func (il *InteractiveLogger) Sync() error {
	return il.logger.Sync()
}