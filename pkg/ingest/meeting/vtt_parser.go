package meeting

import (
	"bufio"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// VTT parsing regular expressions
var (
	// Matches segment header: 1 "Speaker Name" (speaker_id) or just: 1 "" (0)
	vttSegmentHeaderRegex = regexp.MustCompile(`^\d+\s+"([^"]*)"(?:\s+\((\d+)\))?`)

	// Matches timestamp line: 00:00:05.579 --> 00:00:06.858
	vttTimestampRegex = regexp.MustCompile(`^(\d{2}:\d{2}:\d{2}\.\d{3})\s+-->\s+(\d{2}:\d{2}:\d{2}\.\d{3})`)
)

// ParseVTT parses a WebVTT format transcript file.
func ParseVTT(r io.Reader) (*TranscriptResult, error) {
	scanner := bufio.NewScanner(r)
	result := &TranscriptResult{
		Segments: make([]TranscriptSegment, 0),
		Speakers: make([]string, 0),
		Format:   "vtt",
	}

	speakerSet := make(map[string]bool)
	var textBuilder strings.Builder

	var currentSegment *TranscriptSegment
	var lastEndMs int

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and WEBVTT header
		if line == "" || line == "WEBVTT" {
			continue
		}

		// Try to match segment header (e.g., 1 "Speaker Name" (123))
		if matches := vttSegmentHeaderRegex.FindStringSubmatch(line); matches != nil {
			// Save previous segment if exists
			if currentSegment != nil && currentSegment.Text != "" {
				result.Segments = append(result.Segments, *currentSegment)
			}

			speaker := matches[1]
			speakerID := ""
			if len(matches) > 2 {
				speakerID = matches[2]
			}

			currentSegment = &TranscriptSegment{
				Speaker:   speaker,
				SpeakerID: speakerID,
			}

			// Track unique speakers (skip empty speaker names)
			if speaker != "" && !speakerSet[speaker] {
				speakerSet[speaker] = true
				result.Speakers = append(result.Speakers, speaker)
			}
			continue
		}

		// Try to match timestamp line
		if matches := vttTimestampRegex.FindStringSubmatch(line); matches != nil {
			startMs := parseVTTTimestamp(matches[1])
			endMs := parseVTTTimestamp(matches[2])

			if currentSegment != nil {
				currentSegment.StartMs = startMs
				currentSegment.EndMs = endMs
			}

			if endMs > lastEndMs {
				lastEndMs = endMs
			}
			continue
		}

		// Must be text content
		if currentSegment != nil {
			if currentSegment.Text != "" {
				currentSegment.Text += " "
			}
			currentSegment.Text += line

			// Add to full text
			if textBuilder.Len() > 0 {
				textBuilder.WriteString(" ")
			}
			textBuilder.WriteString(line)
		}
	}

	// Don't forget the last segment
	if currentSegment != nil && currentSegment.Text != "" {
		result.Segments = append(result.Segments, *currentSegment)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Set duration (convert ms to seconds)
	result.DurationSeconds = lastEndMs / 1000
	result.FullText = textBuilder.String()

	return result, nil
}

// parseVTTTimestamp parses a VTT timestamp (HH:MM:SS.mmm) to milliseconds.
func parseVTTTimestamp(ts string) int {
	// Format: 00:00:05.579
	parts := strings.Split(ts, ":")
	if len(parts) != 3 {
		return 0
	}

	hours, _ := strconv.Atoi(parts[0])
	minutes, _ := strconv.Atoi(parts[1])

	// Split seconds and milliseconds
	secParts := strings.Split(parts[2], ".")
	seconds, _ := strconv.Atoi(secParts[0])
	milliseconds := 0
	if len(secParts) > 1 {
		milliseconds, _ = strconv.Atoi(secParts[1])
	}

	return hours*3600000 + minutes*60000 + seconds*1000 + milliseconds
}
