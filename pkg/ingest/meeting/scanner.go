package meeting

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// ScanOptions controls the behaviour of ScanMeetingFilesWithOptions.
type ScanOptions struct {
	// Platform enables lenient single-file mode for "macwhisper" or "local":
	// any .txt file is accepted as a transcript regardless of its filename.
	Platform string
}

// File detection patterns
var (
	// VTT/MP4 filename pattern: Meeting Title-YYYYMMDD HHMM-1.ext
	vttMP4Pattern = regexp.MustCompile(`^(.+)-(\d{8})\s+(\d{4})-\d+\.(vtt|mp4|webm)$`)

	// Transcript filename: Transcript_Owner_s meeting_YYYYMMDD.txt
	transcriptPattern = regexp.MustCompile(`^Transcript_.+_(\d{8})\.txt$`)

	// Chat filename: Chat messages_Owner_s meeting_YYYYMMDD.txt
	chatPattern = regexp.MustCompile(`^Chat messages_.+_(\d{8})\.txt$`)

	// Directory name with date: Meeting Name - MMDDYYYY or Meeting Name MMDDYYYY
	dirDatePattern = regexp.MustCompile(`^(.+?)[\s-]+(\d{8})$`)

	// Date extraction from YYYYMMDD
	dateYYYYMMDD = regexp.MustCompile(`(\d{4})(\d{2})(\d{2})`)

	// Date extraction from MMDDYYYY
	dateMMDDYYYY = regexp.MustCompile(`(\d{2})(\d{2})(\d{4})`)
)

// MeetingInfo contains extracted meeting metadata from filename/path.
type MeetingInfo struct {
	Title string
	Date  time.Time
}

// ScanMeetingFiles scans a path (file or directory) and returns discovered meetings.
func ScanMeetingFiles(path string) ([]*Meeting, error) {
	return ScanMeetingFilesWithOptions(path, ScanOptions{})
}

// ScanMeetingFilesWithOptions scans a path using the given options.
// When opts.Platform is "macwhisper" or "local", any .txt file is accepted as a
// transcript regardless of filename, enabling ingest of arbitrarily-named files.
func ScanMeetingFilesWithOptions(path string, opts ScanOptions) ([]*Meeting, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	lenient := isLenientPlatform(opts.Platform)

	if !info.IsDir() {
		// Single file
		m := createMeetingFromFileWithMode(path, lenient)
		if m != nil {
			return []*Meeting{m}, nil
		}
		// When lenient mode is on and we got nil it means the file extension is
		// truly unsupported (e.g. .pdf) — surface a clear empty result.
		return []*Meeting{}, nil
	}

	// Directory — use existing strict logic regardless of platform.
	return scanDirectory(path)
}

// isLenientPlatform reports whether a platform name enables loose filename matching.
func isLenientPlatform(platform string) bool {
	switch strings.ToLower(platform) {
	case "macwhisper", "local":
		return true
	}
	return false
}

