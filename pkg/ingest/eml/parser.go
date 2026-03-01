package eml

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	"golang.org/x/text/transform"
)

// Parser parses RFC 5322 email messages from .eml files.
type Parser struct {
	opts ParseOptions
}

// NewParser creates a new email parser with the given options.
func NewParser(opts ParseOptions) *Parser {
	return &Parser{opts: opts}
}

// ParseFile parses an email from a file path.
func (p *Parser) ParseFile(path string) (*ParseResult, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Get file modification time for date fallback
	stat, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	opts := p.opts
	if opts.FallbackDate == nil {
		mtime := stat.ModTime()
		opts.FallbackDate = &mtime
	}

	result, err := p.parseWithOptions(data, opts)
	if err != nil {
		return nil, err
	}

	result.Email.FilePath = absPath
	return result, nil
}

// ParseBytes parses an email from raw bytes.
func (p *Parser) ParseBytes(data []byte) (*ParseResult, error) {
	return p.parseWithOptions(data, p.opts)
}

// parseWithOptions performs the actual parsing with given options.
func (p *Parser) parseWithOptions(data []byte, opts ParseOptions) (*ParseResult, error) {
	result := &ParseResult{
		Email:    &ParsedEmail{},
		Warnings: []string{},
	}

	email := result.Email
	email.RawContent = data

	// Calculate content hash
	hash := sha256.Sum256(data)
	email.ContentHash = hex.EncodeToString(hash[:])

	// Parse the email message
	msg, err := mail.ReadMessage(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to parse email: %w", err)
	}

	// Extract headers
	p.parseHeaders(msg, email, result)

	// Parse body
	if err := p.parseBody(msg, email, opts, result); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("body parsing warning: %v", err))
	}

	// Generate synthetic Message-ID if missing
	if email.MessageID == "" {
		email.MessageID = fmt.Sprintf("<synthetic-%s@penfold.local>", email.ContentHash[:16])
		email.MessageIDSynthetic = true
	}

	return result, nil
}

// parseHeaders extracts all header information from the email.
func (p *Parser) parseHeaders(msg *mail.Message, email *ParsedEmail, result *ParseResult) {
	// Message-ID
	email.MessageID = cleanMessageID(msg.Header.Get("Message-Id"))
	if email.MessageID == "" {
		email.MessageID = cleanMessageID(msg.Header.Get("Message-ID"))
	}

	// From
	if fromAddrs, err := msg.Header.AddressList("From"); err == nil && len(fromAddrs) > 0 {
		email.From = mailAddrToAddress(fromAddrs[0])
	} else {
		// Try raw parsing
		from := msg.Header.Get("From")
		if from != "" {
			email.From = parseRawAddress(from)
		}
	}

	// To
	if toAddrs, err := msg.Header.AddressList("To"); err == nil {
		email.To = mailAddrsToAddresses(toAddrs)
	} else {
		to := msg.Header.Get("To")
		if to != "" {
			email.To = parseRawAddressList(to)
		}
	}

	// Cc
	if ccAddrs, err := msg.Header.AddressList("Cc"); err == nil {
		email.Cc = mailAddrsToAddresses(ccAddrs)
	} else {
		cc := msg.Header.Get("Cc")
		if cc != "" {
			email.Cc = parseRawAddressList(cc)
		}
	}

	// Bcc (usually stripped, but check anyway)
	if bccAddrs, err := msg.Header.AddressList("Bcc"); err == nil {
		email.Bcc = mailAddrsToAddresses(bccAddrs)
	}

	// Reply-To
	if replyAddrs, err := msg.Header.AddressList("Reply-To"); err == nil && len(replyAddrs) > 0 {
		addr := mailAddrToAddress(replyAddrs[0])
		email.ReplyTo = &addr
	}

	// Subject
	email.Subject = decodeRFC2047(msg.Header.Get("Subject"))

	// Date
	email.Date, email.DateFallback = p.parseDate(msg, result)

	// Threading headers
	email.InReplyTo = cleanMessageID(msg.Header.Get("In-Reply-To"))
	if refs := msg.Header.Get("References"); refs != "" {
		email.References = parseReferences(refs)
	}

	// Content-Type
	email.ContentType = msg.Header.Get("Content-Type")

	// Preserve additional headers if requested
	if len(p.opts.PreserveHeaders) > 0 {
		email.Headers = make(map[string]string)
		for _, h := range p.opts.PreserveHeaders {
			if v := msg.Header.Get(h); v != "" {
				email.Headers[h] = decodeRFC2047(v)
			}
		}
	}
}

