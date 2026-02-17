// Package handlers provides type-specific enrichment processors.
package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"regexp"
	"strings"

	"github.com/otherjamesbrown/penf-cli/pkg/enrichment"
	"github.com/otherjamesbrown/penf-cli/pkg/enrichment/processors"
)

// LinkCategory represents the category of an extracted link.
type LinkCategory string

const (
	LinkCategoryGoogleDoc     LinkCategory = "google_doc"
	LinkCategoryGoogleSheet   LinkCategory = "google_sheet"
	LinkCategoryGoogleSlides  LinkCategory = "google_slides"
	LinkCategoryGoogleDrive   LinkCategory = "google_drive"
	LinkCategoryJiraTicket    LinkCategory = "jira_ticket"
	LinkCategoryJiraBoard     LinkCategory = "jira_board"
	LinkCategoryConfluence    LinkCategory = "confluence"
	LinkCategoryWebexRecording LinkCategory = "webex_recording"
	LinkCategoryZoomRecording LinkCategory = "zoom_recording"
	LinkCategorySharepoint    LinkCategory = "sharepoint"
	LinkCategoryOnedrive      LinkCategory = "onedrive"
	LinkCategoryGitHub        LinkCategory = "github"
	LinkCategoryGitLab        LinkCategory = "gitlab"
	LinkCategoryBitbucket     LinkCategory = "bitbucket"
	LinkCategorySlack         LinkCategory = "slack"
	LinkCategoryTeams         LinkCategory = "teams"
	LinkCategoryGenericURL    LinkCategory = "generic_url"
)

// LinkExtractor extracts and categorizes links from content.
// It runs in Stage 2 (Common Enrichment) for all content types.
type LinkExtractor struct {
	// Configuration
	extractFromSignatures bool
	contextChars          int // Characters of context to extract around link
}

