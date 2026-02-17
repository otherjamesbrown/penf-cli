package attachments

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// HeuristicRules configures the heuristic classifier behavior.
type HeuristicRules struct {
	// AutoProcess rules - high-value attachments
	AutoProcess AutoProcessRules `json:"auto_process"`

	// AutoSkip rules - low-value attachments
	AutoSkip AutoSkipRules `json:"auto_skip"`

	// RecursiveEmail rules - .eml/.msg files to process as emails
	RecursiveEmail RecursiveEmailRules `json:"recursive_email"`
}

// AutoProcessRules defines when to auto-process an attachment.
type AutoProcessRules struct {
	// MimeTypes that should always be processed (e.g., "application/pdf")
	MimeTypes []string `json:"mime_types"`

	// FileExtensions that should always be processed (e.g., ".pdf", ".docx")
	FileExtensions []string `json:"file_extensions"`

	// MinImageSize - images larger than this are likely documents/diagrams (bytes)
	MinImageSize int64 `json:"min_image_size"`
}

// AutoSkipRules defines when to auto-skip an attachment.
type AutoSkipRules struct {
	// MaxSize - attachments smaller than this are likely signatures/logos (bytes)
	MaxSize int64 `json:"max_size"`

	// SkipInlineWithContentID - skip inline images that have Content-ID
	SkipInlineWithContentID bool `json:"skip_inline_with_content_id"`

	// FilenamePatterns - regex patterns that indicate junk (case-insensitive)
	FilenamePatterns []string `json:"filename_patterns"`

	// MimeTypes to always skip (e.g., tracking pixels)
	MimeTypes []string `json:"mime_types"`
}

// RecursiveEmailRules defines when to treat attachment as embedded email.
type RecursiveEmailRules struct {
	// MimeTypes for email messages (e.g., "message/rfc822")
	MimeTypes []string `json:"mime_types"`

	// FileExtensions for email files (e.g., ".eml", ".msg")
	FileExtensions []string `json:"file_extensions"`
}

// DefaultHeuristicRules returns sensible default rules.
func DefaultHeuristicRules() HeuristicRules {
	return HeuristicRules{
		AutoProcess: AutoProcessRules{
			MimeTypes: []string{
				"application/pdf",
				"application/msword",
				"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
				"application/vnd.ms-excel",
				"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
				"application/vnd.ms-powerpoint",
				"application/vnd.openxmlformats-officedocument.presentationml.presentation",
				"text/plain",
				"text/csv",
				"application/rtf",
				"application/vnd.oasis.opendocument.text",
				"application/vnd.oasis.opendocument.spreadsheet",
			},
			FileExtensions: []string{
				".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx",
				".txt", ".csv", ".rtf", ".odt", ".ods", ".odp",
			},
			MinImageSize: 100 * 1024, // 100KB - larger images are likely diagrams/screenshots
		},
		AutoSkip: AutoSkipRules{
			MaxSize:                 20 * 1024, // 20KB - tiny files are signatures/logos
			SkipInlineWithContentID: true,
			FilenamePatterns: []string{
				`(?i)signature`,
				`(?i)logo`,
				`(?i)icon`,
				`(?i)^image\d+\.(png|gif|jpe?g)$`, // image001.png, etc.
				`(?i)spacer`,
				`(?i)pixel`,
				`(?i)tracking`,
				`(?i)banner`,
				`(?i)footer`,
			},
			MimeTypes: []string{
				// Tracking pixels and tiny images
				"image/gif", // Only skip if size is also small
			},
		},
		RecursiveEmail: RecursiveEmailRules{
			MimeTypes: []string{
				"message/rfc822",
				"application/vnd.ms-outlook",
			},
			FileExtensions: []string{
				".eml", ".msg",
			},
		},
	}
}

// HeuristicClassifier classifies attachments based on configurable rules.
type HeuristicClassifier struct {
	rules            HeuristicRules
	skipPatterns     []*regexp.Regexp
}

// NewHeuristicClassifier creates a new heuristic classifier with the given rules.
func NewHeuristicClassifier(rules HeuristicRules) (*HeuristicClassifier, error) {
	// Compile filename patterns
	patterns := make([]*regexp.Regexp, 0, len(rules.AutoSkip.FilenamePatterns))
	for _, p := range rules.AutoSkip.FilenamePatterns {
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, err
		}
		patterns = append(patterns, re)
	}

	return &HeuristicClassifier{
		rules:        rules,
		skipPatterns: patterns,
	}, nil
}

// Name returns the classifier step name.
func (h *HeuristicClassifier) Name() string {
	return "heuristic"
}

// Classify examines an attachment and returns a classification based on heuristics.
func (h *HeuristicClassifier) Classify(ctx context.Context, att *Attachment) (*Classification, error) {
	// Check for recursive email first (special handling)
	if h.isEmbeddedEmail(att) {
		att.IsEmbeddedEmail = true
		return &Classification{
			Tier:       TierAutoProcess,
			Reason:     "embedded email attachment (.eml/.msg)",
			Confidence: 1.0,
			Step:       h.Name(),
		}, nil
	}

	// Check auto-process rules (high-value)
	if classification := h.checkAutoProcess(att); classification != nil {
		return classification, nil
	}

	// Check auto-skip rules (low-value)
	if classification := h.checkAutoSkip(att); classification != nil {
		return classification, nil
	}

	// Default to pending review
	return &Classification{
		Tier:       TierPendingReview,
		Reason:     "no heuristic rule matched",
		Confidence: 0.5,
		Step:       h.Name(),
	}, nil
}

