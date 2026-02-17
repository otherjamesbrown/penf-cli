// Package batch provides batch processing for email ingest.
package batch

import (
	"sync"
	"time"
)

// Progress tracks the progress of a batch ingest operation.
type Progress struct {
	mu sync.RWMutex

	// Counts
	TotalFiles     int
	ProcessedCount int
	ImportedCount  int
	SkippedCount   int
	FailedCount    int

	// Current state
	CurrentFile    string
	Status         string
	processedFiles []string

	// Timing
	StartedAt time.Time
	UpdatedAt time.Time

	// Callbacks
	onUpdate func(*Progress)
}

// NewProgress creates a new progress tracker.
func NewProgress(totalFiles int) *Progress {
	return &Progress{
		TotalFiles: totalFiles,
		Status:     "pending",
		StartedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
}

// SetOnUpdate sets a callback function called on each update.
func (p *Progress) SetOnUpdate(fn func(*Progress)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onUpdate = fn
}

// Start marks the progress as started.
func (p *Progress) Start() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Status = "running"
	p.StartedAt = time.Now()
	p.UpdatedAt = time.Now()
	p.notifyUpdate()
}

// SetCurrentFile updates the current file being processed.
func (p *Progress) SetCurrentFile(path string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.CurrentFile = path
	p.UpdatedAt = time.Now()
	p.notifyUpdate()
}

// RecordImported increments the imported and processed counts.
func (p *Progress) RecordImported() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ImportedCount++
	p.ProcessedCount++
	if p.CurrentFile != "" {
		p.processedFiles = append(p.processedFiles, p.CurrentFile)
	}
	p.UpdatedAt = time.Now()
	p.notifyUpdate()
}

// RecordSkipped increments the skipped and processed counts.
func (p *Progress) RecordSkipped() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.SkippedCount++
	p.ProcessedCount++
	if p.CurrentFile != "" {
		p.processedFiles = append(p.processedFiles, p.CurrentFile)
	}
	p.UpdatedAt = time.Now()
	p.notifyUpdate()
}

// RecordFailed increments the failed and processed counts.
func (p *Progress) RecordFailed() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.FailedCount++
	p.ProcessedCount++
	if p.CurrentFile != "" {
		p.processedFiles = append(p.processedFiles, p.CurrentFile)
	}
	p.UpdatedAt = time.Now()
	p.notifyUpdate()
}

// ProcessedFiles returns a copy of the list of processed file paths.
func (p *Progress) ProcessedFiles() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := make([]string, len(p.processedFiles))
	copy(result, p.processedFiles)
	return result
}

// Complete marks the progress as completed.
func (p *Progress) Complete(success bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if success {
		p.Status = "completed"
	} else {
		p.Status = "failed"
	}
	p.UpdatedAt = time.Now()
	p.notifyUpdate()
}

// Cancel marks the progress as cancelled.
func (p *Progress) Cancel() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Status = "cancelled"
	p.UpdatedAt = time.Now()
	p.notifyUpdate()
}

// Snapshot returns a read-only copy of the current progress.
func (p *Progress) Snapshot() ProgressSnapshot {
	p.mu.RLock()
	defer p.mu.RUnlock()

	elapsed := time.Since(p.StartedAt).Seconds()
	var estimatedRemaining *float64
	if p.ProcessedCount > 0 {
		remaining := p.TotalFiles - p.ProcessedCount
		rate := elapsed / float64(p.ProcessedCount)
		est := rate * float64(remaining)
		estimatedRemaining = &est
	}

	return ProgressSnapshot{
		TotalFiles:                p.TotalFiles,
		ProcessedCount:            p.ProcessedCount,
		ImportedCount:             p.ImportedCount,
		SkippedCount:              p.SkippedCount,
		FailedCount:               p.FailedCount,
		CurrentFile:               p.CurrentFile,
		Status:                    p.Status,
		StartedAt:                 p.StartedAt,
		ElapsedSeconds:            elapsed,
		EstimatedRemainingSeconds: estimatedRemaining,
	}
}

// notifyUpdate calls the update callback if set.
// Must be called with lock held.
func (p *Progress) notifyUpdate() {
	if p.onUpdate != nil {
		// Make a copy to avoid holding lock during callback
		snapshot := &Progress{
			TotalFiles:     p.TotalFiles,
			ProcessedCount: p.ProcessedCount,
			ImportedCount:  p.ImportedCount,
			SkippedCount:   p.SkippedCount,
			FailedCount:    p.FailedCount,
			CurrentFile:    p.CurrentFile,
			Status:         p.Status,
			StartedAt:      p.StartedAt,
			UpdatedAt:      p.UpdatedAt,
		}
		go p.onUpdate(snapshot)
	}
}

// ProgressSnapshot is an immutable snapshot of progress state.
type ProgressSnapshot struct {
	TotalFiles                int
	ProcessedCount            int
	ImportedCount             int
	SkippedCount              int
	FailedCount               int
	CurrentFile               string
	Status                    string
	StartedAt                 time.Time
	ElapsedSeconds            float64
	EstimatedRemainingSeconds *float64
}

// PercentComplete returns the percentage of files processed.
func (s ProgressSnapshot) PercentComplete() float64 {
	if s.TotalFiles == 0 {
		return 0
	}
	return float64(s.ProcessedCount) / float64(s.TotalFiles) * 100
}

// IsComplete returns true if all files have been processed.
func (s ProgressSnapshot) IsComplete() bool {
	return s.ProcessedCount >= s.TotalFiles
}

// IsSuccess returns true if the job completed successfully (no failures).
func (s ProgressSnapshot) IsSuccess() bool {
	return s.Status == "completed" && s.FailedCount == 0
}