// NewLinkExtractor creates a new LinkExtractor processor.
func NewLinkExtractor(opts ...LinkExtractorOption) *LinkExtractor {
	l := &LinkExtractor{
		extractFromSignatures: true,
		contextChars:          100,
	}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// LinkExtractorOption configures the LinkExtractor.
type LinkExtractorOption func(*LinkExtractor)

// WithoutSignatureLinks configures the extractor to skip links in signatures.
func WithoutSignatureLinks() LinkExtractorOption {
	return func(l *LinkExtractor) {
		l.extractFromSignatures = false
	}
}

// WithContextChars sets how many characters of context to extract around links.
func WithContextChars(chars int) LinkExtractorOption {
	return func(l *LinkExtractor) {
		l.contextChars = chars
	}
}

// Name returns the processor name.
func (l *LinkExtractor) Name() string {
	return "LinkExtractor"
}

// Stage returns the pipeline stage.
func (l *LinkExtractor) Stage() processors.Stage {
	return processors.StageCommonEnrichment
}

// CanProcess returns true - links are extracted from all content types.
func (l *LinkExtractor) CanProcess(classification *enrichment.Classification) bool {
	return true
}

// Process extracts links from the source content.
func (l *LinkExtractor) Process(ctx context.Context, pctx *processors.ProcessorContext) error {
	// Get body content from source
	bodyText := l.getBodyText(pctx.Source)
	bodyHTML := l.getBodyHTML(pctx.Source)

	// Track unique links by URL hash
	seen := make(map[string]bool)
	var links []enrichment.ExtractedLink

	// Extract from plain text body
	textLinks := l.extractFromText(bodyText, "body_text")
	for _, link := range textLinks {
		hash := hashURL(link.URL)
		if !seen[hash] {
			seen[hash] = true
			links = append(links, link)
		}
	}

	// Extract from HTML body
	htmlLinks := l.extractFromHTML(bodyHTML, "body_html")
	for _, link := range htmlLinks {
		hash := hashURL(link.URL)
		if !seen[hash] {
			seen[hash] = true
			links = append(links, link)
		}
	}

	// Categorize and enrich links
	for i := range links {
		links[i].Category = string(categorizeLink(links[i].URL))
		links[i].ServiceID = extractServiceID(links[i].URL, LinkCategory(links[i].Category))
	}

	// Filter signature links if configured
	if !l.extractFromSignatures {
		var filtered []enrichment.ExtractedLink
		for _, link := range links {
			if !l.isInSignature(link, bodyText) {
				filtered = append(filtered, link)
			}
		}
		links = filtered
	}

	pctx.Enrichment.ExtractedLinks = links
	return nil
}

// getBodyText extracts plain text body from source metadata.
func (l *LinkExtractor) getBodyText(source *processors.Source) string {
	if source.Metadata == nil {
		return source.RawContent
	}

	// Try common metadata keys
	if body, ok := source.Metadata["body_text"].(string); ok {
		return body
	}
	if body, ok := source.Metadata["body"].(string); ok {
		return body
	}
	if body, ok := source.Metadata["text"].(string); ok {
		return body
	}

	return source.RawContent
}

// getBodyHTML extracts HTML body from source metadata.
func (l *LinkExtractor) getBodyHTML(source *processors.Source) string {
	if source.Metadata == nil {
		return ""
	}

	if html, ok := source.Metadata["body_html"].(string); ok {
		return html
	}
	if html, ok := source.Metadata["html"].(string); ok {
		return html
	}

	return ""
}

// urlPattern matches URLs in text.
var urlPattern = regexp.MustCompile(`https?://[^\s<>"{}|\\^` + "`" + `\[\]]+`)

// extractFromText extracts links from plain text.
func (l *LinkExtractor) extractFromText(text, sourceField string) []enrichment.ExtractedLink {
	if text == "" {
		return nil
	}

	var links []enrichment.ExtractedLink
	matches := urlPattern.FindAllStringIndex(text, -1)

	for _, match := range matches {
		rawURL := text[match[0]:match[1]]
		// Clean up common trailing punctuation
		rawURL = strings.TrimRight(rawURL, ".,;:!?)>")

		// Validate URL
		parsed, err := url.Parse(rawURL)
		if err != nil || parsed.Host == "" {
			continue
		}

		// Extract context
		context := l.extractContext(text, match[0], match[1])

		links = append(links, enrichment.ExtractedLink{
			URL:         rawURL,
			Context:     context,
			SourceField: sourceField,
			IsInline:    false,
		})
	}

	return links
}

// hrefPattern matches href attributes in HTML.
var hrefPattern = regexp.MustCompile(`<a[^>]*href=["']([^"']+)["'][^>]*>([^<]*)</a>`)

// extractFromHTML extracts links from HTML content.
func (l *LinkExtractor) extractFromHTML(html, sourceField string) []enrichment.ExtractedLink {
	if html == "" {
		return nil
	}

	var links []enrichment.ExtractedLink
	matches := hrefPattern.FindAllStringSubmatch(html, -1)

	for _, match := range matches {
		if len(match) < 3 {
			continue
		}

		rawURL := match[1]
		anchorText := strings.TrimSpace(match[2])

		// Skip mailto: and javascript: links
		if strings.HasPrefix(rawURL, "mailto:") || strings.HasPrefix(rawURL, "javascript:") {
			continue
		}

		// Validate URL
		parsed, err := url.Parse(rawURL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			continue
		}

		links = append(links, enrichment.ExtractedLink{
			URL:         rawURL,
			Text:        anchorText,
			SourceField: sourceField,
			IsInline:    true,
		})
	}

	// Also extract plain URLs from HTML that might not be in anchor tags
	textLinks := l.extractFromText(html, sourceField)
	links = append(links, textLinks...)

	return links
}

// extractContext extracts surrounding text around a match position.
func (l *LinkExtractor) extractContext(text string, start, end int) string {
	contextStart := start - l.contextChars/2
	if contextStart < 0 {
		contextStart = 0
	}

	contextEnd := end + l.contextChars/2
	if contextEnd > len(text) {
		contextEnd = len(text)
	}

	context := text[contextStart:contextEnd]
	// Clean up whitespace
	context = strings.Join(strings.Fields(context), " ")

	return context
}

// isInSignature detects if a link appears to be in an email signature.
func (l *LinkExtractor) isInSignature(link enrichment.ExtractedLink, bodyText string) bool {
	// Common signature indicators
	signatureMarkers := []string{
		"--",
		"Best regards",
		"Kind regards",
		"Thanks",
		"Sent from",
		"Get Outlook",
		"Confidentiality Notice",
		"DISCLAIMER",
	}

	// Check if link context contains signature markers
	contextLower := strings.ToLower(link.Context)
	for _, marker := range signatureMarkers {
		if strings.Contains(contextLower, strings.ToLower(marker)) {
			return true
		}
	}

	// Check if link is in the last 20% of the email
	if bodyText != "" {
		linkPos := strings.Index(bodyText, link.URL)
		if linkPos >= 0 && float64(linkPos) > float64(len(bodyText))*0.8 {
			return true
		}
	}

	return false
}

// hashURL creates a SHA-256 hash of a URL for deduplication.
func hashURL(u string) string {
	h := sha256.Sum256([]byte(u))
	return hex.EncodeToString(h[:])
}

// categorizeLink determines the category of a URL.
func categorizeLink(rawURL string) LinkCategory {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return LinkCategoryGenericURL
	}

	host := strings.ToLower(parsed.Host)
	path := strings.ToLower(parsed.Path)

	// Google Docs
	if strings.Contains(host, "docs.google.com") {
		if strings.Contains(path, "/document/") {
			return LinkCategoryGoogleDoc
		}
		if strings.Contains(path, "/spreadsheets/") {
			return LinkCategoryGoogleSheet
		}
		if strings.Contains(path, "/presentation/") {
			return LinkCategoryGoogleSlides
		}
	}

	// Google Drive
	if strings.Contains(host, "drive.google.com") {
		return LinkCategoryGoogleDrive
	}

	// Jira
	if strings.Contains(host, "atlassian.net") || strings.Contains(host, "jira") {
		if strings.Contains(path, "/browse/") || jiraTicketPattern.MatchString(path) {
			return LinkCategoryJiraTicket
		}
		if strings.Contains(path, "/board/") {
			return LinkCategoryJiraBoard
		}
	}

	// Confluence
	if strings.Contains(host, "atlassian.net") && strings.Contains(path, "/wiki/") {
		return LinkCategoryConfluence
	}

	// WebEx
	if strings.Contains(host, "webex.com") {
		if strings.Contains(path, "recording") {
			return LinkCategoryWebexRecording
		}
	}

	// Zoom
	if strings.Contains(host, "zoom.us") {
		if strings.Contains(path, "/rec/") {
			return LinkCategoryZoomRecording
		}
	}

	// SharePoint / OneDrive
	if strings.Contains(host, "sharepoint.com") {
		return LinkCategorySharepoint
	}
	if strings.Contains(host, "onedrive.") || strings.Contains(host, "1drv.") {
		return LinkCategoryOnedrive
	}

	// GitHub
	if strings.Contains(host, "github.com") {
		return LinkCategoryGitHub
	}

	// GitLab
	if strings.Contains(host, "gitlab.com") || strings.Contains(host, "gitlab.") {
		return LinkCategoryGitLab
	}

	// Bitbucket
	if strings.Contains(host, "bitbucket.org") {
		return LinkCategoryBitbucket
	}

	// Slack
	if strings.Contains(host, "slack.com") {
		return LinkCategorySlack
	}

	// Microsoft Teams
	if strings.Contains(host, "teams.microsoft.com") {
		return LinkCategoryTeams
	}

	return LinkCategoryGenericURL
}