// scanDirectory scans a directory for meeting files and groups them.
func scanDirectory(dirPath string) ([]*Meeting, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	// First, check if this directory itself is a meeting folder
	// A meeting folder has complementary files (transcript + chat) for the SAME meeting
	// Multiple VTT files = multiple meetings, should NOT be treated as single folder
	hasMeetingFiles := false
	hasSubdirs := false
	transcriptCount := 0
	chatCount := 0

	for _, entry := range entries {
		if entry.IsDir() {
			hasSubdirs = true
		} else {
			fileType := DetectFileType(entry.Name())
			if fileType != "unknown" {
				hasMeetingFiles = true
				if fileType == "transcript" || fileType == "vtt" {
					transcriptCount++
				}
				if fileType == "chat" {
					chatCount++
				}
			}
		}
	}

	// If this directory has meeting files, no subdirectories, AND only one transcript,
	// treat it as a single meeting folder
	if hasMeetingFiles && !hasSubdirs && transcriptCount <= 1 {
		return scanMeetingDirectory(dirPath)
	}

	meetings := make([]*Meeting, 0)

	// First pass: process subdirectories as potential meeting folders
	for _, entry := range entries {
		if entry.IsDir() {
			subPath := filepath.Join(dirPath, entry.Name())
			subMeetings, err := scanMeetingDirectory(subPath)
			if err != nil {
				continue // Skip problematic directories
			}
			meetings = append(meetings, subMeetings...)
		}
	}

	// Second pass: process standalone files (files at root level)
	fileGroups := make(map[string]*Meeting) // Group by date key

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()
		filePath := filepath.Join(dirPath, filename)
		fileType := DetectFileType(filename)

		if fileType == "unknown" {
			continue
		}

		// Extract date for grouping
		meetingInfo := ExtractMeetingInfo(filename)
		groupKey := meetingInfo.Date.Format("20060102")
		if meetingInfo.Title != "" {
			groupKey = meetingInfo.Title + "-" + groupKey
		}

		// Get or create meeting for this group
		meeting, exists := fileGroups[groupKey]
		if !exists {
			meeting = &Meeting{
				Title:    meetingInfo.Title,
				Date:     meetingInfo.Date,
				Platform: "webex", // Default, could be detected
				Files:    MeetingFiles{},
			}
			fileGroups[groupKey] = meeting
		}

		// Assign file to appropriate slot
		switch fileType {
		case "vtt", "transcript":
			if meeting.Files.TranscriptPath == "" {
				meeting.Files.TranscriptPath = filePath
			}
		case "chat":
			meeting.Files.ChatPath = filePath
		case "video":
			meeting.Files.VideoPath = filePath
		case "audio":
			meeting.Files.AudioPath = filePath
		}
	}

	// Add grouped meetings
	for _, meeting := range fileGroups {
		meetings = append(meetings, meeting)
	}

	return meetings, nil
}

// scanMeetingDirectory scans a single directory that represents one meeting.
func scanMeetingDirectory(dirPath string) ([]*Meeting, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	// Check if this directory has meeting files
	var transcriptPath, chatPath, videoPath, audioPath string

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()
		filePath := filepath.Join(dirPath, filename)
		fileType := DetectFileType(filename)

		switch fileType {
		case "vtt", "transcript":
			if transcriptPath == "" {
				transcriptPath = filePath
			}
		case "chat":
			chatPath = filePath
		case "video":
			videoPath = filePath
		case "audio":
			audioPath = filePath
		}
	}

	// If no meeting files found, skip this directory
	if transcriptPath == "" && chatPath == "" && videoPath == "" {
		return []*Meeting{}, nil
	}

	// Extract meeting info from directory name
	dirName := filepath.Base(dirPath)
	meetingInfo := ExtractMeetingInfo(dirName)

	// If title empty, try to get from transcript filename
	if meetingInfo.Title == "" && transcriptPath != "" {
		meetingInfo = ExtractMeetingInfo(filepath.Base(transcriptPath))
	}

	meeting := &Meeting{
		Title:    meetingInfo.Title,
		Date:     meetingInfo.Date,
		Platform: "webex",
		Files: MeetingFiles{
			TranscriptPath: transcriptPath,
			ChatPath:       chatPath,
			VideoPath:      videoPath,
			AudioPath:      audioPath,
		},
	}

	return []*Meeting{meeting}, nil
}

// createMeetingFromFile creates a meeting from a single file using strict filename matching.
func createMeetingFromFile(filePath string) *Meeting {
	return createMeetingFromFileWithMode(filePath, false)
}

// createMeetingFromFileWithMode creates a meeting from a single file.
// When lenient is true any .txt file is treated as a transcript.
func createMeetingFromFileWithMode(filePath string, lenient bool) *Meeting {
	filename := filepath.Base(filePath)
	var fileType string
	if lenient {
		fileType = detectFileTypeLenient(filename)
	} else {
		fileType = DetectFileType(filename)
	}

	if fileType == "unknown" {
		return nil
	}

	meetingInfo := ExtractMeetingInfo(filename)

	m := &Meeting{
		Title:    meetingInfo.Title,
		Date:     meetingInfo.Date,
		Platform: "webex",
		Files:    MeetingFiles{},
	}

	switch fileType {
	case "vtt", "transcript":
		m.Files.TranscriptPath = filePath
	case "chat":
		m.Files.ChatPath = filePath
	case "video":
		m.Files.VideoPath = filePath
	case "audio":
		m.Files.AudioPath = filePath
	}

	return m
}