// parseDate extracts and parses the Date header with fallback.
func (p *Parser) parseDate(msg *mail.Message, result *ParseResult) (time.Time, bool) {
	dateStr := msg.Header.Get("Date")
	if dateStr == "" {
		if p.opts.FallbackDate != nil {
			return *p.opts.FallbackDate, true
		}
		return time.Now(), true
	}

	// Standard RFC 5322 parsing
	if t, err := mail.ParseDate(dateStr); err == nil {
		return t, false
	}

	// Try additional date formats
	formats := []string{
		time.RFC1123Z,
		time.RFC1123,
		time.RFC822Z,
		time.RFC822,
		"2 Jan 2006 15:04:05 -0700",
		"Mon, 2 Jan 2006 15:04:05 -0700 (MST)",
		"Mon, 2 Jan 2006 15:04:05 MST",
		"2006-01-02 15:04:05",
		"02 Jan 2006 15:04:05 -0700",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t, false
		}
	}

	result.Warnings = append(result.Warnings, fmt.Sprintf("could not parse date: %s", dateStr))

	if p.opts.FallbackDate != nil {
		return *p.opts.FallbackDate, true
	}
	return time.Now(), true
}

// parseBody extracts the body content from the email.
func (p *Parser) parseBody(msg *mail.Message, email *ParsedEmail, opts ParseOptions, result *ParseResult) error {
	contentType := msg.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "text/plain"
	}

	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		// Treat as plain text if Content-Type is malformed
		body, _ := io.ReadAll(msg.Body)
		email.BodyText = string(body)
		return nil
	}

	if strings.HasPrefix(mediaType, "multipart/") {
		return p.parseMultipart(msg.Body, params["boundary"], email, opts, result)
	}

	// Single part message
	body, err := io.ReadAll(msg.Body)
	if err != nil {
		return fmt.Errorf("failed to read body: %w", err)
	}

	// Decode transfer encoding
	transferEncoding := strings.ToLower(msg.Header.Get("Content-Transfer-Encoding"))
	body, err = decodeTransferEncoding(body, transferEncoding)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("transfer encoding warning: %v", err))
	}

	// Convert charset
	charset := params["charset"]
	if charset != "" && !strings.EqualFold(charset, "utf-8") && !strings.EqualFold(charset, "us-ascii") {
		decoded, err := decodeCharset(body, charset)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("charset decoding warning: %v", err))
		} else {
			body = decoded
		}
	}

	content := string(body)
	if opts.MaxBodySize > 0 && len(content) > opts.MaxBodySize {
		content = content[:opts.MaxBodySize]
	}

	if strings.HasPrefix(mediaType, "text/html") {
		email.BodyHTML = content
	} else {
		email.BodyText = content
	}

	return nil
}

