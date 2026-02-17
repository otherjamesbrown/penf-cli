package products

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ==================== EventType Constants Tests ====================

func TestEventTypeConstants(t *testing.T) {
	assert.Equal(t, EventType("decision"), EventTypeDecision)
	assert.Equal(t, EventType("milestone"), EventTypeMilestone)
	assert.Equal(t, EventType("risk"), EventTypeRisk)
	assert.Equal(t, EventType("release"), EventTypeRelease)
	assert.Equal(t, EventType("competitor"), EventTypeCompetitor)
	assert.Equal(t, EventType("org_change"), EventTypeOrgChange)
	assert.Equal(t, EventType("market"), EventTypeMarket)
	assert.Equal(t, EventType("note"), EventTypeNote)
}

func TestEventTypeValidity(t *testing.T) {
	validTypes := []EventType{
		EventTypeDecision,
		EventTypeMilestone,
		EventTypeRisk,
		EventTypeRelease,
		EventTypeCompetitor,
		EventTypeOrgChange,
		EventTypeMarket,
		EventTypeNote,
	}

	for _, et := range validTypes {
		t.Run(string(et), func(t *testing.T) {
			assert.NotEmpty(t, et)
		})
	}
}

// ==================== EventVisibility Constants Tests ====================

func TestEventVisibilityConstants(t *testing.T) {
	assert.Equal(t, EventVisibility("internal"), EventVisibilityInternal)
	assert.Equal(t, EventVisibility("external"), EventVisibilityExternal)
}

// ==================== EventSourceType Constants Tests ====================

func TestEventSourceTypeConstants(t *testing.T) {
	assert.Equal(t, EventSourceType("manual"), EventSourceManual)
	assert.Equal(t, EventSourceType("derived"), EventSourceDerived)
}

// ==================== ProductEvent Struct Tests ====================

func TestProductEventStructure(t *testing.T) {
	now := time.Now()
	eventUUID := uuid.New()
	metadata := map[string]any{
		"source_meeting_id": int64(123),
		"attendees":         []string{"alice", "bob"},
	}

	event := &ProductEvent{
		ID:          1,
		EventUUID:   eventUUID,
		TenantID:    "tenant-123",
		ProductID:   10,
		EventType:   EventTypeDecision,
		Visibility:  EventVisibilityInternal,
		SourceType:  EventSourceManual,
		Title:       "Chose PostgreSQL for database",
		Description: "After evaluating options, team decided on PostgreSQL",
		OccurredAt:  now.Add(-24 * time.Hour),
		RecordedBy:  "john@example.com",
		Metadata:    metadata,
		CreatedAt:   now,
		UpdatedAt:   now,
		ProductName: "Test Product",
	}

	assert.Equal(t, int64(1), event.ID)
	assert.Equal(t, eventUUID, event.EventUUID)
	assert.Equal(t, "tenant-123", event.TenantID)
	assert.Equal(t, int64(10), event.ProductID)
	assert.Equal(t, EventTypeDecision, event.EventType)
	assert.Equal(t, EventVisibilityInternal, event.Visibility)
	assert.Equal(t, EventSourceManual, event.SourceType)
	assert.Equal(t, "Chose PostgreSQL for database", event.Title)
	assert.Contains(t, event.Description, "PostgreSQL")
	assert.Equal(t, "john@example.com", event.RecordedBy)
	require.NotNil(t, event.Metadata)
	assert.Equal(t, int64(123), event.Metadata["source_meeting_id"])
	assert.Equal(t, "Test Product", event.ProductName)
}

func TestProductEventMinimal(t *testing.T) {
	event := &ProductEvent{
		ID:         1,
		EventUUID:  uuid.New(),
		TenantID:   "tenant-123",
		ProductID:  10,
		EventType:  EventTypeNote,
		Visibility: EventVisibilityInternal,
		SourceType: EventSourceManual,
		Title:      "Simple note",
		OccurredAt: time.Now(),
	}

	assert.Equal(t, EventTypeNote, event.EventType)
	assert.Empty(t, event.Description)
	assert.Empty(t, event.RecordedBy)
	assert.Nil(t, event.Metadata)
	assert.Nil(t, event.Links)
}

func TestProductEventWithLinks(t *testing.T) {
	now := time.Now()
	event := &ProductEvent{
		ID:         1,
		EventUUID:  uuid.New(),
		TenantID:   "tenant-123",
		ProductID:  10,
		EventType:  EventTypeDecision,
		Visibility: EventVisibilityInternal,
		SourceType: EventSourceDerived,
		Title:      "Derived from meeting",
		OccurredAt: now,
		Links: []*ProductEventLink{
			{
				ID:               1,
				EventID:          1,
				LinkedEntityType: "meeting",
				LinkedEntityID:   100,
				LinkType:         "source",
				CreatedAt:        now,
			},
			{
				ID:               2,
				EventID:          1,
				LinkedEntityType: "email",
				LinkedEntityID:   200,
				LinkType:         "reference",
				CreatedAt:        now,
			},
		},
	}

	assert.Len(t, event.Links, 2)
	assert.Equal(t, "meeting", event.Links[0].LinkedEntityType)
	assert.Equal(t, "email", event.Links[1].LinkedEntityType)
}

