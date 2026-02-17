package meeting

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScanMeetingFiles_SingleVTTFile(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	vttPath := filepath.Join(tmpDir, "Team Meeting-20250218 1509-1.vtt")
	err := os.WriteFile(vttPath, []byte("WEBVTT\n\n1 \"Speaker\" (1)\n00:00:00.000 --> 00:00:05.000\nHello"), 0644)
	require.NoError(t, err)

	meetings, err := ScanMeetingFiles(vttPath)
	require.NoError(t, err)

	assert.Len(t, meetings, 1)
	assert.Equal(t, "Team Meeting", meetings[0].Title)
	assert.Equal(t, 2025, meetings[0].Date.Year())
	assert.Equal(t, vttPath, meetings[0].Files.TranscriptPath)
}

func TestScanMeetingFiles_DirectoryWithTranscriptAndChat(t *testing.T) {
	// Create temp directory with transcript and chat
	tmpDir := t.TempDir()
	meetingDir := filepath.Join(tmpDir, "TT MTC TER - 09092025")
	err := os.MkdirAll(meetingDir, 0755)
	require.NoError(t, err)

	// Create transcript file
	transcriptPath := filepath.Join(meetingDir, "Transcript_Massiel Campos_s meeting_20250909.txt")
	err = os.WriteFile(transcriptPath, []byte("0:00 : Speaker : Hello"), 0644)
	require.NoError(t, err)

	// Create chat file
	chatPath := filepath.Join(meetingDir, "Chat messages_Massiel Campos_s meeting_20250909.txt")
	err = os.WriteFile(chatPath, []byte("2025-09-09 09:00 : Speaker : Hi"), 0644)
	require.NoError(t, err)

	meetings, err := ScanMeetingFiles(meetingDir)
	require.NoError(t, err)

	assert.Len(t, meetings, 1)
	assert.Contains(t, meetings[0].Title, "TT MTC TER")
	assert.NotEmpty(t, meetings[0].Files.TranscriptPath)
	assert.NotEmpty(t, meetings[0].Files.ChatPath)
}

func TestScanMeetingFiles_DirectoryWithVTTAndMP4(t *testing.T) {
	// Create temp directory with VTT and MP4
	tmpDir := t.TempDir()
	meetingDir := filepath.Join(tmpDir, "DBaaS Meeting-20250228 1936-1")
	err := os.MkdirAll(meetingDir, 0755)
	require.NoError(t, err)

	vttPath := filepath.Join(meetingDir, "DBaaS Meeting-20250228 1936-1.vtt")
	err = os.WriteFile(vttPath, []byte("WEBVTT\n\n"), 0644)
	require.NoError(t, err)

	mp4Path := filepath.Join(meetingDir, "DBaaS Meeting-20250228 1936-1.mp4")
	err = os.WriteFile(mp4Path, []byte{}, 0644)
	require.NoError(t, err)

	meetings, err := ScanMeetingFiles(meetingDir)
	require.NoError(t, err)

	assert.Len(t, meetings, 1)
	assert.Equal(t, vttPath, meetings[0].Files.TranscriptPath)
	assert.Equal(t, mp4Path, meetings[0].Files.VideoPath)
	assert.True(t, meetings[0].Files.VideoPath != "")
}

func TestScanMeetingFiles_MultipleMeetingsInDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create multiple VTT files (different meetings)
	files := []string{
		"Meeting A-20250218 1509-1.vtt",
		"Meeting B-20250219 1000-1.vtt",
		"Meeting C-20250220 1400-1.vtt",
	}

	for _, f := range files {
		path := filepath.Join(tmpDir, f)
		err := os.WriteFile(path, []byte("WEBVTT\n\n"), 0644)
		require.NoError(t, err)
	}

	meetings, err := ScanMeetingFiles(tmpDir)
	require.NoError(t, err)

	assert.Len(t, meetings, 3)
}