// parseMultipart handles multipart MIME messages.
func (p *Parser) parseMultipart(body io.Reader, boundary string, email *ParsedEmail, opts ParseOptions, result *ParseResult) error {
	reader := multipart.NewReader(body, boundary)

	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read part: %w", err)
		}

		contentType := part.Header.Get("Content-Type")
		if contentType == "" {
			contentType = "text/plain"
		}

		mediaType, params, err := mime.ParseMediaType(contentType)
		if err != nil {
			continue
		}

		contentDisposition := part.Header.Get("Content-Disposition")
		disposition, dispParams, _ := mime.ParseMediaType(contentDisposition)

		// Recursive multipart handling
		if strings.HasPrefix(mediaType, "multipart/") {
			if err := p.parseMultipart(part, params["boundary"], email, opts, result); err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("nested multipart warning: %v", err))
			}
			continue
		}

		// Handle embedded message/rfc822 (forwarded emails) — pf-f08812
		if mediaType == "message/rfc822" {
			nestedData, readErr := io.ReadAll(part)
			if readErr == nil {
				nestedResult, parseErr := p.parseWithOptions(nestedData, opts)
				if parseErr == nil && nestedResult.Email != nil {
					nestedBody := nestedResult.Email.GetBody()
					if nestedBody != "" && email.BodyText != "" {
						email.BodyText += "\n\n" + nestedBody
					} else if nestedBody != "" {
						email.BodyText = nestedBody
					}
				}
			}
			continue
		}

		// Handle text/calendar (ICS) — extract event metadata as body text (pf-58c28d)
		if mediaType == "text/calendar" {
			icsContent, readErr := io.ReadAll(part)
			if readErr == nil {
				if summary := parseICSMetadata(string(icsContent)); summary != "" {
					if email.BodyText != "" {
						email.BodyText += "\n\n" + summary
					} else {
						email.BodyText = summary
					}
				}
			}
			continue
		}

		// Check if this is an attachment
		isAttachment := disposition == "attachment" ||
			(disposition == "inline" && dispParams["filename"] != "") ||
			(dispParams["filename"] != "" && !strings.HasPrefix(mediaType, "text/"))

		if isAttachment {
			attachment := p.parseAttachment(part, mediaType, dispParams, opts)
			email.Attachments = append(email.Attachments, attachment)
			continue
		}

		// Read part content
		content, err := io.ReadAll(part)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("failed to read part: %v", err))
			continue
		}

		// Decode transfer encoding
		transferEncoding := strings.ToLower(part.Header.Get("Content-Transfer-Encoding"))
		content, err = decodeTransferEncoding(content, transferEncoding)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("transfer encoding warning: %v", err))
		}

		// Convert charset
		charset := params["charset"]
		if charset != "" && !strings.EqualFold(charset, "utf-8") && !strings.EqualFold(charset, "us-ascii") {
			decoded, err := decodeCharset(content, charset)
			if err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("charset warning: %v", err))
			} else {
				content = decoded
			}
		}

		text := string(content)
		if opts.MaxBodySize > 0 && len(text) > opts.MaxBodySize {
			text = text[:opts.MaxBodySize]
		}

		// Store based on content type
		if strings.HasPrefix(mediaType, "text/html") {
			if email.BodyHTML == "" {
				email.BodyHTML = text
			}
		} else if strings.HasPrefix(mediaType, "text/") {
			if email.BodyText == "" {
				email.BodyText = text
			}
		}
	}

	return nil
}

// parseAttachment extracts attachment metadata and optionally content.
func (p *Parser) parseAttachment(part *multipart.Part, mediaType string, dispParams map[string]string, opts ParseOptions) Attachment {
	filename := dispParams["filename"]
	if filename == "" {
		filename = part.FileName()
	}
	if filename == "" {
		filename = "unnamed"
	}
	filename = decodeRFC2047(filename)

	contentID := part.Header.Get("Content-ID")
	contentID = strings.Trim(contentID, "<>")

	disposition := part.Header.Get("Content-Disposition")
	isInline := strings.HasPrefix(disposition, "inline")

	attachment := Attachment{
		Filename:  filename,
		MimeType:  mediaType,
		ContentID: contentID,
		IsInline:  isInline,
	}

	// Read content to get size and optionally store
	content, err := io.ReadAll(part)
	if err == nil {
		// Decode if needed
		transferEncoding := strings.ToLower(part.Header.Get("Content-Transfer-Encoding"))
		decoded, _ := decodeTransferEncoding(content, transferEncoding)
		attachment.Size = len(decoded)

		if opts.IncludeAttachmentContent {
			attachment.ContentData = decoded
		}
	}

	return attachment
}

// Helper functions

func cleanMessageID(id string) string {
	id = strings.TrimSpace(id)
	// Some message IDs don't have angle brackets
	if id != "" && !strings.HasPrefix(id, "<") {
		id = "<" + id
	}
	if id != "" && !strings.HasSuffix(id, ">") {
		id = id + ">"
	}
	return id
}