// ==================== ProductEventLink Tests ====================

func TestProductEventLinkStructure(t *testing.T) {
	now := time.Now()
	link := &ProductEventLink{
		ID:               1,
		EventID:          10,
		LinkedEntityType: "meeting",
		LinkedEntityID:   100,
		LinkType:         "source",
		CreatedAt:        now,
	}

	assert.Equal(t, int64(1), link.ID)
	assert.Equal(t, int64(10), link.EventID)
	assert.Equal(t, "meeting", link.LinkedEntityType)
	assert.Equal(t, int64(100), link.LinkedEntityID)
	assert.Equal(t, "source", link.LinkType)
}

func TestProductEventLinkTypes(t *testing.T) {
	// Common linked entity types
	entityTypes := []string{"meeting", "email", "document", "source"}
	for _, et := range entityTypes {
		t.Run(et, func(t *testing.T) {
			link := ProductEventLink{LinkedEntityType: et}
			assert.Equal(t, et, link.LinkedEntityType)
		})
	}

	// Common link types
	linkTypes := []string{"source", "reference", "follow_up"}
	for _, lt := range linkTypes {
		t.Run(lt, func(t *testing.T) {
			link := ProductEventLink{LinkType: lt}
			assert.Equal(t, lt, link.LinkType)
		})
	}
}

// ==================== EventFilter Tests ====================

func TestEventFilterDefaults(t *testing.T) {
	filter := EventFilter{}

	assert.Empty(t, filter.TenantID)
	assert.Nil(t, filter.ProductID)
	assert.Nil(t, filter.EventTypes)
	assert.Nil(t, filter.Visibility)
	assert.Nil(t, filter.Since)
	assert.Nil(t, filter.Until)
	assert.Equal(t, 0, filter.Limit)
	assert.Equal(t, 0, filter.Offset)
}

func TestEventFilterFullyPopulated(t *testing.T) {
	productID := int64(10)
	visibility := EventVisibilityInternal
	since := time.Now().Add(-30 * 24 * time.Hour)
	until := time.Now()

	filter := EventFilter{
		TenantID:   "tenant-123",
		ProductID:  &productID,
		EventTypes: []EventType{EventTypeDecision, EventTypeMilestone},
		Visibility: &visibility,
		Since:      &since,
		Until:      &until,
		Limit:      50,
		Offset:     10,
	}

	assert.Equal(t, "tenant-123", filter.TenantID)
	require.NotNil(t, filter.ProductID)
	assert.Equal(t, int64(10), *filter.ProductID)
	assert.Len(t, filter.EventTypes, 2)
	assert.Contains(t, filter.EventTypes, EventTypeDecision)
	require.NotNil(t, filter.Visibility)
	assert.Equal(t, EventVisibilityInternal, *filter.Visibility)
	require.NotNil(t, filter.Since)
	require.NotNil(t, filter.Until)
	assert.Equal(t, 50, filter.Limit)
	assert.Equal(t, 10, filter.Offset)
}

func TestEventFilterDateRange(t *testing.T) {
	// Test filtering by date range
	now := time.Now()
	lastWeek := now.Add(-7 * 24 * time.Hour)
	lastMonth := now.Add(-30 * 24 * time.Hour)

	filter := EventFilter{
		TenantID: "tenant-123",
		Since:    &lastMonth,
		Until:    &lastWeek,
	}

	assert.True(t, filter.Since.Before(*filter.Until))
}

func TestEventFilterMultipleTypes(t *testing.T) {
	filter := EventFilter{
		TenantID: "tenant-123",
		EventTypes: []EventType{
			EventTypeDecision,
			EventTypeMilestone,
			EventTypeRelease,
		},
	}

	assert.Len(t, filter.EventTypes, 3)
	assert.Contains(t, filter.EventTypes, EventTypeDecision)
	assert.Contains(t, filter.EventTypes, EventTypeMilestone)
	assert.Contains(t, filter.EventTypes, EventTypeRelease)
	assert.NotContains(t, filter.EventTypes, EventTypeRisk)
}

// ==================== ContextWindow Tests ====================

