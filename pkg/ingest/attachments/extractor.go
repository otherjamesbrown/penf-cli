package attachments

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/otherjamesbrown/penf-cli/pkg/ingest/eml"
	"github.com/otherjamesbrown/penf-cli/pkg/ingest/storage"
	"github.com/otherjamesbrown/penf-cli/pkg/logging"
)

// MaxEmbeddedEmailDepth is the maximum nesting depth for embedded emails.
const MaxEmbeddedEmailDepth = 5

// EmbeddedEmailHandler handles recursive processing of embedded email attachments.
type EmbeddedEmailHandler interface {
	// HandleEmbeddedEmail processes an embedded email and returns its source ID.
	// The handler should create the email source and recursively process its attachments.
	HandleEmbeddedEmail(ctx context.Context, params EmbeddedEmailParams) (*EmbeddedEmailResult, error)
}

// EmbeddedEmailParams contains parameters for processing an embedded email.
type EmbeddedEmailParams struct {
	TenantID        string
	ParentSourceID  int64
	Email           *eml.ParsedEmail
	Filename        string
	Position        int
	Depth           int      // Current nesting depth
	SeenMessageIDs  []string // Message IDs already processed (cycle detection)
}

// EmbeddedEmailResult contains the result of processing an embedded email.
type EmbeddedEmailResult struct {
	SourceID        int64
	MessageID       string
	AttachmentCount int
	WasSkipped      bool   // True if skipped due to duplicate/cycle
	SkipReason      string
}

// Extractor handles attachment extraction and storage for emails.
type Extractor struct {
	classifier           *Classifier
	repo                 *storage.Repository
	embeddedEmailHandler EmbeddedEmailHandler
	logger               logging.Logger
}

// NewExtractor creates a new attachment extractor.
func NewExtractor(repo *storage.Repository, logger logging.Logger) (*Extractor, error) {
	// Create classifier with default heuristics
	heuristic, err := NewHeuristicClassifier(DefaultHeuristicRules())
	if err != nil {
		return nil, err
	}

	classifier := NewClassifier(logger, heuristic)

	return &Extractor{
		classifier: classifier,
		repo:       repo,
		logger:     logger.With(logging.F("component", "attachment_extractor")),
	}, nil
}

// NewExtractorWithClassifier creates an extractor with a custom classifier.
func NewExtractorWithClassifier(repo *storage.Repository, classifier *Classifier, logger logging.Logger) *Extractor {
	return &Extractor{
		classifier: classifier,
		repo:       repo,
		logger:     logger.With(logging.F("component", "attachment_extractor")),
	}
}

// SetEmbeddedEmailHandler sets the handler for recursive embedded email processing.
func (e *Extractor) SetEmbeddedEmailHandler(handler EmbeddedEmailHandler) {
	e.embeddedEmailHandler = handler
}

// ExtractParams contains parameters for attachment extraction.
type ExtractParams struct {
	TenantID        string
	ParentSourceID  int64
	Email           *eml.ParsedEmail
	SourceTimestamp time.Time
	Depth           int      // Current nesting depth (0 for top-level)
	SeenMessageIDs  []string // Message IDs already processed (cycle detection)
}

// ExtractAndStore extracts attachments from an email and stores them.
// Returns extraction results including which attachments were processed/skipped.
func (e *Extractor) ExtractAndStore(ctx context.Context, params ExtractParams) (*ExtractionResult, error) {
	if len(params.Email.Attachments) == 0 {
		return &ExtractionResult{
			Attachments: []*AttachmentResult{},
			TotalCount:  0,
		}, nil
	}

	result := &ExtractionResult{
		Attachments: make([]*AttachmentResult, 0, len(params.Email.Attachments)),
		TotalCount:  len(params.Email.Attachments),
	}

	for i, emlAtt := range params.Email.Attachments {
		attResult, err := e.processAttachment(ctx, params, emlAtt, i)
		if err != nil {
			e.logger.Warn("Failed to process attachment",
				logging.Err(err),
				logging.F("filename", emlAtt.Filename),
				logging.F("position", i))

			result.Errors++
			attResult = &AttachmentResult{
				Attachment: e.emlToAttachment(emlAtt, i),
				Error:      err,
			}
		}

		result.Attachments = append(result.Attachments, attResult)

		// Update counters
		if attResult.Classification != nil {
			switch {
			case attResult.Classification.Tier.IsProcessable():
				result.Processed++
			case attResult.Classification.Tier.IsSkipped():
				result.Skipped++
			case attResult.Classification.Tier.IsPending():
				result.Pending++
			}
		}
	}

	e.logger.Debug("Attachment extraction complete",
		logging.F("parent_source_id", params.ParentSourceID),
		logging.F("total", result.TotalCount),
		logging.F("processed", result.Processed),
		logging.F("skipped", result.Skipped),
		logging.F("pending", result.Pending),
		logging.F("errors", result.Errors))

	return result, nil
}