// detectFileTypeLenient is like DetectFileType but accepts any .txt file as a transcript.
func detectFileTypeLenient(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".vtt":
		return "vtt"
	case ".mp4", ".webm", ".mov", ".avi":
		return "video"
	case ".m4a", ".mp3", ".wav":
		return "audio"
	case ".txt":
		return "transcript"
	}
	return "unknown"
}

// DetectFileType determines the type of a meeting-related file.
func DetectFileType(filename string) string {
	lower := strings.ToLower(filename)

	// Check extension first
	ext := strings.ToLower(filepath.Ext(filename))

	switch ext {
	case ".vtt":
		return "vtt"
	case ".mp4", ".webm", ".mov", ".avi":
		return "video"
	case ".m4a", ".mp3", ".wav":
		return "audio"
	case ".txt":
		// Distinguish between transcript and chat
		if strings.HasPrefix(lower, "transcript_") {
			return "transcript"
		}
		if strings.HasPrefix(lower, "chat messages_") || strings.HasPrefix(lower, "chat_") {
			return "chat"
		}
		// Check content patterns
		if transcriptPattern.MatchString(filename) {
			return "transcript"
		}
		if chatPattern.MatchString(filename) {
			return "chat"
		}
		return "unknown"
	}

	return "unknown"
}

// ExtractMeetingInfo extracts meeting title and date from a filename or directory name.
func ExtractMeetingInfo(name string) MeetingInfo {
	info := MeetingInfo{}

	// Remove extension if present
	name = strings.TrimSuffix(name, filepath.Ext(name))

	// Try VTT/MP4 pattern: Meeting Title-YYYYMMDD HHMM-1
	if matches := vttMP4Pattern.FindStringSubmatch(name + ".vtt"); matches != nil {
		info.Title = strings.TrimSpace(matches[1])
		info.Date = parseDate(matches[2])
		return info
	}

	// Try transcript pattern
	if matches := transcriptPattern.FindStringSubmatch(filepath.Base(name) + ".txt"); matches != nil {
		info.Date = parseDate(matches[1])
		return info
	}

	// Try directory date pattern: Meeting Name - MMDDYYYY or YYYYMMDD
	if matches := dirDatePattern.FindStringSubmatch(name); matches != nil {
		info.Title = strings.TrimSpace(matches[1])
		dateStr := matches[2]

		// Try YYYYMMDD first
		if date := parseDate(dateStr); !date.IsZero() {
			info.Date = date
		} else {
			// Try MMDDYYYY
			info.Date = parseDateMMDDYYYY(dateStr)
		}
		return info
	}

	// Fallback: just use the name as title
	info.Title = name

	return info
}

// parseDate parses YYYYMMDD format.
func parseDate(s string) time.Time {
	if len(s) != 8 {
		return time.Time{}
	}

	t, err := time.Parse("20060102", s)
	if err != nil {
		return time.Time{}
	}
	return t
}

// parseDateMMDDYYYY parses MMDDYYYY format.
func parseDateMMDDYYYY(s string) time.Time {
	if len(s) != 8 {
		return time.Time{}
	}

	// MMDDYYYY -> YYYYMMDD
	reordered := s[4:8] + s[0:2] + s[2:4]
	return parseDate(reordered)
}

// Date suffix patterns to strip from titles
var titleDatePatterns = []*regexp.Regexp{
	regexp.MustCompile(`[_\s]*\d{8}$`),              // _20251007 or 20251007 at end
	regexp.MustCompile(`\s*-?\s*\d{8}\s+\d{4}-\d+$`), // -20250228 1936-1 pattern
}

// NormalizeTitle cleans up a meeting title by removing date suffixes and extra whitespace.
func NormalizeTitle(title string) string {
	normalized := title

	// Strip date patterns
	for _, pattern := range titleDatePatterns {
		normalized = pattern.ReplaceAllString(normalized, "")
	}

	// Collapse multiple spaces
	normalized = regexp.MustCompile(`\s+`).ReplaceAllString(normalized, " ")

	// Trim whitespace
	normalized = strings.TrimSpace(normalized)

	return normalized
}