func mailAddrToAddress(addr *mail.Address) Address {
	return Address{
		Name:  addr.Name,
		Email: addr.Address,
	}
}

func mailAddrsToAddresses(addrs []*mail.Address) []Address {
	result := make([]Address, len(addrs))
	for i, addr := range addrs {
		result[i] = mailAddrToAddress(addr)
	}
	return result
}

func parseRawAddress(raw string) Address {
	raw = strings.TrimSpace(raw)
	// Try to extract email from angle brackets
	if start := strings.Index(raw, "<"); start != -1 {
		if end := strings.Index(raw, ">"); end > start {
			email := raw[start+1 : end]
			name := strings.TrimSpace(raw[:start])
			name = strings.Trim(name, "\"")
			return Address{Name: decodeRFC2047(name), Email: email}
		}
	}
	// Assume entire string is email
	return Address{Email: raw}
}

func parseRawAddressList(raw string) []Address {
	var result []Address
	// Split on comma, but be careful of commas in quoted strings
	parts := splitAddresses(raw)
	for _, part := range parts {
		addr := parseRawAddress(part)
		if addr.Email != "" {
			result = append(result, addr)
		}
	}
	return result
}

func splitAddresses(raw string) []string {
	var result []string
	var current strings.Builder
	inQuotes := false
	depth := 0

	for _, r := range raw {
		switch r {
		case '"':
			inQuotes = !inQuotes
			current.WriteRune(r)
		case '<':
			depth++
			current.WriteRune(r)
		case '>':
			depth--
			current.WriteRune(r)
		case ',':
			if !inQuotes && depth == 0 {
				if s := strings.TrimSpace(current.String()); s != "" {
					result = append(result, s)
				}
				current.Reset()
			} else {
				current.WriteRune(r)
			}
		default:
			current.WriteRune(r)
		}
	}

	if s := strings.TrimSpace(current.String()); s != "" {
		result = append(result, s)
	}

	return result
}

func parseReferences(refs string) []string {
	var result []string
	parts := strings.Fields(refs)
	for _, p := range parts {
		if id := cleanMessageID(p); id != "" {
			result = append(result, id)
		}
	}
	return result
}

func decodeRFC2047(s string) string {
	dec := new(mime.WordDecoder)
	decoded, err := dec.DecodeHeader(s)
	if err != nil {
		return s
	}
	return decoded
}

func decodeTransferEncoding(data []byte, encoding string) ([]byte, error) {
	switch encoding {
	case "base64":
		decoded := make([]byte, base64.StdEncoding.DecodedLen(len(data)))
		n, err := base64.StdEncoding.Decode(decoded, data)
		if err != nil {
			// Try with line breaks removed
			cleaned := bytes.ReplaceAll(data, []byte("\r\n"), []byte(""))
			cleaned = bytes.ReplaceAll(cleaned, []byte("\n"), []byte(""))
			decoded = make([]byte, base64.StdEncoding.DecodedLen(len(cleaned)))
			n, err = base64.StdEncoding.Decode(decoded, cleaned)
			if err != nil {
				return data, fmt.Errorf("base64 decode failed: %w", err)
			}
		}
		return decoded[:n], nil

	case "quoted-printable":
		reader := quotedprintable.NewReader(bytes.NewReader(data))
		decoded, err := io.ReadAll(reader)
		if err != nil {
			return data, fmt.Errorf("quoted-printable decode failed: %w", err)
		}
		return decoded, nil

	case "7bit", "8bit", "binary", "":
		return data, nil

	default:
		return data, nil
	}
}