// processAttachment processes a single attachment.
func (e *Extractor) processAttachment(ctx context.Context, params ExtractParams, emlAtt eml.Attachment, position int) (*AttachmentResult, error) {
	// Convert to our attachment type
	att := e.emlToAttachment(emlAtt, position)

	// Compute content hash if we have content
	if len(emlAtt.ContentData) > 0 {
		hash := sha256.Sum256(emlAtt.ContentData)
		att.ContentHash = hex.EncodeToString(hash[:])
		att.Content = emlAtt.ContentData
	}

	// Classify the attachment
	classification, steps, err := e.classifier.Classify(ctx, att)
	if err != nil {
		return nil, err
	}

	// Handle embedded emails specially
	if att.IsEmbeddedEmail && len(att.Content) > 0 {
		return e.processEmbeddedEmail(ctx, params, att, classification, steps, position)
	}

	// Create the link record for regular attachments
	link := &storage.AttachmentLink{
		ParentSourceID:  params.ParentSourceID,
		Filename:        att.Filename,
		MimeType:        att.MimeType,
		SizeBytes:       att.SizeBytes,
		ContentHash:     att.ContentHash,
		Position:        att.Position,
		ContentID:       att.ContentID,
		IsInline:        att.IsInline,
		ProcessingTier:  classification.Tier,
		TierReason:      classification.Reason,
		ProcessingSteps: steps,
		IsEmbeddedEmail: false,
	}

	var created *storage.CreatedAttachment

	if classification.Tier.IsProcessable() && len(att.Content) > 0 {
		// Store as source + link
		attSource := &storage.AttachmentSource{
			TenantID:        params.TenantID,
			ParentSourceID:  params.ParentSourceID,
			Filename:        att.Filename,
			MimeType:        att.MimeType,
			SizeBytes:       att.SizeBytes,
			ContentHash:     att.ContentHash,
			Content:         att.Content,
			SourceTimestamp: params.SourceTimestamp,
		}

		created, err = e.repo.CreateAttachmentWithSource(ctx, attSource, link)
		if err != nil {
			return nil, err
		}
	} else {
		// Create link only (for skipped or pending without content)
		created, err = e.repo.CreateAttachmentLinkOnly(ctx, link)
		if err != nil {
			return nil, err
		}
	}

	return &AttachmentResult{
		Attachment:     att,
		Classification: classification,
		SourceID:       created.SourceID,
		LinkID:         created.LinkID,
	}, nil
}

