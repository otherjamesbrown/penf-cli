package eml

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func getTestdataDir() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "testdata")
}

func TestParseSimpleEmail(t *testing.T) {
	parser := NewParser(DefaultParseOptions())
	result, err := parser.ParseFile(filepath.Join(getTestdataDir(), "simple.eml"))
	if err != nil {
		t.Fatalf("failed to parse simple email: %v", err)
	}

	email := result.Email

	// Check Message-ID
	if email.MessageID != "<test123@example.com>" {
		t.Errorf("expected message id <test123@example.com>, got %s", email.MessageID)
	}
	if email.MessageIDSynthetic {
		t.Error("expected non-synthetic message id")
	}

	// Check From
	if email.From.Email != "john@example.com" {
		t.Errorf("expected from email john@example.com, got %s", email.From.Email)
	}
	if email.From.Name != "John Doe" {
		t.Errorf("expected from name 'John Doe', got %s", email.From.Name)
	}

	// Check To
	if len(email.To) != 1 {
		t.Fatalf("expected 1 To address, got %d", len(email.To))
	}
	if email.To[0].Email != "jane@example.com" {
		t.Errorf("expected to email jane@example.com, got %s", email.To[0].Email)
	}

	// Check Subject
	if email.Subject != "Test Email" {
		t.Errorf("expected subject 'Test Email', got %s", email.Subject)
	}

	// Check Date
	expectedDate := time.Date(2024, 1, 15, 10, 30, 0, 0, time.FixedZone("EST", -5*3600))
	if !email.Date.Equal(expectedDate) {
		t.Errorf("expected date %v, got %v", expectedDate, email.Date)
	}
	if email.DateFallback {
		t.Error("expected non-fallback date")
	}

	// Check Body
	if !strings.Contains(email.BodyText, "simple test email") {
		t.Errorf("expected body to contain 'simple test email', got: %s", email.BodyText)
	}

	// Check content hash is set
	if email.ContentHash == "" {
		t.Error("expected content hash to be set")
	}
}

func TestParseMultipartEmail(t *testing.T) {
	parser := NewParser(DefaultParseOptions())
	result, err := parser.ParseFile(filepath.Join(getTestdataDir(), "multipart.eml"))
	if err != nil {
		t.Fatalf("failed to parse multipart email: %v", err)
	}

	email := result.Email

	// Check Message-ID
	if email.MessageID != "<multipart456@example.com>" {
		t.Errorf("expected message id <multipart456@example.com>, got %s", email.MessageID)
	}

	// Check From
	if email.From.Email != "alice@example.com" {
		t.Errorf("expected from alice@example.com, got %s", email.From.Email)
	}

	// Check To (multiple recipients)
	if len(email.To) != 2 {
		t.Fatalf("expected 2 To addresses, got %d", len(email.To))
	}
	toEmails := email.ToAddresses()
	if toEmails[0] != "bob@example.com" || toEmails[1] != "carol@example.com" {
		t.Errorf("unexpected To addresses: %v", toEmails)
	}

	// Check Cc
	if len(email.Cc) != 1 || email.Cc[0].Email != "dave@example.com" {
		t.Errorf("expected Cc dave@example.com, got %v", email.Cc)
	}

	// Check threading headers
	if email.InReplyTo != "<original789@example.com>" {
		t.Errorf("expected In-Reply-To <original789@example.com>, got %s", email.InReplyTo)
	}
	if len(email.References) != 2 {
		t.Fatalf("expected 2 References, got %d", len(email.References))
	}

	// Check bodies
	if !strings.Contains(email.BodyText, "plain text version") {
		t.Errorf("expected plain text body, got: %s", email.BodyText)
	}
	if !strings.Contains(email.BodyHTML, "<strong>HTML</strong>") {
		t.Errorf("expected HTML body with bold, got: %s", email.BodyHTML)
	}
}

