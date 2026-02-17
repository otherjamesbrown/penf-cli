package batch

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestProgress(t *testing.T) {
	p := NewProgress(100)

	if p.TotalFiles != 100 {
		t.Errorf("unexpected total files: %d", p.TotalFiles)
	}
	if p.Status != "pending" {
		t.Errorf("unexpected status: %s", p.Status)
	}

	p.Start()
	if p.Status != "running" {
		t.Errorf("expected running status, got: %s", p.Status)
	}

	p.SetCurrentFile("/path/to/file.eml")
	if p.CurrentFile != "/path/to/file.eml" {
		t.Errorf("unexpected current file: %s", p.CurrentFile)
	}

	p.RecordImported()
	if p.ImportedCount != 1 || p.ProcessedCount != 1 {
		t.Errorf("unexpected counts after import: imported=%d, processed=%d",
			p.ImportedCount, p.ProcessedCount)
	}

	p.RecordSkipped()
	if p.SkippedCount != 1 || p.ProcessedCount != 2 {
		t.Errorf("unexpected counts after skip: skipped=%d, processed=%d",
			p.SkippedCount, p.ProcessedCount)
	}

	p.RecordFailed()
	if p.FailedCount != 1 || p.ProcessedCount != 3 {
		t.Errorf("unexpected counts after fail: failed=%d, processed=%d",
			p.FailedCount, p.ProcessedCount)
	}

	p.Complete(true)
	if p.Status != "completed" {
		t.Errorf("expected completed status, got: %s", p.Status)
	}
}

func TestProgressSnapshot(t *testing.T) {
	p := NewProgress(100)
	p.Start()

	// Process some files
	for i := 0; i < 50; i++ {
		p.RecordImported()
	}
	for i := 0; i < 10; i++ {
		p.RecordSkipped()
	}
	for i := 0; i < 5; i++ {
		p.RecordFailed()
	}

	snapshot := p.Snapshot()

	if snapshot.TotalFiles != 100 {
		t.Errorf("unexpected total: %d", snapshot.TotalFiles)
	}
	if snapshot.ProcessedCount != 65 {
		t.Errorf("unexpected processed: %d", snapshot.ProcessedCount)
	}
	if snapshot.ImportedCount != 50 {
		t.Errorf("unexpected imported: %d", snapshot.ImportedCount)
	}
	if snapshot.SkippedCount != 10 {
		t.Errorf("unexpected skipped: %d", snapshot.SkippedCount)
	}
	if snapshot.FailedCount != 5 {
		t.Errorf("unexpected failed: %d", snapshot.FailedCount)
	}

	pct := snapshot.PercentComplete()
	if pct != 65 {
		t.Errorf("unexpected percent complete: %f", pct)
	}

	if snapshot.IsComplete() {
		t.Error("should not be complete yet")
	}
}

func TestProgressCancel(t *testing.T) {
	p := NewProgress(100)
	p.Start()
	p.Cancel()

	if p.Status != "cancelled" {
		t.Errorf("expected cancelled status, got: %s", p.Status)
	}
}

func TestProgressCallback(t *testing.T) {
	p := NewProgress(10)

	var callbackCount int32
	p.SetOnUpdate(func(*Progress) {
		atomic.AddInt32(&callbackCount, 1)
	})

	p.Start()
	time.Sleep(10 * time.Millisecond) // Allow callback goroutine to run

	p.RecordImported()
	time.Sleep(10 * time.Millisecond)

	if atomic.LoadInt32(&callbackCount) < 2 {
		t.Errorf("expected at least 2 callbacks, got: %d", atomic.LoadInt32(&callbackCount))
	}
}

func TestProgressSnapshotEstimation(t *testing.T) {
	p := NewProgress(100)
	p.Start()

	// Process 25 files
	for i := 0; i < 25; i++ {
		p.RecordImported()
	}

	snapshot := p.Snapshot()

	if snapshot.EstimatedRemainingSeconds == nil {
		t.Error("expected estimated remaining to be calculated")
	}

	// 25% done, so remaining should be roughly 3x elapsed
	if *snapshot.EstimatedRemainingSeconds <= 0 {
		t.Error("estimated remaining should be positive")
	}
}

func TestProgressSnapshotIsSuccess(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*Progress)
		expected bool
	}{
		{
			name: "completed with no failures",
			setup: func(p *Progress) {
				p.Start()
				p.RecordImported()
				p.Complete(true)
			},
			expected: true,
		},
		{
			name: "completed with failures",
			setup: func(p *Progress) {
				p.Start()
				p.RecordFailed()
				p.Complete(false)
			},
			expected: false,
		},
		{
			name: "not completed",
			setup: func(p *Progress) {
				p.Start()
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewProgress(10)
			tt.setup(p)
			snapshot := p.Snapshot()
			if snapshot.IsSuccess() != tt.expected {
				t.Errorf("expected IsSuccess=%v, got %v", tt.expected, snapshot.IsSuccess())
			}
		})
	}
}

func TestProcessorConfig(t *testing.T) {
	cfg := ProcessorConfig{
		Concurrency: 8,
		TenantID:    "tenant-123",
		SourceTag:   "test-source",
		Labels:      []string{"label1", "label2"},
		DryRun:      true,
	}

	if cfg.Concurrency != 8 {
		t.Errorf("unexpected concurrency: %d", cfg.Concurrency)
	}
	if cfg.TenantID != "tenant-123" {
		t.Errorf("unexpected tenant id: %s", cfg.TenantID)
	}
	if len(cfg.Labels) != 2 {
		t.Errorf("unexpected labels count: %d", len(cfg.Labels))
	}
	if !cfg.DryRun {
		t.Error("expected dry run to be true")
	}
}

func TestProcessResult(t *testing.T) {
	result := ProcessResult{
		JobID:         "job-123",
		TotalFiles:    100,
		ImportedCount: 90,
		SkippedCount:  5,
		FailedCount:   5,
		StartedAt:     time.Now().Add(-time.Minute),
		CompletedAt:   time.Now(),
		Success:       false,
		Errors: []FileError{
			{FilePath: "/path/to/bad.eml", Error: "parse error"},
		},
	}

	if result.TotalFiles != 100 {
		t.Errorf("unexpected total files: %d", result.TotalFiles)
	}
	if result.ImportedCount+result.SkippedCount+result.FailedCount != 100 {
		t.Error("counts should add up to total")
	}
	if result.Success {
		t.Error("expected success to be false with failures")
	}
	if len(result.Errors) != 1 {
		t.Errorf("unexpected error count: %d", len(result.Errors))
	}
}

func TestFileError(t *testing.T) {
	fe := FileError{
		FilePath: "/path/to/file.eml",
		Error:    "invalid format",
	}

	if fe.FilePath != "/path/to/file.eml" {
		t.Errorf("unexpected file path: %s", fe.FilePath)
	}
	if fe.Error != "invalid format" {
		t.Errorf("unexpected error: %s", fe.Error)
	}
}

func TestDefaultConcurrency(t *testing.T) {
	if DefaultConcurrency != 4 {
		t.Errorf("unexpected default concurrency: %d", DefaultConcurrency)
	}
}