func TestExtractMeetingInfo_VTTFilename(t *testing.T) {
	tests := []struct {
		filename string
		title    string
		hasDate  bool
	}{
		{"TikTok MTC PMO - weekly-20250218 1509-1.vtt", "TikTok MTC PMO - weekly", true},
		{"DBaaS and VPC-20250228 1936-1.vtt", "DBaaS and VPC", true},
		{"Simple Meeting-20250101 0900-1.vtt", "Simple Meeting", true},
	}

	for _, tc := range tests {
		t.Run(tc.filename, func(t *testing.T) {
			info := ExtractMeetingInfo(tc.filename)
			assert.Equal(t, tc.title, info.Title)
			if tc.hasDate {
				assert.False(t, info.Date.IsZero())
			}
		})
	}
}

func TestExtractMeetingInfo_TranscriptFilename(t *testing.T) {
	tests := []struct {
		filename string
		hasDate  bool
	}{
		{"Transcript_Massiel Campos_s meeting_20250909.txt", true},
		{"Transcript_John Doe_s meeting_20251015.txt", true},
	}

	for _, tc := range tests {
		t.Run(tc.filename, func(t *testing.T) {
			info := ExtractMeetingInfo(tc.filename)
			if tc.hasDate {
				assert.False(t, info.Date.IsZero())
			}
		})
	}
}

func TestExtractMeetingInfo_DirectoryName(t *testing.T) {
	tests := []struct {
		dirname string
		title   string
	}{
		{"TT MTC TER - 09092025", "TT MTC TER"},
		{"TT MTC TER 09302025", "TT MTC TER"},
		{"DBaaS and VPC-20250228 1936-1", "DBaaS and VPC"},
	}

	for _, tc := range tests {
		t.Run(tc.dirname, func(t *testing.T) {
			info := ExtractMeetingInfo(tc.dirname)
			assert.Equal(t, tc.title, info.Title)
		})
	}
}

func TestScanMeetingFiles_RealTestData(t *testing.T) {
	// Skip if test data not available
	testDir := filepath.Join(os.Getenv("HOME"), "github/otherjamesbrown/penfold_test_data/meetings-small")
	if _, err := os.Stat(testDir); os.IsNotExist(err) {
		t.Skip("Test data directory not found")
	}

	meetings, err := ScanMeetingFiles(testDir)
	require.NoError(t, err)

	// Should find multiple meetings
	assert.Greater(t, len(meetings), 0, "Should find meetings")

	// Log what we found
	for i, m := range meetings {
		t.Logf("Meeting %d: %s (date: %s)", i+1, m.Title, m.Date.Format("2006-01-02"))
		t.Logf("  Transcript: %s", m.Files.TranscriptPath)
		t.Logf("  Chat: %s", m.Files.ChatPath)
		t.Logf("  Video: %s", m.Files.VideoPath)
	}
}

func TestDetectFileType(t *testing.T) {
	tests := []struct {
		filename string
		expected string
	}{
		{"meeting.vtt", "vtt"},
		{"Transcript_user_s meeting_20250909.txt", "transcript"},
		{"Chat messages_user_s meeting_20250909.txt", "chat"},
		{"meeting.mp4", "video"},
		{"meeting.webm", "video"},
		{"meeting.m4a", "audio"},
		{"meeting.mp3", "audio"},
		{"random.txt", "unknown"},
		{"document.pdf", "unknown"},
	}

	for _, tc := range tests {
		t.Run(tc.filename, func(t *testing.T) {
			result := DetectFileType(tc.filename)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestNormalizeTitle(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"TT MTC TER_20251007", "TT MTC TER"},
		{"TT MTC  TER_20251007", "TT MTC TER"},
		{"DBaaS and VPC-20250228 1936-1", "DBaaS and VPC"},
		{"TikTok MTC PMO - weekly", "TikTok MTC PMO - weekly"},
		{"Meeting Name 20251225", "Meeting Name"},
		{"Simple Title", "Simple Title"},
		{"  Extra   Spaces  ", "Extra Spaces"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := NormalizeTitle(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}