func TestParseEmailWithAttachment(t *testing.T) {
	opts := DefaultParseOptions()
	opts.IncludeAttachmentContent = true
	parser := NewParser(opts)

	result, err := parser.ParseFile(filepath.Join(getTestdataDir(), "with_attachment.eml"))
	if err != nil {
		t.Fatalf("failed to parse email with attachment: %v", err)
	}

	email := result.Email

	// Check basic fields
	if email.MessageID != "<attachment789@example.com>" {
		t.Errorf("expected message id <attachment789@example.com>, got %s", email.MessageID)
	}

	// Check attachment
	if !email.HasAttachments() {
		t.Fatal("expected email to have attachments")
	}
	if email.AttachmentCount() != 1 {
		t.Errorf("expected 1 attachment, got %d", email.AttachmentCount())
	}

	att := email.Attachments[0]
	if att.Filename != "document.txt" {
		t.Errorf("expected filename document.txt, got %s", att.Filename)
	}
	if att.MimeType != "text/plain" {
		t.Errorf("expected mime type text/plain, got %s", att.MimeType)
	}
	if att.Size == 0 {
		t.Error("expected attachment size > 0")
	}

	// Check attachment content was included
	if len(att.ContentData) == 0 {
		t.Error("expected attachment content to be included")
	}
	if !strings.Contains(string(att.ContentData), "content of the attached") {
		t.Errorf("unexpected attachment content: %s", string(att.ContentData))
	}

	// Check body is still available
	if !strings.Contains(email.BodyText, "attached document") {
		t.Errorf("expected body text about attachment, got: %s", email.BodyText)
	}
}

func TestParseMissingMessageID(t *testing.T) {
	parser := NewParser(DefaultParseOptions())
	result, err := parser.ParseFile(filepath.Join(getTestdataDir(), "no_message_id.eml"))
	if err != nil {
		t.Fatalf("failed to parse email without message id: %v", err)
	}

	email := result.Email

	// Check synthetic Message-ID
	if email.MessageID == "" {
		t.Fatal("expected synthetic message id")
	}
	if !email.MessageIDSynthetic {
		t.Error("expected MessageIDSynthetic to be true")
	}
	if !strings.HasPrefix(email.MessageID, "<synthetic-") {
		t.Errorf("expected synthetic prefix, got %s", email.MessageID)
	}
	if !strings.HasSuffix(email.MessageID, "@penfold.local>") {
		t.Errorf("expected @penfold.local suffix, got %s", email.MessageID)
	}

	// Synthetic ID should contain first 16 chars of content hash
	hashPart := email.ContentHash[:16]
	if !strings.Contains(email.MessageID, hashPart) {
		t.Errorf("expected message id to contain hash prefix %s, got %s", hashPart, email.MessageID)
	}
}

func TestParseBytes(t *testing.T) {
	rawEmail := []byte(`From: test@example.com
To: recipient@example.com
Subject: Bytes Test
Date: Fri, 19 Jan 2024 08:00:00 +0000
Message-ID: <bytes-test@example.com>

This email was parsed from raw bytes.
`)

	result, err := ParseBytes(rawEmail)
	if err != nil {
		t.Fatalf("failed to parse bytes: %v", err)
	}

	if result.Email.MessageID != "<bytes-test@example.com>" {
		t.Errorf("unexpected message id: %s", result.Email.MessageID)
	}
	if result.Email.Subject != "Bytes Test" {
		t.Errorf("unexpected subject: %s", result.Email.Subject)
	}
}

func TestContentHash(t *testing.T) {
	// Same content should produce same hash
	rawEmail := []byte(`From: test@example.com
To: recipient@example.com
Subject: Hash Test
Date: Fri, 19 Jan 2024 08:00:00 +0000

Content for hashing.
`)

	result1, _ := ParseBytes(rawEmail)
	result2, _ := ParseBytes(rawEmail)

	if result1.Email.ContentHash != result2.Email.ContentHash {
		t.Error("same content should produce same hash")
	}

	// Different content should produce different hash
	differentEmail := []byte(`From: test@example.com
To: recipient@example.com
Subject: Different Hash Test
Date: Fri, 19 Jan 2024 08:00:00 +0000

Different content for hashing.
`)

	result3, _ := ParseBytes(differentEmail)
	if result1.Email.ContentHash == result3.Email.ContentHash {
		t.Error("different content should produce different hash")
	}
}

func TestGetBody(t *testing.T) {
	// Test with both text and HTML
	parser := NewParser(DefaultParseOptions())
	result, _ := parser.ParseFile(filepath.Join(getTestdataDir(), "multipart.eml"))

	body := result.Email.GetBody()
	if !strings.Contains(body, "plain text version") {
		t.Error("GetBody should prefer plain text over HTML")
	}

	// Test with only HTML
	email := &ParsedEmail{
		BodyHTML: "<p>HTML only</p>",
	}
	if email.GetBody() != "<p>HTML only</p>" {
		t.Error("GetBody should return HTML when text is empty")
	}
}

func TestMaxBodySize(t *testing.T) {
	opts := DefaultParseOptions()
	opts.MaxBodySize = 10

	parser := NewParser(opts)
	result, err := parser.ParseFile(filepath.Join(getTestdataDir(), "simple.eml"))
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	if len(result.Email.BodyText) > 10 {
		t.Errorf("body should be limited to 10 chars, got %d", len(result.Email.BodyText))
	}
}