// isEmbeddedEmail checks if attachment is an embedded email.
func (h *HeuristicClassifier) isEmbeddedEmail(att *Attachment) bool {
	// Check MIME type
	mimeType := strings.ToLower(att.MimeType)
	for _, mt := range h.rules.RecursiveEmail.MimeTypes {
		if strings.EqualFold(mimeType, mt) {
			return true
		}
	}

	// Check file extension
	ext := strings.ToLower(filepath.Ext(att.Filename))
	for _, e := range h.rules.RecursiveEmail.FileExtensions {
		if strings.EqualFold(ext, e) {
			return true
		}
	}

	return false
}

// checkAutoProcess checks if attachment should be auto-processed.
func (h *HeuristicClassifier) checkAutoProcess(att *Attachment) *Classification {
	mimeType := strings.ToLower(att.MimeType)
	ext := strings.ToLower(filepath.Ext(att.Filename))

	// Check MIME type
	for _, mt := range h.rules.AutoProcess.MimeTypes {
		if strings.EqualFold(mimeType, mt) {
			return &Classification{
				Tier:       TierAutoProcess,
				Reason:     "document MIME type: " + mimeType,
				Confidence: 0.95,
				Step:       h.Name(),
			}
		}
	}

	// Check file extension
	for _, e := range h.rules.AutoProcess.FileExtensions {
		if strings.EqualFold(ext, e) {
			return &Classification{
				Tier:       TierAutoProcess,
				Reason:     "document extension: " + ext,
				Confidence: 0.9,
				Step:       h.Name(),
			}
		}
	}

	// Check for large images (likely diagrams/screenshots)
	if h.isImage(att) && att.SizeBytes >= h.rules.AutoProcess.MinImageSize {
		return &Classification{
			Tier:       TierAutoProcess,
			Reason:     "large image (likely diagram/screenshot): " + formatSize(att.SizeBytes),
			Confidence: 0.8,
			Step:       h.Name(),
		}
	}

	return nil
}

// checkAutoSkip checks if attachment should be auto-skipped.
func (h *HeuristicClassifier) checkAutoSkip(att *Attachment) *Classification {
	// Check inline images with Content-ID (usually embedded in HTML)
	if h.rules.AutoSkip.SkipInlineWithContentID && att.IsInline && att.ContentID != "" {
		return &Classification{
			Tier:       TierAutoSkip,
			Reason:     "inline image with Content-ID (embedded in HTML body)",
			Confidence: 0.9,
			Step:       h.Name(),
		}
	}

	// Check tiny files (signatures, logos, pixels)
	if att.SizeBytes > 0 && att.SizeBytes <= h.rules.AutoSkip.MaxSize {
		// Only skip if it's an image
		if h.isImage(att) {
			return &Classification{
				Tier:       TierAutoSkip,
				Reason:     "tiny image (likely signature/logo): " + formatSize(att.SizeBytes),
				Confidence: 0.85,
				Step:       h.Name(),
			}
		}
	}

	// Check filename patterns
	for _, pattern := range h.skipPatterns {
		if pattern.MatchString(att.Filename) {
			return &Classification{
				Tier:       TierAutoSkip,
				Reason:     "filename matches skip pattern: " + pattern.String(),
				Confidence: 0.85,
				Step:       h.Name(),
			}
		}
	}

	// Check skip MIME types (with size consideration for GIFs)
	mimeType := strings.ToLower(att.MimeType)
	for _, mt := range h.rules.AutoSkip.MimeTypes {
		if strings.EqualFold(mimeType, mt) {
			// For GIFs, only skip if they're small (likely tracking pixels)
			if mimeType == "image/gif" && att.SizeBytes > h.rules.AutoSkip.MaxSize {
				continue // Don't skip large GIFs
			}
			return &Classification{
				Tier:       TierAutoSkip,
				Reason:     "skip MIME type: " + mimeType,
				Confidence: 0.8,
				Step:       h.Name(),
			}
		}
	}

	return nil
}

// isImage returns true if the attachment appears to be an image.
func (h *HeuristicClassifier) isImage(att *Attachment) bool {
	mimeType := strings.ToLower(att.MimeType)
	if strings.HasPrefix(mimeType, "image/") {
		return true
	}

	ext := strings.ToLower(filepath.Ext(att.Filename))
	imageExts := []string{".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", ".tiff", ".tif", ".svg"}
	for _, e := range imageExts {
		if ext == e {
			return true
		}
	}

	return false
}

// formatSize returns a human-readable size string.
func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
	)

	switch {
	case bytes >= MB:
		mb := float64(bytes) / float64(MB)
		if mb == float64(int64(mb)) {
			return fmt.Sprintf("%dMB", int64(mb))
		}
		return fmt.Sprintf("%.1fMB", mb)
	case bytes >= KB:
		kb := float64(bytes) / float64(KB)
		if kb == float64(int64(kb)) {
			return fmt.Sprintf("%dKB", int64(kb))
		}
		return fmt.Sprintf("%.1fKB", kb)
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}
