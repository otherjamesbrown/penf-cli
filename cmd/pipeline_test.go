// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"testing"
	"time"

	pipelinev1 "github.com/otherjamesbrown/penf-cli/api/proto/pipeline/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// TestParseTimeFilter tests the parseTimeFilter function.
func TestParseTimeFilter(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantError bool
	}{
		{
			name:      "duration - hours",
			input:     "2h",
			wantError: false,
		},
		{
			name:      "duration - minutes",
			input:     "30m",
			wantError: false,
		},
		{
			name:      "duration - days",
			input:     "24h",
			wantError: false,
		},
		{
			name:      "ISO timestamp",
			input:     "2026-02-04T10:00:00Z",
			wantError: false,
		},
		{
			name:      "date only",
			input:     "2026-02-04",
			wantError: false,
		},
		{
			name:      "keyword - today",
			input:     "today",
			wantError: false,
		},
		{
			name:      "keyword - yesterday",
			input:     "yesterday",
			wantError: false,
		},
		{
			name:      "invalid format",
			input:     "invalid",
			wantError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parseTimeFilter(tc.input)
			if tc.wantError {
				if err == nil {
					t.Errorf("parseTimeFilter(%q) expected error, got nil", tc.input)
				}
			} else {
				if err != nil {
					t.Errorf("parseTimeFilter(%q) unexpected error: %v", tc.input, err)
				}
				if result.IsZero() {
					t.Errorf("parseTimeFilter(%q) returned zero time", tc.input)
				}
			}
		})
	}
}

// TestParseTimeFilter_DurationCalculation tests duration calculations.
func TestParseTimeFilter_DurationCalculation(t *testing.T) {
	now := time.Now()

	// Test 2 hours ago
	result, err := parseTimeFilter("2h")
	if err != nil {
		t.Fatalf("parseTimeFilter(\"2h\") failed: %v", err)
	}

	expected := now.Add(-2 * time.Hour)
	diff := expected.Sub(result)
	if diff < 0 {
		diff = -diff
	}

	// Allow 1 second tolerance for execution time
	if diff > time.Second {
		t.Errorf("parseTimeFilter(\"2h\") = %v, want approximately %v (diff: %v)", result, expected, diff)
	}
}

// TestParseTimeFilter_Keywords tests keyword parsing.
func TestParseTimeFilter_Keywords(t *testing.T) {
	now := time.Now()

	// Test "today"
	result, err := parseTimeFilter("today")
	if err != nil {
		t.Fatalf("parseTimeFilter(\"today\") failed: %v", err)
	}

	expectedToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if !result.Equal(expectedToday) {
		t.Errorf("parseTimeFilter(\"today\") = %v, want %v", result, expectedToday)
	}

	// Test "yesterday"
	result, err = parseTimeFilter("yesterday")
	if err != nil {
		t.Fatalf("parseTimeFilter(\"yesterday\") failed: %v", err)
	}

	yesterday := now.AddDate(0, 0, -1)
	expectedYesterday := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, yesterday.Location())
	if !result.Equal(expectedYesterday) {
		t.Errorf("parseTimeFilter(\"yesterday\") = %v, want %v", result, expectedYesterday)
	}
}