func TestContextWindowStructure(t *testing.T) {
	now := time.Now()
	centerEvent := &ProductEvent{
		ID:        5,
		Title:     "Center Event",
		EventType: EventTypeDecision,
	}
	before1 := &ProductEvent{ID: 3, Title: "Before 1"}
	before2 := &ProductEvent{ID: 4, Title: "Before 2"}
	after1 := &ProductEvent{ID: 6, Title: "After 1"}
	after2 := &ProductEvent{ID: 7, Title: "After 2"}

	window := &ContextWindow{
		CenterEvent:  centerEvent,
		CenterTime:   now,
		EventsBefore: []*ProductEvent{before1, before2},
		EventsAfter:  []*ProductEvent{after1, after2},
		WindowStart:  now.Add(-7 * 24 * time.Hour),
		WindowEnd:    now.Add(7 * 24 * time.Hour),
	}

	assert.Equal(t, centerEvent, window.CenterEvent)
	assert.Equal(t, now, window.CenterTime)
	assert.Len(t, window.EventsBefore, 2)
	assert.Len(t, window.EventsAfter, 2)
	assert.True(t, window.WindowStart.Before(window.CenterTime))
	assert.True(t, window.WindowEnd.After(window.CenterTime))
}

func TestContextWindowWithNoCenterEvent(t *testing.T) {
	now := time.Now()
	window := &ContextWindow{
		CenterEvent:  nil,
		CenterTime:   now,
		EventsBefore: []*ProductEvent{},
		EventsAfter:  []*ProductEvent{},
		WindowStart:  now.Add(-7 * 24 * time.Hour),
		WindowEnd:    now.Add(7 * 24 * time.Hour),
	}

	assert.Nil(t, window.CenterEvent)
	assert.Empty(t, window.EventsBefore)
	assert.Empty(t, window.EventsAfter)
}

func TestContextWindowDuration(t *testing.T) {
	now := time.Now()
	start := now.Add(-30 * 24 * time.Hour)
	end := now.Add(30 * 24 * time.Hour)

	window := &ContextWindow{
		CenterTime:  now,
		WindowStart: start,
		WindowEnd:   end,
	}

	duration := window.WindowEnd.Sub(window.WindowStart)
	assert.Equal(t, 60*24*time.Hour, duration)
}

// ==================== Event Type Use Cases ====================

func TestEventTypeDecisionUseCase(t *testing.T) {
	event := &ProductEvent{
		ID:         1,
		EventType:  EventTypeDecision,
		Visibility: EventVisibilityInternal,
		Title:      "Selected Go as primary language",
		Description: "After team discussion, decided to use Go for better " +
			"performance and maintainability",
		Metadata: map[string]any{
			"alternatives_considered": []string{"Rust", "Python"},
			"decision_maker":          "CTO",
		},
	}

	assert.Equal(t, EventTypeDecision, event.EventType)
	assert.Contains(t, event.Description, "Go")
	assert.NotNil(t, event.Metadata["alternatives_considered"])
}

func TestEventTypeMilestoneUseCase(t *testing.T) {
	event := &ProductEvent{
		ID:          1,
		EventType:   EventTypeMilestone,
		Visibility:  EventVisibilityExternal,
		Title:       "GA Release",
		Description: "Product is now generally available",
		Metadata: map[string]any{
			"version": "1.0.0",
			"region":  "us-east",
		},
	}

	assert.Equal(t, EventTypeMilestone, event.EventType)
	assert.Equal(t, EventVisibilityExternal, event.Visibility)
}

func TestEventTypeRiskUseCase(t *testing.T) {
	event := &ProductEvent{
		ID:         1,
		EventType:  EventTypeRisk,
		Visibility: EventVisibilityInternal,
		Title:      "Dependency vulnerability discovered",
		Metadata: map[string]any{
			"severity": "high",
			"cve":      "CVE-2024-1234",
		},
	}

	assert.Equal(t, EventTypeRisk, event.EventType)
	assert.Equal(t, "high", event.Metadata["severity"])
}

func TestEventTypeCompetitorUseCase(t *testing.T) {
	event := &ProductEvent{
		ID:         1,
		EventType:  EventTypeCompetitor,
		Visibility: EventVisibilityInternal,
		Title:      "Competitor X launched similar feature",
		Metadata: map[string]any{
			"competitor": "CompetitorX",
			"impact":     "medium",
		},
	}

	assert.Equal(t, EventTypeCompetitor, event.EventType)
}

func TestEventTypeOrgChangeUseCase(t *testing.T) {
	event := &ProductEvent{
		ID:         1,
		EventType:  EventTypeOrgChange,
		Visibility: EventVisibilityInternal,
		Title:      "New DRI assigned",
		Metadata: map[string]any{
			"previous_dri": "alice@example.com",
			"new_dri":      "bob@example.com",
		},
	}

	assert.Equal(t, EventTypeOrgChange, event.EventType)
}
