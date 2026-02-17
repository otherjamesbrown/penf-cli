package attachments

import (
	"context"

	"github.com/otherjamesbrown/penf-cli/pkg/logging"
)

// ClassifierStep is the interface for a single classification step in the pipeline.
// Each step examines an attachment and returns a classification or nil to defer to the next step.
type ClassifierStep interface {
	// Name returns the identifier for this classifier step.
	Name() string

	// Classify examines an attachment and returns a classification.
	// Return nil to defer to the next step in the pipeline.
	// Return a Classification with TierPendingReview to indicate uncertainty.
	Classify(ctx context.Context, att *Attachment) (*Classification, error)
}

// Classifier runs attachments through a pipeline of classification steps.
type Classifier struct {
	steps  []ClassifierStep
	logger logging.Logger
}

// NewClassifier creates a new classifier with the given steps.
// Steps are run in order; first definitive classification wins.
func NewClassifier(logger logging.Logger, steps ...ClassifierStep) *Classifier {
	return &Classifier{
		steps:  steps,
		logger: logger.With(logging.F("component", "attachment_classifier")),
	}
}

// AddStep appends a classification step to the pipeline.
func (c *Classifier) AddStep(step ClassifierStep) {
	c.steps = append(c.steps, step)
}

// Classify runs an attachment through all classification steps.
// Returns the first definitive classification, or pending_review if all steps are uncertain.
func (c *Classifier) Classify(ctx context.Context, att *Attachment) (*Classification, []ProcessingStep, error) {
	var steps []ProcessingStep

	for _, step := range c.steps {
		classification, err := step.Classify(ctx, att)
		if err != nil {
			c.logger.Warn("Classification step failed, continuing to next",
				logging.Err(err),
				logging.F("step", step.Name()),
				logging.F("filename", att.Filename))
			continue
		}

		if classification == nil {
			// Step deferred, continue to next
			c.logger.Debug("Step deferred classification",
				logging.F("step", step.Name()),
				logging.F("filename", att.Filename))
			continue
		}

		// Record the step
		procStep := NewProcessingStep(
			step.Name(),
			classification.Tier,
			classification.Reason,
			classification.Confidence,
		)
		steps = append(steps, procStep)

		// If this is a definitive classification, return it
		if classification.IsDefinitive() {
			c.logger.Debug("Definitive classification",
				logging.F("step", step.Name()),
				logging.F("filename", att.Filename),
				logging.F("tier", string(classification.Tier)),
				logging.F("reason", classification.Reason),
				logging.F("confidence", classification.Confidence))

			return classification, steps, nil
		}

		// Record pending_review but continue to see if another step is more decisive
		c.logger.Debug("Step returned pending_review, continuing",
			logging.F("step", step.Name()),
			logging.F("filename", att.Filename),
			logging.F("reason", classification.Reason))
	}

	// No definitive classification; return pending_review
	if len(steps) == 0 {
		// No steps produced any result
		steps = append(steps, NewProcessingStep(
			"default",
			TierPendingReview,
			"no classifier steps matched",
			0.5,
		))
	}

	// Return the last step's classification (should be pending_review)
	lastStep := steps[len(steps)-1]
	return &Classification{
		Tier:       lastStep.Result,
		Reason:     lastStep.Reason,
		Confidence: lastStep.Confidence,
		Step:       lastStep.Step,
	}, steps, nil
}

// ClassifyAll classifies multiple attachments and returns results for each.
func (c *Classifier) ClassifyAll(ctx context.Context, attachments []*Attachment) ([]*ClassificationResult, error) {
	results := make([]*ClassificationResult, len(attachments))

	for i, att := range attachments {
		classification, steps, err := c.Classify(ctx, att)
		results[i] = &ClassificationResult{
			Attachment:     att,
			Classification: classification,
			Steps:          steps,
			Error:          err,
		}
	}

	return results, nil
}

// ClassificationResult pairs an attachment with its classification result.
type ClassificationResult struct {
	Attachment     *Attachment
	Classification *Classification
	Steps          []ProcessingStep
	Error          error
}

// Summary returns a summary of classification results.
func SummaryOf(results []*ClassificationResult) (processed, skipped, pending, errors int) {
	for _, r := range results {
		if r.Error != nil {
			errors++
			continue
		}
		if r.Classification == nil {
			pending++
			continue
		}
		switch {
		case r.Classification.Tier.IsProcessable():
			processed++
		case r.Classification.Tier.IsSkipped():
			skipped++
		case r.Classification.Tier.IsPending():
			pending++
		}
	}
	return
}
