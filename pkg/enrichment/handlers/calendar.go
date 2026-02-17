package handlers

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/otherjamesbrown/penf-cli/pkg/enrichment"
	"github.com/otherjamesbrown/penf-cli/pkg/enrichment/entities"
	"github.com/otherjamesbrown/penf-cli/pkg/enrichment/processors"
)

// MeetingEventType represents calendar event types.
type MeetingEventType string

const (
	MeetingEventInvite   MeetingEventType = "invite_sent"
	MeetingEventCancelled MeetingEventType = "cancelled"
	MeetingEventUpdated  MeetingEventType = "updated"
	MeetingEventResponse MeetingEventType = "response_received"
)

// MeetingStatus represents meeting status.
type MeetingStatus string

const (
	MeetingStatusActive    MeetingStatus = "active"
	MeetingStatusCancelled MeetingStatus = "cancelled"
	MeetingStatusUpdated   MeetingStatus = "updated"
)

// AttendeeResponse represents attendee response status.
type AttendeeResponse string

const (
	ResponseAccepted  AttendeeResponse = "accepted"
	ResponseDeclined  AttendeeResponse = "declined"
	ResponseTentative AttendeeResponse = "tentative"
	ResponseNone      AttendeeResponse = "none"
)

// MeetingAttendee represents a meeting attendee.
type MeetingAttendee struct {
	Email          string           `json:"email"`
	Name           string           `json:"name,omitempty"`
	ResponseStatus AttendeeResponse `json:"response_status"`
	IsOptional     bool             `json:"is_optional"`
}

// MeetingData represents extracted calendar meeting information.
type MeetingData struct {
	ICalUID        string            `json:"ical_uid"`
	Title          string            `json:"title"`
	Description    string            `json:"description,omitempty"`
	Location       string            `json:"location,omitempty"`
	VideoURL       string            `json:"video_url,omitempty"`
	StartTime      time.Time         `json:"start_time"`
	EndTime        time.Time         `json:"end_time"`
	RecurrenceRule string            `json:"recurrence_rule,omitempty"`
	Status         MeetingStatus     `json:"status"`
	EventType      MeetingEventType  `json:"event_type"`
	OrganizerEmail string            `json:"organizer_email,omitempty"`
	OrganizerName  string            `json:"organizer_name,omitempty"`
	Attendees      []MeetingAttendee `json:"attendees,omitempty"`
}

// CalendarExtractor extracts meeting information from calendar emails.
// It runs in Stage 3 (Type-Specific) for calendar/* content.
type CalendarExtractor struct{}

// NewCalendarExtractor creates a new CalendarExtractor processor.
func NewCalendarExtractor() *CalendarExtractor {
	return &CalendarExtractor{}
}

// Name returns the processor name.
func (c *CalendarExtractor) Name() string {
	return "CalendarExtractor"
}

// Stage returns the pipeline stage.
func (c *CalendarExtractor) Stage() processors.Stage {
	return processors.StageTypeSpecific
}

// Subtypes returns the content subtypes this processor handles.
func (c *CalendarExtractor) Subtypes() []enrichment.ContentSubtype {
	return []enrichment.ContentSubtype{
		enrichment.SubtypeCalendarInvite,
		enrichment.SubtypeCalendarCancellation,
		enrichment.SubtypeCalendarUpdate,
		enrichment.SubtypeCalendarResponse,
	}
}

// Process implements the Processor interface.
func (c *CalendarExtractor) Process(ctx context.Context, pctx *processors.ProcessorContext) error {
	return c.Extract(ctx, pctx)
}

// Extract extracts meeting information from the calendar email.
func (c *CalendarExtractor) Extract(ctx context.Context, pctx *processors.ProcessorContext) error {
	data, err := c.extractMeetingData(pctx.Source, pctx.Enrichment.Classification.Subtype)
	if err != nil {
		return err
	}

	if data != nil {
		if pctx.Enrichment.ExtractedData == nil {
			pctx.Enrichment.ExtractedData = make(map[string]interface{})
		}
		pctx.Enrichment.ExtractedData["meeting"] = data
	}

	return nil
}

