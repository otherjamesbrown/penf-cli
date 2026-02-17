package meeting

import (
	"bufio"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// TXT transcript parsing regular expressions
var (
	// Matches transcript line: 0:11 : Speaker Name : Text content
	// or: 12:45 : Speaker Name (pronouns) : Text content
	txtTranscriptLineRegex = regexp.MustCompile(`^(\d+):(\d{2})\s*:\s*([^:]+?)\s*:\s*(.+)$`)
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
