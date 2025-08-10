package ui

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// ProgressBar represents a simple progress bar
type ProgressBar struct {
	mu          sync.RWMutex
	total       int64
	current     int64
	width       int
	startTime   time.Time
	lastUpdate  time.Time
	description string
	showRate    bool
	showETA     bool
	finished    bool
}

// NewProgressBar creates a new progress bar
func NewProgressBar(total int64, description string) *ProgressBar {
	return &ProgressBar{
		total:       total,
		width:       50,
		startTime:   time.Now(),
		lastUpdate:  time.Now(),
		description: description,
		showRate:    true,
		showETA:     true,
	}
}

// Add increments the progress
func (pb *ProgressBar) Add(n int64) {
	pb.mu.Lock()
	defer pb.mu.Unlock()
	
	pb.current += n
	if pb.current > pb.total {
		pb.current = pb.total
	}
	pb.lastUpdate = time.Now()
}

// Set sets the current progress
func (pb *ProgressBar) Set(current int64) {
	pb.mu.Lock()
	defer pb.mu.Unlock()
	
	pb.current = current
	if pb.current > pb.total {
		pb.current = pb.total
	}
	pb.lastUpdate = time.Now()
}

// Finish marks the progress as complete
func (pb *ProgressBar) Finish() {
	pb.mu.Lock()
	defer pb.mu.Unlock()
	
	pb.current = pb.total
	pb.finished = true
	pb.lastUpdate = time.Now()
}

// String returns the progress bar as a string
func (pb *ProgressBar) String() string {
	pb.mu.RLock()
	defer pb.mu.RUnlock()
	
	percent := float64(pb.current) / float64(pb.total) * 100
	filled := int(float64(pb.width) * percent / 100)
	
	bar := strings.Repeat("█", filled) + strings.Repeat("░", pb.width-filled)
	
	result := fmt.Sprintf("%s [%s] %d/%d (%.1f%%)",
		pb.description, bar, pb.current, pb.total, percent)
	
	if pb.showRate && pb.current > 0 {
		elapsed := pb.lastUpdate.Sub(pb.startTime).Seconds()
		rate := float64(pb.current) / elapsed
		result += fmt.Sprintf(" %.1f/s", rate)
	}
	
	if pb.showETA && pb.current > 0 && !pb.finished {
		elapsed := pb.lastUpdate.Sub(pb.startTime).Seconds()
		rate := float64(pb.current) / elapsed
		remaining := pb.total - pb.current
		eta := time.Duration(float64(remaining)/rate) * time.Second
		result += fmt.Sprintf(" ETA: %v", eta.Round(time.Second))
	}
	
	if pb.finished {
		elapsed := pb.lastUpdate.Sub(pb.startTime)
		result += fmt.Sprintf(" [DONE in %v]", elapsed.Round(time.Millisecond))
	}
	
	return result
}

// Stats represents processing statistics
type Stats struct {
	mu              sync.RWMutex
	processed       int64
	successful      int64
	failed          int64
	edges           int64
	startTime       time.Time
	lastLogTime     time.Time
	logInterval     time.Duration
	progressEnabled bool
	progressBar     *ProgressBar
}

// NewStats creates new processing statistics
func NewStats() *Stats {
	return &Stats{
		startTime:   time.Now(),
		lastLogTime: time.Now(),
		logInterval: 10 * time.Second,
	}
}

// SetTotal sets the total number of items to process
func (s *Stats) SetTotal(total int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if total > 0 {
		s.progressBar = NewProgressBar(total, "Processing domains")
		s.progressEnabled = true
	}
}

// IncrementProcessed increments processed count
func (s *Stats) IncrementProcessed() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.processed++
	if s.progressBar != nil {
		s.progressBar.Add(1)
	}
}

// IncrementSuccessful increments successful count
func (s *Stats) IncrementSuccessful() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.successful++
}

// IncrementFailed increments failed count
func (s *Stats) IncrementFailed() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failed++
}

// AddEdges adds to the edge count
func (s *Stats) AddEdges(count int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.edges += count
}

// ShouldLog returns true if it's time to log progress
func (s *Stats) ShouldLog() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	return time.Since(s.lastLogTime) >= s.logInterval
}

// LogAndReset logs current stats and resets log timer
func (s *Stats) LogAndReset() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.lastLogTime = time.Now()
	elapsed := s.lastLogTime.Sub(s.startTime)
	
	rate := float64(s.processed) / elapsed.Seconds()
	errorRate := float64(s.failed) / float64(s.processed) * 100
	
	message := fmt.Sprintf("Progress: %d processed, %d successful, %d failed (%.1f%% errors), %d edges, %.1f domains/sec",
		s.processed, s.successful, s.failed, errorRate, s.edges, rate)
		
	return message
}

// GetProgressBar returns the progress bar string if enabled
func (s *Stats) GetProgressBar() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if s.progressBar != nil {
		return s.progressBar.String()
	}
	return ""
}

// Finish marks processing as complete
func (s *Stats) Finish() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.progressBar != nil {
		s.progressBar.Finish()
	}
}

// Summary returns a final summary
func (s *Stats) Summary() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	elapsed := time.Since(s.startTime)
	rate := float64(s.processed) / elapsed.Seconds()
	errorRate := float64(s.failed) / float64(s.processed) * 100
	
	return fmt.Sprintf("Final summary: %d domains processed in %v (%.1f domains/sec), %d successful, %d failed (%.1f%% errors), %d edges discovered",
		s.processed, elapsed.Round(time.Millisecond), rate, s.successful, s.failed, errorRate, s.edges)
}

// Spinner represents a simple spinner for indeterminate progress
type Spinner struct {
	mu       sync.Mutex
	chars    []rune
	current  int
	active   bool
	message  string
	lastSpin time.Time
	interval time.Duration
}

// NewSpinner creates a new spinner
func NewSpinner(message string) *Spinner {
	return &Spinner{
		chars:    []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'},
		message:  message,
		interval: 100 * time.Millisecond,
	}
}

// Start starts the spinner
func (s *Spinner) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active = true
	s.lastSpin = time.Now()
}

// Stop stops the spinner
func (s *Spinner) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active = false
}

// String returns the current spinner state
func (s *Spinner) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if !s.active {
		return ""
	}
	
	now := time.Now()
	if now.Sub(s.lastSpin) >= s.interval {
		s.current = (s.current + 1) % len(s.chars)
		s.lastSpin = now
	}
	
	return fmt.Sprintf("%c %s", s.chars[s.current], s.message)
}

// IsActive returns whether the spinner is active
func (s *Spinner) IsActive() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active
}