// extractMeetingData extracts meeting data from the source.
func (c *CalendarExtractor) extractMeetingData(source *processors.Source, subtype enrichment.ContentSubtype) (*MeetingData, error) {
	data := &MeetingData{
		Status: MeetingStatusActive,
	}

	// Determine event type from subtype
	switch subtype {
	case enrichment.SubtypeCalendarInvite:
		data.EventType = MeetingEventInvite
	case enrichment.SubtypeCalendarCancellation:
		data.EventType = MeetingEventCancelled
		data.Status = MeetingStatusCancelled
	case enrichment.SubtypeCalendarUpdate:
		data.EventType = MeetingEventUpdated
		data.Status = MeetingStatusUpdated
	case enrichment.SubtypeCalendarResponse:
		data.EventType = MeetingEventResponse
	}

	// Extract from metadata
	subject := c.getMetadataString(source, "subject")
	data.Title = c.cleanTitle(subject)

	// Extract organizer from "from" or "organizer" field
	from := c.getMetadataString(source, "from", "organizer")
	data.OrganizerEmail = c.extractEmail(from)
	data.OrganizerName = c.extractName(from)

	// Try to extract iCal UID from metadata or content
	data.ICalUID = c.extractICalUID(source)

	// Extract location
	data.Location = c.getMetadataString(source, "location")

	// Extract times from metadata or body
	if start, ok := source.Metadata["start_time"].(time.Time); ok {
		data.StartTime = start
	}
	if end, ok := source.Metadata["end_time"].(time.Time); ok {
		data.EndTime = end
	}

	// If times not in metadata, try to parse from body
	if data.StartTime.IsZero() {
		bodyText := c.getMetadataString(source, "body_text", "body")
		c.extractTimesFromBody(bodyText, data)
	}

	// Extract video URL from body
	bodyText := c.getMetadataString(source, "body_text", "body")
	data.VideoURL = c.extractVideoURL(bodyText)

	// Extract description from body (first paragraph)
	data.Description = c.extractDescription(bodyText)

	// Extract attendees
	data.Attendees = c.extractAttendees(source)

	// If no UID found, generate one from subject + organizer + time
	if data.ICalUID == "" && !data.StartTime.IsZero() {
		data.ICalUID = c.generateFallbackUID(data)
	}

	return data, nil
}

// cleanTitle removes prefixes from meeting subject.
func (c *CalendarExtractor) cleanTitle(subject string) string {
	prefixes := []string{
		"Canceled:", "Cancelled:",
		"Updated:", "Accepted:", "Declined:", "Tentative:",
		"Re:", "Fwd:", "FW:",
	}

	title := subject
	for _, prefix := range prefixes {
		if strings.HasPrefix(strings.ToLower(title), strings.ToLower(prefix)) {
			title = strings.TrimSpace(title[len(prefix):])
		}
	}

	return strings.TrimSpace(title)
}

// uidPattern matches iCal UIDs
var uidPattern = regexp.MustCompile(`(?i)UID[:\s]+([^\s\r\n]+)`)

// extractICalUID extracts the iCal UID from source.
func (c *CalendarExtractor) extractICalUID(source *processors.Source) string {
	// Check metadata first
	if uid, ok := source.Metadata["ical_uid"].(string); ok && uid != "" {
		return uid
	}
	if uid, ok := source.Metadata["uid"].(string); ok && uid != "" {
		return uid
	}

	// Try to find in iCal content
	icalContent := c.getICalContent(source)
	if matches := uidPattern.FindStringSubmatch(icalContent); len(matches) >= 2 {
		return strings.TrimSpace(matches[1])
	}

	return ""
}

// getICalContent gets iCal content from attachments or raw content.
func (c *CalendarExtractor) getICalContent(source *processors.Source) string {
	// Check attachments
	if attachments, ok := source.Metadata["attachments"].([]interface{}); ok {
		for _, att := range attachments {
			if attMap, ok := att.(map[string]interface{}); ok {
				mimeType, _ := attMap["mime_type"].(string)
				if strings.Contains(mimeType, "calendar") {
					if content, ok := attMap["content"].(string); ok {
						return content
					}
				}
			}
		}
	}

	// Check raw content if it's calendar type
	if strings.Contains(source.ContentType, "calendar") {
		return source.RawContent
	}

	return ""
}

// extractTimesFromBody extracts meeting times from email body.
func (c *CalendarExtractor) extractTimesFromBody(body string, data *MeetingData) {
	// Common patterns for meeting times
	// "When: Tuesday, January 21, 2026 10:00 AM - 11:00 AM"
	whenPattern := regexp.MustCompile(`(?i)When:\s*([^\n]+)`)
	if matches := whenPattern.FindStringSubmatch(body); len(matches) >= 2 {
		// Parse the when string - simplified implementation
		// In production, this would use a proper date parser
		_ = matches[1] // Would parse this
	}

	// "Date: January 21, 2026"
	// "Time: 10:00 AM - 11:00 AM"
	datePattern := regexp.MustCompile(`(?i)Date:\s*([^\n]+)`)
	timePattern := regexp.MustCompile(`(?i)Time:\s*([^\n]+)`)

	_ = datePattern.FindStringSubmatch(body)
	_ = timePattern.FindStringSubmatch(body)

	// For now, leave times as zero if not in metadata
	// A full implementation would parse these patterns
}

// videoURLPatterns matches common video conferencing URLs
var videoURLPatterns = []*regexp.Regexp{
	regexp.MustCompile(`https?://[^\s]*\.webex\.com/[^\s<>"]+`),
	regexp.MustCompile(`https?://[^\s]*zoom\.us/[^\s<>"]+`),
	regexp.MustCompile(`https?://meet\.google\.com/[^\s<>"]+`),
	regexp.MustCompile(`https?://teams\.microsoft\.com/[^\s<>"]+`),
}

