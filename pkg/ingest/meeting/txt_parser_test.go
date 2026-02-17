package meeting

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTXTTranscript_BasicFormat(t *testing.T) {
	txtContent := `0:11 : Sara Weisman (she/her) : Hey, we didn't talk about notes.
0:20 : Massiel Campos : Yes.
0:28 : Sara Weisman (she/her) : Let's see who's gonna show up.
0:30 : Massiel Campos : Yeah.
`

	result, err := ParseTXTTranscript(strings.NewReader(txtContent))
	require.NoError(t, err)

	// Should extract 4 segments
	assert.Len(t, result.Segments, 4)

	// Should extract 2 unique speakers
	assert.Len(t, result.Speakers, 2)
	assert.Contains(t, result.Speakers, "Sara Weisman (she/her)")
	assert.Contains(t, result.Speakers, "Massiel Campos")

	// Check first segment
	assert.Equal(t, "Sara Weisman (she/her)", result.Segments[0].Speaker)
	assert.Equal(t, "Hey, we didn't talk about notes.", result.Segments[0].Text)
	assert.Equal(t, 11000, result.Segments[0].StartMs) // 0:11 = 11 seconds = 11000ms
}

func TestParseTXTTranscript_TimestampFormats(t *testing.T) {
	// Test various timestamp formats: M:SS and MM:SS
	txtContent := `0:05 : Speaker A : Five seconds in.
1:30 : Speaker B : One minute thirty.
12:45 : Speaker A : Twelve minutes forty-five.
`

	result, err := ParseTXTTranscript(strings.NewReader(txtContent))
	require.NoError(t, err)

	assert.Len(t, result.Segments, 3)
	assert.Equal(t, 5000, result.Segments[0].StartMs)    // 0:05 = 5s
	assert.Equal(t, 90000, result.Segments[1].StartMs)   // 1:30 = 90s
	assert.Equal(t, 765000, result.Segments[2].StartMs)  // 12:45 = 765s
}

func TestParseTXTTranscript_ExtractsSpeakers(t *testing.T) {
	txtContent := `0:00 : Alice : Hello.
0:05 : Bob : Hi Alice.
0:10 : Charlie : Hey everyone.
0:15 : Alice : Let's start.
`

	result, err := ParseTXTTranscript(strings.NewReader(txtContent))
	require.NoError(t, err)

	// Should have 3 unique speakers
	assert.Len(t, result.Speakers, 3)
	assert.Contains(t, result.Speakers, "Alice")
	assert.Contains(t, result.Speakers, "Bob")
	assert.Contains(t, result.Speakers, "Charlie")
}

func TestParseTXTTranscript_CalculatesDuration(t *testing.T) {
	txtContent := `0:00 : Speaker : Start.
5:30 : Speaker : Middle.
10:45 : Speaker : End of meeting.
`

	result, err := ParseTXTTranscript(strings.NewReader(txtContent))
	require.NoError(t, err)

	// Duration should be based on last timestamp (10:45 = 645 seconds)
	assert.Equal(t, 645, result.DurationSeconds)
}

func TestParseTXTTranscript_GeneratesFullText(t *testing.T) {
	txtContent := `0:00 : Alice : Hello everyone.
0:05 : Bob : Hi Alice, how are you?
`

	result, err := ParseTXTTranscript(strings.NewReader(txtContent))
	require.NoError(t, err)

	assert.Contains(t, result.FullText, "Hello everyone")
	assert.Contains(t, result.FullText, "Hi Alice, how are you?")
}

func TestParseTXTTranscript_RealFile(t *testing.T) {
	// Skip if test data not available
	testFile := filepath.Join(os.Getenv("HOME"), "github/otherjamesbrown/penfold_test_data/meetings-small/TT MTC TER - 09092025/Transcript_Massiel Campos_s meeting_20250909.txt")
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Skip("Test data file not found")
	}

	f, err := os.Open(testFile)
	require.NoError(t, err)
	defer f.Close()

	result, err := ParseTXTTranscript(f)
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
	t.Logf("First 3 speakers: %v", result.Speakers[:min(3, len(result.Speakers))])
}

func TestParseTXTTranscript_EmptyInput(t *testing.T) {
	result, err := ParseTXTTranscript(strings.NewReader(""))
	require.NoError(t, err)
	assert.Len(t, result.Segments, 0)
	assert.Len(t, result.Speakers, 0)
}

func TestParseTXTTranscript_MalformedLines(t *testing.T) {
	// Parser should be lenient and skip malformed lines
	txtContent := `0:00 : Speaker : Good line.
This line has no timestamp or speaker
Another bad line
0:10 : Speaker : Another good line.
`

	result, err := ParseTXTTranscript(strings.NewReader(txtContent))
	require.NoError(t, err)

	// Should have 2 valid segments
	assert.Len(t, result.Segments, 2)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
