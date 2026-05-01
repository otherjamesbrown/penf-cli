package meeting

import (
	"bufio"
	"bytes"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// TXT transcript parsing regular expressions
var (
	// Matches Webex/Teams timestamped format: 0:11 : Speaker Name : Text content
	txtTranscriptLineRegex = regexp.MustCompile(`^(\d+):(\d{2})\s*:\s*([^:]+?)\s*:\s*(.+)$`)

	// Matches MacWhisper / hand-labelled format: "JAMES: text" or "James Brown (CEO): text"
	// Speaker names may include spaces, hyphens, apostrophes, and optional "(role)" suffix.
	speakerLabelRegex = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9 '\-]*(?:\s*\([^)]+\))?)\s*:\s+(.+)$`)
)

// ParseTXTTranscript parses a plain text transcript file.
// Format: timestamp : Speaker Name : text
func ParseTXTTranscript(r io.Reader) (*TranscriptResult, error) {
	scanner := bufio.NewScanner(r)
	result := &TranscriptResult{
		Segments: make([]TranscriptSegment, 0),
		Speakers: make([]string, 0),
		Format:   "txt",
	}

	speakerSet := make(map[string]bool)
	var textBuilder strings.Builder
	var lastTimestampMs int

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines
		if line == "" {
			continue
		}

		// Try to match transcript line
		matches := txtTranscriptLineRegex.FindStringSubmatch(line)
		if matches == nil {
			// Skip malformed lines
			continue
		}

		minutes, _ := strconv.Atoi(matches[1])
		seconds, _ := strconv.Atoi(matches[2])
		speaker := strings.TrimSpace(matches[3])
		text := strings.TrimSpace(matches[4])

		timestampMs := (minutes*60 + seconds) * 1000

		segment := TranscriptSegment{
			Speaker: speaker,
			Text:    text,
			StartMs: timestampMs,
			EndMs:   timestampMs, // TXT format doesn't have end times
		}

		result.Segments = append(result.Segments, segment)

		// Track unique speakers
		if !speakerSet[speaker] {
			speakerSet[speaker] = true
			result.Speakers = append(result.Speakers, speaker)
		}

		// Track last timestamp for duration
		if timestampMs > lastTimestampMs {
			lastTimestampMs = timestampMs
		}

		// Add to full text
		if textBuilder.Len() > 0 {
			textBuilder.WriteString(" ")
		}
		textBuilder.WriteString(text)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Set duration (convert ms to seconds)
	result.DurationSeconds = lastTimestampMs / 1000
	result.FullText = textBuilder.String()

	return result, nil
}

// ParseMacWhisperTranscript parses a plain-text transcript with speaker-label turns.
//
// Canonical format (one turn per line, blank lines between turns):
//
//	JAMES: Hiya, how are you?
//
//	ROB: I'm good, thank you.
//
// Also accepts "NAME (role): text" and mixed-case names.
// Lines that don't match the speaker-label pattern are appended to FullText only.
func ParseMacWhisperTranscript(r io.Reader) (*TranscriptResult, error) {
	scanner := bufio.NewScanner(r)
	result := &TranscriptResult{
		Segments: make([]TranscriptSegment, 0),
		Speakers: make([]string, 0),
		Format:   "txt",
	}

	speakerSet := make(map[string]bool)
	var textBuilder strings.Builder

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		matches := speakerLabelRegex.FindStringSubmatch(line)
		if matches == nil {
			if textBuilder.Len() > 0 {
				textBuilder.WriteString(" ")
			}
			textBuilder.WriteString(line)
			continue
		}

		speaker := strings.TrimSpace(matches[1])
		text := strings.TrimSpace(matches[2])

		result.Segments = append(result.Segments, TranscriptSegment{
			Speaker: speaker,
			Text:    text,
		})

		if !speakerSet[speaker] {
			speakerSet[speaker] = true
			result.Speakers = append(result.Speakers, speaker)
		}

		if textBuilder.Len() > 0 {
			textBuilder.WriteString(" ")
		}
		textBuilder.WriteString(text)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	result.FullText = textBuilder.String()
	return result, nil
}

// ParseTXTAuto reads a plain-text transcript and dispatches to the right parser.
// It buffers the full content so the format can be detected before parsing.
func ParseTXTAuto(r io.Reader) (*TranscriptResult, error) {
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	if isMacWhisperFormat(content) {
		return ParseMacWhisperTranscript(bytes.NewReader(content))
	}
	return ParseTXTTranscript(bytes.NewReader(content))
}

// isMacWhisperFormat returns true when the content looks like a speaker-label
// transcript rather than the Webex/Teams timestamped format.
func isMacWhisperFormat(content []byte) bool {
	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		// First non-empty line determines format.
		if txtTranscriptLineRegex.MatchString(line) {
			return false
		}
		if speakerLabelRegex.MatchString(line) {
			return true
		}
		// Unrecognised first line — default to MacWhisper (more permissive).
		return true
	}
	return true
}
