package meeting

import (
	"bufio"
	"io"
	"regexp"
	"strings"
	"time"
)

// Chat log parsing regular expressions
var (
	// Matches chat line: 2025-09-09 09:07 : Speaker Name : message
	// Also handles optional "----->" prefix
	chatLineRegex = regexp.MustCompile(`^(?:---+>\s*)?(\d{4}-\d{2}-\d{2})\s+(\d{2}:\d{2})\s*:\s*([^:]+?)\s*:\s*(.+)$`)

	// Matches URLs in text (both plain and in HTML anchor tags)
	urlRegex = regexp.MustCompile(`https?://[^\s<>"]+`)

	// Matches href attribute in anchor tags
	hrefRegex = regexp.MustCompile(`href="([^"]+)"`)
)

// ParseChatLog parses a chat log file.
// Format: YYYY-MM-DD HH:MM : Speaker Name : message
func ParseChatLog(r io.Reader) (*ChatResult, error) {
	scanner := bufio.NewScanner(r)
	result := &ChatResult{
		Messages: make([]ChatMessage, 0),
		Speakers: make([]string, 0),
		URLs:     make([]string, 0),
	}

	speakerSet := make(map[string]bool)
	urlSet := make(map[string]bool)
	var firstTime, lastTime time.Time

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines
		if line == "" {
			continue
		}

		// Try to match chat line
		matches := chatLineRegex.FindStringSubmatch(line)
		if matches == nil {
			// Skip malformed lines
			continue
		}

		dateStr := matches[1]
		timeStr := matches[2]
		speaker := strings.TrimSpace(matches[3])
		message := strings.TrimSpace(matches[4])

		// Parse timestamp
		timestampStr := dateStr + " " + timeStr
		timestamp, err := time.Parse("2006-01-02 15:04", timestampStr)
		if err != nil {
			// Skip lines with invalid timestamps
			continue
		}

		// Extract URLs from message
		messageURLs := extractURLs(message)

		chatMsg := ChatMessage{
			Timestamp: timestamp,
			Speaker:   speaker,
			Message:   message,
			URLs:      messageURLs,
		}

		result.Messages = append(result.Messages, chatMsg)

		// Track unique speakers
		if !speakerSet[speaker] {
			speakerSet[speaker] = true
			result.Speakers = append(result.Speakers, speaker)
		}

		// Track unique URLs
		for _, url := range messageURLs {
			if !urlSet[url] {
				urlSet[url] = true
				result.URLs = append(result.URLs, url)
			}
		}

		// Track time range
		if firstTime.IsZero() || timestamp.Before(firstTime) {
			firstTime = timestamp
		}
		if lastTime.IsZero() || timestamp.After(lastTime) {
			lastTime = timestamp
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	result.StartTime = firstTime
	result.EndTime = lastTime

	return result, nil
}

// extractURLs extracts all URLs from a message, including from HTML anchor tags.
func extractURLs(message string) []string {
	urls := make([]string, 0)
	urlSet := make(map[string]bool)

	// Extract URLs from href attributes first (higher priority)
	hrefMatches := hrefRegex.FindAllStringSubmatch(message, -1)
	for _, match := range hrefMatches {
		if len(match) > 1 {
			url := match[1]
			if !urlSet[url] {
				urlSet[url] = true
				urls = append(urls, url)
			}
		}
	}

	// Also extract plain URLs
	plainMatches := urlRegex.FindAllString(message, -1)
	for _, url := range plainMatches {
		// Clean up trailing punctuation
		url = strings.TrimRight(url, ".,;:!?)")
		if !urlSet[url] {
			urlSet[url] = true
			urls = append(urls, url)
		}
	}

	return urls
}