func decodeCharset(data []byte, charset string) ([]byte, error) {
	charset = strings.ToLower(strings.TrimSpace(charset))

	// Map common charset names to encodings
	var decoder transform.Transformer
	switch charset {
	case "iso-8859-1", "latin1", "iso_8859-1":
		decoder = charmap.ISO8859_1.NewDecoder()
	case "iso-8859-2", "latin2":
		decoder = charmap.ISO8859_2.NewDecoder()
	case "iso-8859-15", "latin9":
		decoder = charmap.ISO8859_15.NewDecoder()
	case "windows-1252", "cp1252":
		decoder = charmap.Windows1252.NewDecoder()
	case "windows-1251", "cp1251":
		decoder = charmap.Windows1251.NewDecoder()
	case "koi8-r":
		decoder = charmap.KOI8R.NewDecoder()
	case "gb2312", "gbk", "gb18030":
		decoder = simplifiedchinese.GBK.NewDecoder()
	case "big5":
		decoder = traditionalchinese.Big5.NewDecoder()
	case "euc-jp":
		decoder = japanese.EUCJP.NewDecoder()
	case "iso-2022-jp":
		decoder = japanese.ISO2022JP.NewDecoder()
	case "shift_jis", "shift-jis", "sjis":
		decoder = japanese.ShiftJIS.NewDecoder()
	case "euc-kr":
		decoder = korean.EUCKR.NewDecoder()
	default:
		// Unknown charset - return as-is
		return data, fmt.Errorf("unknown charset: %s", charset)
	}

	reader := transform.NewReader(bytes.NewReader(data), decoder)
	result, err := io.ReadAll(reader)
	if err != nil {
		return data, fmt.Errorf("charset decoding failed: %w", err)
	}
	return result, nil
}

// ParseFile is a convenience function for parsing a single file with default options.
func ParseFile(path string) (*ParseResult, error) {
	parser := NewParser(DefaultParseOptions())
	return parser.ParseFile(path)
}

// ParseBytes is a convenience function for parsing raw bytes with default options.
func ParseBytes(data []byte) (*ParseResult, error) {
	parser := NewParser(DefaultParseOptions())
	return parser.ParseBytes(data)
}

// parseICSMetadata extracts key calendar event fields from iCalendar data
// and returns a human-readable summary. Returns "" if no VEVENT is found.
func parseICSMetadata(icsData string) string {
	var summary, dtStart, dtEnd, organizer string
	var attendees []string
	inEvent := false

	for _, line := range strings.Split(icsData, "\n") {
		line = strings.TrimRight(line, "\r")

		if line == "BEGIN:VEVENT" {
			inEvent = true
			continue
		}
		if line == "END:VEVENT" {
			break
		}
		if !inEvent {
			continue
		}

		// Use HasPrefix+Index pattern to handle parameterised properties
		// e.g. SUMMARY;LANGUAGE=en-US:Declined: James/Tom
		if strings.HasPrefix(line, "SUMMARY") {
			if idx := strings.Index(line, ":"); idx >= 0 {
				summary = line[idx+1:]
			}
		} else if strings.HasPrefix(line, "DTSTART") {
			if idx := strings.Index(line, ":"); idx >= 0 {
				dtStart = line[idx+1:]
			}
		} else if strings.HasPrefix(line, "DTEND") {
			if idx := strings.Index(line, ":"); idx >= 0 {
				dtEnd = line[idx+1:]
			}
		} else if strings.HasPrefix(line, "ORGANIZER") {
			if idx := strings.Index(line, ":"); idx >= 0 {
				val := line[idx+1:]
				organizer = strings.TrimPrefix(val, "mailto:")
			}
		} else if strings.HasPrefix(line, "ATTENDEE") {
			if idx := strings.Index(line, ":"); idx >= 0 {
				val := line[idx+1:]
				attendees = append(attendees, strings.TrimPrefix(val, "mailto:"))
			}
		}
	}

	if summary == "" && organizer == "" && len(attendees) == 0 {
		return ""
	}

	var parts []string
	parts = append(parts, "[Calendar Event]")
	if summary != "" {
		parts = append(parts, "Title: "+summary)
	}
	if dtStart != "" {
		parts = append(parts, "Start: "+dtStart)
	}
	if dtEnd != "" {
		parts = append(parts, "End: "+dtEnd)
	}
	if organizer != "" {
		parts = append(parts, "Organizer: "+organizer)
	}
	if len(attendees) > 0 {
		parts = append(parts, "Attendees: "+strings.Join(attendees, ", "))
	}

	return strings.Join(parts, "\n")
}
