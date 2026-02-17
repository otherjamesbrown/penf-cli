package meeting

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseVTT_BasicFormat(t *testing.T) {
	vttContent := `WEBVTT

1 "" (0)
00:00:00.000 --> 00:00:05.579
Okay, that sounds good. Thanks. All right, 321.

2 "Alan Dickens" (1262511360)
00:00:05.579 --> 00:00:06.858
Go.

3 "Mitul Mehta" (3330436864)
00:00:06.858 --> 00:00:34.950
Alright, thanks everyone for joining today. This is the agenda that we have lined up.
`

	result, err := ParseVTT(strings.NewReader(vttContent))
	require.NoError(t, err)

	// Should extract segments
	assert.GreaterOrEqual(t, len(result.Segments), 3)

	// Should extract speakers
	assert.Contains(t, result.Speakers, "Alan Dickens")
	assert.Contains(t, result.Speakers, "Mitul Mehta")

	// Check first segment with speaker
	var alanSegment *TranscriptSegment
	for i := range result.Segments {
		if result.Segments[i].Speaker == "Alan Dickens" {
			alanSegment = &result.Segments[i]
			break
		}
	}
	require.NotNil(t, alanSegment)
	assert.Equal(t, "Go.", alanSegment.Text)
	assert.Equal(t, 5579, alanSegment.StartMs) // 00:00:05.579
}

func TestParseVTT_ExtractsSpeakers(t *testing.T) {
	vttContent := `WEBVTT

1 "Speaker One" (123)
00:00:00.000 --> 00:00:05.000
Hello everyone.

2 "Speaker Two" (456)
00:00:05.000 --> 00:00:10.000
Hi there.

3 "Speaker One" (123)
00:00:10.000 --> 00:00:15.000
Let's begin.
`

	result, err := ParseVTT(strings.NewReader(vttContent))
	require.NoError(t, err)

	// Should have exactly 2 unique speakers
	assert.Len(t, result.Speakers, 2)
	assert.Contains(t, result.Speakers, "Speaker One")
	assert.Contains(t, result.Speakers, "Speaker Two")
}

func TestParseVTT_CalculatesDuration(t *testing.T) {
	vttContent := `WEBVTT

1 "Speaker" (123)
00:00:00.000 --> 00:00:05.000
Start.

2 "Speaker" (123)
00:05:30.000 --> 00:05:45.500
End of meeting.
`

	result, err := ParseVTT(strings.NewReader(vttContent))
	require.NoError(t, err)

	// Duration should be the end time of last segment (5:45.5 = 345500ms = 345 seconds)
	assert.Equal(t, 345, result.DurationSeconds)
}

func TestParseVTT_GeneratesFullText(t *testing.T) {
	vttContent := `WEBVTT

1 "Alice" (1)
00:00:00.000 --> 00:00:05.000
Hello everyone.

2 "Bob" (2)
00:00:05.000 --> 00:00:10.000
Hi Alice.
`

	result, err := ParseVTT(strings.NewReader(vttContent))
	require.NoError(t, err)

	// Full text should contain all dialogue
	assert.Contains(t, result.FullText, "Hello everyone")
	assert.Contains(t, result.FullText, "Hi Alice")
}

func TestParseVTT_RealFile(t *testing.T) {
	// Skip if test data not available
	testFile := filepath.Join(os.Getenv("HOME"), "github/otherjamesbrown/penfold_test_data/meetings-small/TikTok MTC PMO - weekly-20250218 1509-1.vtt")
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Skip("Test data file not found")
	}

	f, err := os.Open(testFile)
	require.NoError(t, err)
	defer f.Close()

	result, err := ParseVTT(f)
	require.NoError(t, err)

	// Should have segments
	assert.Greater(t, len(result.Segments), 0, "Should have transcript segments")

	// Should have speakers
	assert.Greater(t, len(result.Speakers), 0, "Should have speakers")

	// Should have duration
	assert.Greater(t, result.DurationSeconds, 0, "Should have duration")

	// Should have full text
	assert.NotEmpty(t, result.FullText, "Should have full text")

	// Log some info for debugging
	t.Logf("Parsed %d segments from %d speakers, duration %d seconds",
		len(result.Segments), len(result.Speakers), result.DurationSeconds)
	t.Logf("Speakers: %v", result.Speakers)
}

func TestParseVTT_EmptyInput(t *testing.T) {
	result, err := ParseVTT(strings.NewReader(""))
	require.NoError(t, err)
	assert.Len(t, result.Segments, 0)
	assert.Len(t, result.Speakers, 0)
}

func TestParseVTT_InvalidTimestamp(t *testing.T) {
	// Parser should be lenient and skip malformed lines
	vttContent := `WEBVTT

1 "Speaker" (123)
invalid --> timestamp
Some text here.

2 "Speaker" (123)
00:00:05.000 --> 00:00:10.000
Valid segment.
`

	result, err := ParseVTT(strings.NewReader(vttContent))
	require.NoError(t, err)

	// Should still parse valid segments
	assert.GreaterOrEqual(t, len(result.Segments), 1)
}