// jiraTicketPattern matches Jira ticket paths like /browse/OUT-123
var jiraTicketPattern = regexp.MustCompile(`/browse/[A-Z]{2,10}-\d+`)

// googleDocIDPattern extracts Google doc IDs
var googleDocIDPattern = regexp.MustCompile(`/d/([a-zA-Z0-9_-]+)`)

// extractServiceID extracts the service-specific ID from a URL.
func extractServiceID(rawURL string, category LinkCategory) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	switch category {
	case LinkCategoryGoogleDoc, LinkCategoryGoogleSheet, LinkCategoryGoogleSlides, LinkCategoryGoogleDrive:
		matches := googleDocIDPattern.FindStringSubmatch(parsed.Path)
		if len(matches) >= 2 {
			return matches[1]
		}

	case LinkCategoryJiraTicket:
		// Extract ticket key from /browse/OUT-123
		if strings.Contains(parsed.Path, "/browse/") {
			parts := strings.Split(parsed.Path, "/browse/")
			if len(parts) >= 2 {
				// Get ticket key, may have trailing path
				ticketPart := strings.Split(parts[1], "/")[0]
				ticketPart = strings.Split(ticketPart, "?")[0]
				return strings.ToUpper(ticketPart)
			}
		}

	case LinkCategoryGitHub:
		// Extract owner/repo from github.com/owner/repo
		parts := strings.Split(strings.TrimPrefix(parsed.Path, "/"), "/")
		if len(parts) >= 2 {
			return parts[0] + "/" + parts[1]
		}
	}

	return ""
}

// Verify interface compliance
var _ processors.CommonEnrichmentProcessor = (*LinkExtractor)(nil)
var _ processors.Processor = (*LinkExtractor)(nil)