func TestPreserveHeaders(t *testing.T) {
	opts := DefaultParseOptions()
	opts.PreserveHeaders = []string{"X-Custom-Header", "Nonexistent-Header"}

	rawEmail := []byte(`From: test@example.com
To: recipient@example.com
Subject: Custom Headers
X-Custom-Header: custom-value
Date: Fri, 19 Jan 2024 08:00:00 +0000
Message-ID: <headers@example.com>

Body text.
`)

	parser := NewParser(opts)
	result, err := parser.ParseBytes(rawEmail)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	if result.Email.Headers == nil {
		t.Fatal("expected headers map to be initialized")
	}

	if result.Email.Headers["X-Custom-Header"] != "custom-value" {
		t.Errorf("expected X-Custom-Header=custom-value, got %s", result.Email.Headers["X-Custom-Header"])
	}

	// Nonexistent header should not be in map
	if _, exists := result.Email.Headers["Nonexistent-Header"]; exists {
		t.Error("nonexistent header should not be in map")
	}
}

func TestAddressHelpers(t *testing.T) {
	email := &ParsedEmail{
		To: []Address{
			{Email: "to1@example.com", Name: "To One"},
			{Email: "to2@example.com", Name: "To Two"},
		},
		Cc: []Address{
			{Email: "cc1@example.com", Name: "Cc One"},
		},
	}

	toAddrs := email.ToAddresses()
	if len(toAddrs) != 2 || toAddrs[0] != "to1@example.com" || toAddrs[1] != "to2@example.com" {
		t.Errorf("unexpected ToAddresses: %v", toAddrs)
	}

	ccAddrs := email.CcAddresses()
	if len(ccAddrs) != 1 || ccAddrs[0] != "cc1@example.com" {
		t.Errorf("unexpected CcAddresses: %v", ccAddrs)
	}
}

func TestAllParticipantEmails(t *testing.T) {
	email := &ParsedEmail{
		From: Address{Email: "sender@example.com", Name: "Sender"},
		To: []Address{
			{Email: "to1@example.com", Name: "To One"},
			{Email: "to2@example.com", Name: "To Two"},
		},
		Cc: []Address{
			{Email: "cc1@example.com", Name: "Cc One"},
		},
		Bcc: []Address{
			{Email: "bcc1@example.com", Name: "Bcc One"},
			{Email: "bcc2@example.com", Name: "Bcc Two"},
		},
	}

	participants := email.AllParticipantEmails()

	// Should have 6 participants: 1 from + 2 to + 1 cc + 2 bcc
	if len(participants) != 6 {
		t.Errorf("expected 6 participants, got %d: %v", len(participants), participants)
	}

	// Check order: From, To, Cc, Bcc
	expected := []string{
		"sender@example.com",
		"to1@example.com",
		"to2@example.com",
		"cc1@example.com",
		"bcc1@example.com",
		"bcc2@example.com",
	}
	for i, exp := range expected {
		if participants[i] != exp {
			t.Errorf("participant[%d]: expected %q, got %q", i, exp, participants[i])
		}
	}
}

func TestAllParticipantEmailsEmpty(t *testing.T) {
	email := &ParsedEmail{}
	participants := email.AllParticipantEmails()
	if len(participants) != 0 {
		t.Errorf("expected empty participants for empty email, got %v", participants)
	}
}

func TestBccAddresses(t *testing.T) {
	email := &ParsedEmail{
		Bcc: []Address{
			{Email: "bcc1@example.com", Name: "Bcc One"},
			{Email: "bcc2@example.com", Name: "Bcc Two"},
		},
	}

	bccAddrs := email.BccAddresses()
	if len(bccAddrs) != 2 || bccAddrs[0] != "bcc1@example.com" || bccAddrs[1] != "bcc2@example.com" {
		t.Errorf("unexpected BccAddresses: %v", bccAddrs)
	}
}

func TestFilePath(t *testing.T) {
	parser := NewParser(DefaultParseOptions())
	result, err := parser.ParseFile(filepath.Join(getTestdataDir(), "simple.eml"))
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	if result.Email.FilePath == "" {
		t.Error("expected FilePath to be set")
	}
	if !strings.HasSuffix(result.Email.FilePath, "simple.eml") {
		t.Errorf("unexpected FilePath: %s", result.Email.FilePath)
	}
	if !filepath.IsAbs(result.Email.FilePath) {
		t.Error("expected FilePath to be absolute")
	}
}
