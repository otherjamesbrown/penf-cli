package attachments

import (
	"context"
	"testing"
)

func TestHeuristicClassifier_AutoProcess(t *testing.T) {
	classifier, err := NewHeuristicClassifier(DefaultHeuristicRules())
	if err != nil {
		t.Fatalf("failed to create classifier: %v", err)
	}

	tests := []struct {
		name     string
		att      *Attachment
		wantTier ProcessingTier
	}{
		{
			name: "PDF by mime type",
			att: &Attachment{
				Filename:  "contract.pdf",
				MimeType:  "application/pdf",
				SizeBytes: 500000,
			},
			wantTier: TierAutoProcess,
		},
		{
			name: "Word doc by extension",
			att: &Attachment{
				Filename:  "proposal.docx",
				MimeType:  "application/octet-stream", // Sometimes wrong
				SizeBytes: 100000,
			},
			wantTier: TierAutoProcess,
		},
		{
			name: "Large image (diagram)",
			att: &Attachment{
				Filename:  "architecture.png",
				MimeType:  "image/png",
				SizeBytes: 150000, // 150KB
			},
			wantTier: TierAutoProcess,
		},
		{
			name: "Embedded email",
			att: &Attachment{
				Filename:  "forwarded.eml",
				MimeType:  "message/rfc822",
				SizeBytes: 50000,
			},
			wantTier: TierAutoProcess,
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			classification, err := classifier.Classify(ctx, tt.att)
			if err != nil {
				t.Fatalf("Classify() error = %v", err)
			}
			if classification.Tier != tt.wantTier {
				t.Errorf("Classify() tier = %v, want %v (reason: %s)",
					classification.Tier, tt.wantTier, classification.Reason)
			}
		})
	}
}

func TestHeuristicClassifier_AutoSkip(t *testing.T) {
	classifier, err := NewHeuristicClassifier(DefaultHeuristicRules())
	if err != nil {
		t.Fatalf("failed to create classifier: %v", err)
	}

	tests := []struct {
		name     string
		att      *Attachment
		wantTier ProcessingTier
	}{
		{
			name: "Tiny signature image",
			att: &Attachment{
				Filename:  "sig.png",
				MimeType:  "image/png",
				SizeBytes: 5000, // 5KB
			},
			wantTier: TierAutoSkip,
		},
		{
			name: "Inline image with Content-ID",
			att: &Attachment{
				Filename:  "logo.png",
				MimeType:  "image/png",
				SizeBytes: 50000, // 50KB - medium size but inline
				IsInline:  true,
				ContentID: "logo@company.com",
			},
			wantTier: TierAutoSkip,
		},
		{
			name: "Filename matches signature pattern",
			att: &Attachment{
				Filename:  "email_signature.jpg",
				MimeType:  "image/jpeg",
				SizeBytes: 25000, // 25KB
			},
			wantTier: TierAutoSkip,
		},
		{
			name: "Image001.png pattern",
			att: &Attachment{
				Filename:  "image001.png",
				MimeType:  "image/png",
				SizeBytes: 30000,
			},
			wantTier: TierAutoSkip,
		},
		{
			name: "Tracking pixel GIF",
			att: &Attachment{
				Filename:  "pixel.gif",
				MimeType:  "image/gif",
				SizeBytes: 100, // Tiny
			},
			wantTier: TierAutoSkip,
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			classification, err := classifier.Classify(ctx, tt.att)
			if err != nil {
				t.Fatalf("Classify() error = %v", err)
			}
			if classification.Tier != tt.wantTier {
				t.Errorf("Classify() tier = %v, want %v (reason: %s)",
					classification.Tier, tt.wantTier, classification.Reason)
			}
		})
	}
}

func TestHeuristicClassifier_PendingReview(t *testing.T) {
	classifier, err := NewHeuristicClassifier(DefaultHeuristicRules())
	if err != nil {
		t.Fatalf("failed to create classifier: %v", err)
	}

	tests := []struct {
		name     string
		att      *Attachment
		wantTier ProcessingTier
	}{
		{
			name: "Medium image (uncertain)",
			att: &Attachment{
				Filename:  "screenshot.png",
				MimeType:  "image/png",
				SizeBytes: 50000, // 50KB - between 20KB and 100KB
			},
			wantTier: TierPendingReview,
		},
		{
			name: "Unknown file type",
			att: &Attachment{
				Filename:  "data.xyz",
				MimeType:  "application/octet-stream",
				SizeBytes: 100000,
			},
			wantTier: TierPendingReview,
		},
		{
			name: "Medium GIF (uncertain)",
			att: &Attachment{
				Filename:  "animation.gif",
				MimeType:  "image/gif",
				SizeBytes: 50000, // 50KB - between tiny and large, uncertain
			},
			wantTier: TierPendingReview,
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			classification, err := classifier.Classify(ctx, tt.att)
			if err != nil {
				t.Fatalf("Classify() error = %v", err)
			}
			if classification.Tier != tt.wantTier {
				t.Errorf("Classify() tier = %v, want %v (reason: %s)",
					classification.Tier, tt.wantTier, classification.Reason)
			}
		})
	}
}

func TestHeuristicClassifier_EmbeddedEmail(t *testing.T) {
	classifier, err := NewHeuristicClassifier(DefaultHeuristicRules())
	if err != nil {
		t.Fatalf("failed to create classifier: %v", err)
	}

	att := &Attachment{
		Filename:  "original_message.eml",
		MimeType:  "message/rfc822",
		SizeBytes: 50000,
	}

	ctx := context.Background()
	classification, err := classifier.Classify(ctx, att)
	if err != nil {
		t.Fatalf("Classify() error = %v", err)
	}

	if classification.Tier != TierAutoProcess {
		t.Errorf("embedded email should be auto_process, got %v", classification.Tier)
	}

	if !att.IsEmbeddedEmail {
		t.Error("IsEmbeddedEmail flag should be set")
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{500, "500B"},
		{1024, "1KB"},
		{2048, "2KB"},
		{1536, "1.5KB"},
		{1048576, "1MB"},
		{1572864, "1.5MB"},
		{104857600, "100MB"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := formatSize(tt.bytes)
			if got != tt.want {
				t.Errorf("formatSize(%d) = %s, want %s", tt.bytes, got, tt.want)
			}
		})
	}
}