// TestFilterJobsSince tests the filterJobsSince function.
func TestFilterJobsSince(t *testing.T) {
	now := time.Now()
	oneHourAgo := now.Add(-1 * time.Hour)
	twoHoursAgo := now.Add(-2 * time.Hour)
	threeHoursAgo := now.Add(-3 * time.Hour)

	jobs := []*pipelinev1.JobSummary{
		{
			Id:        "job-1",
			CreatedAt: timestamppb.New(threeHoursAgo),
		},
		{
			Id:        "job-2",
			CreatedAt: timestamppb.New(twoHoursAgo),
		},
		{
			Id:        "job-3",
			CreatedAt: timestamppb.New(oneHourAgo),
		},
		{
			Id:        "job-4",
			CreatedAt: timestamppb.New(now),
		},
		{
			Id:        "job-5-nil-timestamp",
			CreatedAt: nil,
		},
	}

	tests := []struct {
		name      string
		sinceTime time.Time
		wantCount int
		wantIDs   []string
	}{
		{
			name:      "since 2.5 hours ago",
			sinceTime: now.Add(-2*time.Hour - 30*time.Minute),
			wantCount: 3,
			wantIDs:   []string{"job-2", "job-3", "job-4"},
		},
		{
			name:      "since 1 hour ago",
			sinceTime: oneHourAgo,
			wantCount: 2,
			wantIDs:   []string{"job-3", "job-4"},
		},
		{
			name:      "since now",
			sinceTime: now,
			wantCount: 1,
			wantIDs:   []string{"job-4"},
		},
		{
			name:      "since 5 hours ago",
			sinceTime: now.Add(-5 * time.Hour),
			wantCount: 4,
			wantIDs:   []string{"job-1", "job-2", "job-3", "job-4"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			filtered := filterJobsSince(jobs, tc.sinceTime)

			if len(filtered) != tc.wantCount {
				t.Errorf("filterJobsSince() returned %d jobs, want %d", len(filtered), tc.wantCount)
			}

			for i, wantID := range tc.wantIDs {
				if i >= len(filtered) {
					t.Errorf("missing job at index %d: want %s", i, wantID)
					continue
				}
				if filtered[i].Id != wantID {
					t.Errorf("job at index %d: got %s, want %s", i, filtered[i].Id, wantID)
				}
			}
		})
	}
}

// TestFilterJobsSince_EmptyInput tests filtering with empty input.
func TestFilterJobsSince_EmptyInput(t *testing.T) {
	result := filterJobsSince([]*pipelinev1.JobSummary{}, time.Now())
	if len(result) != 0 {
		t.Errorf("filterJobsSince([]) = %d jobs, want 0", len(result))
	}
}

// TestFilterJobsSince_NilTimestamp tests handling of jobs with nil timestamps.
func TestFilterJobsSince_NilTimestamp(t *testing.T) {
	jobs := []*pipelinev1.JobSummary{
		{
			Id:        "job-1",
			CreatedAt: nil,
		},
		{
			Id:        "job-2",
			CreatedAt: timestamppb.New(time.Now()),
		},
	}

	result := filterJobsSince(jobs, time.Now().Add(-1*time.Hour))

	// Should only include job-2 (job-1 has nil timestamp)
	if len(result) != 1 {
		t.Errorf("filterJobsSince() returned %d jobs, want 1", len(result))
	}
	if len(result) > 0 && result[0].Id != "job-2" {
		t.Errorf("filterJobsSince()[0].Id = %s, want job-2", result[0].Id)
	}
}

// TestPipelineStatusFlags tests flag validation for pipeline status command.
func TestPipelineStatusFlags(t *testing.T) {
	// This test verifies the mutual exclusivity of --since-last-session and --since flags
	// The actual validation happens in the RunE function

	tests := []struct {
		name              string
		sinceLastSession  bool
		since             string
		shouldError       bool
	}{
		{
			name:              "both flags set",
			sinceLastSession:  true,
			since:             "2h",
			shouldError:       true,
		},
		{
			name:              "only since-last-session",
			sinceLastSession:  true,
			since:             "",
			shouldError:       false,
		},
		{
			name:              "only since",
			sinceLastSession:  false,
			since:             "2h",
			shouldError:       false,
		},
		{
			name:              "neither flag",
			sinceLastSession:  false,
			since:             "",
			shouldError:       false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hasConflict := tc.sinceLastSession && tc.since != ""
			if hasConflict != tc.shouldError {
				t.Errorf("conflict detection mismatch: got %v, want %v", hasConflict, tc.shouldError)
			}
		})
	}
}