// extractVideoURL extracts video conferencing URL from body.
func (c *CalendarExtractor) extractVideoURL(body string) string {
	for _, pattern := range videoURLPatterns {
		if matches := pattern.FindString(body); matches != "" {
			return strings.TrimRight(matches, ".,;:!?)>")
		}
	}
	return ""
}

// extractDescription extracts meeting description (first paragraph of body).
func (c *CalendarExtractor) extractDescription(body string) string {
	if body == "" {
		return ""
	}

	// Skip common headers and get first substantive paragraph
	lines := strings.Split(body, "\n")
	var description strings.Builder
	started := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if started {
				break // End of first paragraph
			}
			continue
		}

		// Skip header-like lines
		lowerLine := strings.ToLower(line)
		if strings.HasPrefix(lowerLine, "when:") ||
			strings.HasPrefix(lowerLine, "where:") ||
			strings.HasPrefix(lowerLine, "location:") ||
			strings.HasPrefix(lowerLine, "organizer:") ||
			strings.HasPrefix(lowerLine, "invitees:") ||
			strings.HasPrefix(lowerLine, "attendees:") {
			continue
		}

		started = true
		if description.Len() > 0 {
			description.WriteString(" ")
		}
		description.WriteString(line)

		// Limit description length
		if description.Len() > 500 {
			break
		}
	}

	return strings.TrimSpace(description.String())
}

// extractAttendees extracts meeting attendees from source.
func (c *CalendarExtractor) extractAttendees(source *processors.Source) []MeetingAttendee {
	var attendees []MeetingAttendee

	// Check for attendees in metadata
	if atts, ok := source.Metadata["attendees"].([]interface{}); ok {
		for _, att := range atts {
			if attMap, ok := att.(map[string]interface{}); ok {
				attendee := MeetingAttendee{
					ResponseStatus: ResponseNone,
				}
				if email, ok := attMap["email"].(string); ok {
					attendee.Email = email
				}
				if name, ok := attMap["name"].(string); ok {
					attendee.Name = name
				}
				if status, ok := attMap["status"].(string); ok {
					attendee.ResponseStatus = c.parseResponseStatus(status)
				}
				if optional, ok := attMap["optional"].(bool); ok {
					attendee.IsOptional = optional
				}
				if attendee.Email != "" {
					attendees = append(attendees, attendee)
				}
			}
		}
	}

	// Also try to extract from To/Cc fields
	if len(attendees) == 0 {
		to := c.getMetadataString(source, "to")
		cc := c.getMetadataString(source, "cc")

		for _, addr := range strings.Split(to+","+cc, ",") {
			addr = strings.TrimSpace(addr)
			if addr == "" {
				continue
			}
			email := c.extractEmail(addr)
			if email != "" {
				attendees = append(attendees, MeetingAttendee{
					Email:          email,
					Name:           c.extractName(addr),
					ResponseStatus: ResponseNone,
				})
			}
		}
	}

	return attendees
}

// parseResponseStatus converts status string to AttendeeResponse.
func (c *CalendarExtractor) parseResponseStatus(status string) AttendeeResponse {
	switch strings.ToLower(status) {
	case "accepted", "accept":
		return ResponseAccepted
	case "declined", "decline":
		return ResponseDeclined
	case "tentative":
		return ResponseTentative
	default:
		return ResponseNone
	}
}

// generateFallbackUID generates a fallback UID when none is found.
func (c *CalendarExtractor) generateFallbackUID(data *MeetingData) string {
	// Simple fallback: organizer + start time + title hash
	return strings.ReplaceAll(
		strings.ToLower(data.OrganizerEmail)+"_"+data.StartTime.Format("20060102T150405"),
		" ", "_",
	)
}

// getMetadataString extracts a string from source metadata.
func (c *CalendarExtractor) getMetadataString(source *processors.Source, keys ...string) string {
	if source.Metadata == nil {
		return ""
	}

	for _, key := range keys {
		if val, ok := source.Metadata[key].(string); ok && val != "" {
			return val
		}
	}

	// Also check nested headers
	if headers, ok := source.Metadata["headers"].(map[string]interface{}); ok {
		for _, key := range keys {
			if val, ok := headers[key].(string); ok && val != "" {
				return val
			}
		}
	}

	return ""
}

// extractEmail extracts an email address from a "Name <email>" string.
func (c *CalendarExtractor) extractEmail(from string) string {
	if matches := emailPattern.FindStringSubmatch(from); len(matches) >= 2 {
		return strings.ToLower(matches[1])
	}
	return ""
}

// extractName extracts the display name from a "Name <email>" string.
func (c *CalendarExtractor) extractName(from string) string {
	name := regexp.MustCompile(`\s*<[^>]+>\s*`).ReplaceAllString(from, "")
	name = strings.Trim(name, `"' `)
	return entities.NormalizeDisplayName(name)
}

// Verify interface compliance
var _ processors.TypeSpecificProcessor = (*CalendarExtractor)(nil)
var _ processors.Processor = (*CalendarExtractor)(nil)
