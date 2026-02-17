// Package meeting provides parsing and processing for meeting transcripts and chat logs.
package meeting

import "time"

// TranscriptSegment represents a single segment of a transcript.
type TranscriptSegment struct {
	Speaker   string `json:"speaker,omitempty"`
	SpeakerID string `json:"speaker_id,omitempty"`
	Text      string `json:"text"`
	StartMs   int    `json:"start_ms"`
	EndMs     int    `json:"end_ms"`
}

// TranscriptResult is the result of parsing a transcript file.
type TranscriptResult struct {
	Segments        []TranscriptSegment `json:"segments"`
	Speakers        []string            `json:"speakers"`
	DurationSeconds int                 `json:"duration_seconds"`
	FullText        string              `json:"full_text"`
	Format          string              `json:"format"` // "vtt", "txt"
}

// ChatMessage represents a single chat message.
type ChatMessage struct {
	Timestamp time.Time `json:"timestamp"`
	Speaker   string    `json:"speaker"`
	Message   string    `json:"message"`
	URLs      []string  `json:"urls,omitempty"`
}

// ChatResult is the result of parsing a chat log file.
type ChatResult struct {
	Messages  []ChatMessage `json:"messages"`
	Speakers  []string      `json:"speakers"`
	URLs      []string      `json:"urls"`      // All URLs mentioned in chat
	StartTime time.Time     `json:"start_time"`
	EndTime   time.Time     `json:"end_time"`
}

// MeetingFiles represents the files associated with a meeting.
type MeetingFiles struct {
	TranscriptPath string   `json:"transcript_path,omitempty"`
	ChatPath       string   `json:"chat_path,omitempty"`
	VideoPath      string   `json:"video_path,omitempty"`
	AudioPath      string   `json:"audio_path,omitempty"`
	OtherPaths     []string `json:"other_paths,omitempty"`
}

// Meeting represents a parsed meeting with all its components.
type Meeting struct {
	Title           string            `json:"title"`
	Date            time.Time         `json:"date"`
	Platform        string            `json:"platform"`
	DurationSeconds int               `json:"duration_seconds"`
	Participants    []string          `json:"participants"`
	Files           MeetingFiles      `json:"files"`
	Transcript      *TranscriptResult `json:"transcript,omitempty"`
	Chat            *ChatResult       `json:"chat,omitempty"`
}