// processEmbeddedEmail handles an embedded .eml/.msg attachment.
func (e *Extractor) processEmbeddedEmail(ctx context.Context, params ExtractParams, att *Attachment, classification *Classification, steps []ProcessingStep, position int) (*AttachmentResult, error) {
	// Check depth limit
	if params.Depth >= MaxEmbeddedEmailDepth {
		e.logger.Warn("Max embedded email depth reached, storing as attachment",
			logging.F("depth", params.Depth),
			logging.F("filename", att.Filename))

		// Store as regular attachment instead
		return e.storeAsRegularAttachment(ctx, params, att, classification, steps)
	}

	// Parse the embedded email
	parser := eml.NewParser(eml.ParseOptions{
		IncludeAttachmentContent: true,
	})
	parseResult, err := parser.ParseBytes(att.Content)
	if err != nil {
		e.logger.Warn("Failed to parse embedded email, storing as attachment",
			logging.Err(err),
			logging.F("filename", att.Filename))

		// Store as regular attachment on parse failure
		return e.storeAsRegularAttachment(ctx, params, att, classification, steps)
	}

	embeddedEmail := parseResult.Email

	// Check for cycles (same message-id already processed)
	for _, seenID := range params.SeenMessageIDs {
		if seenID == embeddedEmail.MessageID {
			e.logger.Warn("Cycle detected in embedded emails, skipping",
				logging.F("message_id", embeddedEmail.MessageID),
				logging.F("filename", att.Filename))

			// Create link only with skip reason
			link := &storage.AttachmentLink{
				ParentSourceID:  params.ParentSourceID,
				Filename:        att.Filename,
				MimeType:        att.MimeType,
				SizeBytes:       att.SizeBytes,
				ContentHash:     att.ContentHash,
				Position:        position,
				ProcessingTier:  TierAutoSkip,
				TierReason:      fmt.Sprintf("cycle detected: message-id %s already processed", embeddedEmail.MessageID),
				ProcessingSteps: steps,
				IsEmbeddedEmail: true,
			}

			created, err := e.repo.CreateAttachmentLinkOnly(ctx, link)
			if err != nil {
				return nil, err
			}

			return &AttachmentResult{
				Attachment: att,
				Classification: &Classification{
					Tier:       TierAutoSkip,
					Reason:     "cycle detected",
					Confidence: 1.0,
					Step:       "cycle_detection",
				},
				LinkID: created.LinkID,
			}, nil
		}
	}

	// No handler - store as regular attachment
	if e.embeddedEmailHandler == nil {
		e.logger.Debug("No embedded email handler, storing as regular attachment",
			logging.F("filename", att.Filename))
		return e.storeAsRegularAttachment(ctx, params, att, classification, steps)
	}

	// Call the handler to process the embedded email
	result, err := e.embeddedEmailHandler.HandleEmbeddedEmail(ctx, EmbeddedEmailParams{
		TenantID:       params.TenantID,
		ParentSourceID: params.ParentSourceID,
		Email:          embeddedEmail,
		Filename:       att.Filename,
		Position:       position,
		Depth:          params.Depth + 1,
		SeenMessageIDs: append(params.SeenMessageIDs, params.Email.MessageID),
	})
	if err != nil {
		e.logger.Warn("Failed to process embedded email, storing as attachment",
			logging.Err(err),
			logging.F("filename", att.Filename))
		return e.storeAsRegularAttachment(ctx, params, att, classification, steps)
	}

	// Create the link to the embedded email source
	link := &storage.AttachmentLink{
		ParentSourceID:  params.ParentSourceID,
		ChildSourceID:   &result.SourceID,
		Filename:        att.Filename,
		MimeType:        "message/rfc822",
		SizeBytes:       att.SizeBytes,
		ContentHash:     att.ContentHash,
		Position:        position,
		ProcessingTier:  TierAutoProcess,
		TierReason:      "embedded email processed recursively",
		ProcessingSteps: steps,
		IsEmbeddedEmail: true,
	}

	created, err := e.repo.CreateAttachmentLinkOnly(ctx, link)
	if err != nil {
		return nil, err
	}

	e.logger.Debug("Embedded email processed recursively",
		logging.F("parent_source_id", params.ParentSourceID),
		logging.F("embedded_source_id", result.SourceID),
		logging.F("message_id", result.MessageID),
		logging.F("depth", params.Depth+1))

	return &AttachmentResult{
		Attachment: att,
		Classification: &Classification{
			Tier:       TierAutoProcess,
			Reason:     "embedded email",
			Confidence: 1.0,
			Step:       "recursive_email",
		},
		SourceID: &result.SourceID,
		LinkID:   created.LinkID,
	}, nil
}

// storeAsRegularAttachment stores an embedded email as a regular attachment.
func (e *Extractor) storeAsRegularAttachment(ctx context.Context, params ExtractParams, att *Attachment, classification *Classification, steps []ProcessingStep) (*AttachmentResult, error) {
	link := &storage.AttachmentLink{
		ParentSourceID:  params.ParentSourceID,
		Filename:        att.Filename,
		MimeType:        att.MimeType,
		SizeBytes:       att.SizeBytes,
		ContentHash:     att.ContentHash,
		Position:        att.Position,
		ContentID:       att.ContentID,
		IsInline:        att.IsInline,
		ProcessingTier:  classification.Tier,
		TierReason:      classification.Reason,
		ProcessingSteps: steps,
		IsEmbeddedEmail: true, // Still mark as embedded email for reference
	}

	attSource := &storage.AttachmentSource{
		TenantID:        params.TenantID,
		ParentSourceID:  params.ParentSourceID,
		Filename:        att.Filename,
		MimeType:        att.MimeType,
		SizeBytes:       att.SizeBytes,
		ContentHash:     att.ContentHash,
		Content:         att.Content,
		SourceTimestamp: params.SourceTimestamp,
	}

	created, err := e.repo.CreateAttachmentWithSource(ctx, attSource, link)
	if err != nil {
		return nil, err
	}

	return &AttachmentResult{
		Attachment:     att,
		Classification: classification,
		SourceID:       created.SourceID,
		LinkID:         created.LinkID,
	}, nil
}

// emlToAttachment converts an eml.Attachment to our Attachment type.
func (e *Extractor) emlToAttachment(emlAtt eml.Attachment, position int) *Attachment {
	return &Attachment{
		Filename:    emlAtt.Filename,
		MimeType:    emlAtt.MimeType,
		SizeBytes:   int64(emlAtt.Size),
		ContentID:   emlAtt.ContentID,
		IsInline:    emlAtt.IsInline,
		Position:    position,
		Content:     emlAtt.ContentData,
	}
}

// GetContentHashForDedup computes a content hash for deduplication.
func GetContentHashForDedup(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}
