package meeting

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseChatLog_BasicFormat(t *testing.T) {
	chatContent := `2025-09-09 09:07 : Alice : Hello everyone!
2025-09-09 09:08 : Bob : Hi Alice
2025-09-09 09:10 : Alice : Let's get started.
`

	result, err := ParseChatLog(strings.NewReader(chatContent))
	require.NoError(t, err)

	// Should extract 3 messages
	assert.Len(t, result.Messages, 3)

	// Should extract 2 unique speakers
	assert.Len(t, result.Speakers, 2)
	assert.Contains(t, result.Speakers, "Alice")
	assert.Contains(t, result.Speakers, "Bob")

	// Check first message
	assert.Equal(t, "Alice", result.Messages[0].Speaker)
	assert.Equal(t, "Hello everyone!", result.Messages[0].Message)
	assert.Equal(t, 2025, result.Messages[0].Timestamp.Year())
	assert.Equal(t, time.September, result.Messages[0].Timestamp.Month())
	assert.Equal(t, 9, result.Messages[0].Timestamp.Day())
}

func TestParseChatLog_ExtractsURLs(t *testing.T) {
	chatContent := `2025-09-09 09:07 : Alice : Check this link: https://example.com/doc
2025-09-09 09:08 : Bob : Also see <a href="https://confluence.com/page">confluence</a>
2025-09-09 09:09 : Alice : And http://docs.google.com/spreadsheet
`

	result, err := ParseChatLog(strings.NewReader(chatContent))
	require.NoError(t, err)

	// Should extract URLs from messages
	assert.GreaterOrEqual(t, len(result.URLs), 3)
	assert.Contains(t, result.URLs, "https://example.com/doc")
	assert.Contains(t, result.URLs, "https://confluence.com/page")
	assert.Contains(t, result.URLs, "http://docs.google.com/spreadsheet")

	// Individual messages should have their URLs
	assert.Contains(t, result.Messages[0].URLs, "https://example.com/doc")
}

func TestParseChatLog_HandlesHTMLLinks(t *testing.T) {
	chatContent := `2025-09-09 09:07 : Alice : <a href="https://example.com/path" alt="some text" onClick="return false;">https://example.com/path</a>
`

	result, err := ParseChatLog(strings.NewReader(chatContent))
	require.NoError(t, err)

	// Should extract URL from HTML anchor tag
	assert.Len(t, result.URLs, 1)
	assert.Contains(t, result.URLs, "https://example.com/path")
}

func TestParseChatLog_TracksTimeRange(t *testing.T) {
	chatContent := `2025-09-09 09:00 : Alice : Start of chat.
2025-09-09 09:30 : Bob : Middle message.
2025-09-09 10:00 : Alice : End of chat.
`

	result, err := ParseChatLog(strings.NewReader(chatContent))
	require.NoError(t, err)

	// Should track start and end times
	assert.Equal(t, 9, result.StartTime.Hour())
	assert.Equal(t, 0, result.StartTime.Minute())
	assert.Equal(t, 10, result.EndTime.Hour())
	assert.Equal(t, 0, result.EndTime.Minute())
}

func TestParseChatLog_HandlesArrowMarkers(t *testing.T) {
	// Some chat exports have special markers like "----->"
	chatContent := `2025-09-09 09:07 : Alice : Regular message
-----> 2025-09-09 09:08 : Bob : Highlighted message
2025-09-09 09:09 : Alice : Another regular message
`

	result, err := ParseChatLog(strings.NewReader(chatContent))
	require.NoError(t, err)

	// Should parse all messages, skipping the arrow marker
	assert.Len(t, result.Messages, 3)
}

func TestParseChatLog_RealFile(t *testing.T) {
	// Skip if test data not available
	testFile := filepath.Join(os.Getenv("HOME"), "github/otherjamesbrown/penfold_test_data/meetings-small/TT MTC TER - 09092025/Chat messages_Massiel Campos_s meeting_20250909.txt")
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Skip("Test data file not found")
	}

	f, err := os.Open(testFile)
	require.NoError(t, err)
	defer f.Close()

	result, err := ParseChatLog(f)
	require.NoError(t, err)

	// Should have messages
	assert.Greater(t, len(result.Messages), 0, "Should have chat messages")

	// Should have speakers
	assert.Greater(t, len(result.Speakers), 0, "Should have speakers")

	// Should have URLs (the test file has several)
	assert.Greater(t, len(result.URLs), 0, "Should have URLs")

	// Log some info for debugging
	t.Logf("Parsed %d messages from %d speakers, %d URLs",
		len(result.Messages), len(result.Speakers), len(result.URLs))
	t.Logf("Time range: %s to %s", result.StartTime.Format("15:04"), result.EndTime.Format("15:04"))
}

func TestParseChatLog_EmptyInput(t *testing.T) {
	result, err := ParseChatLog(strings.NewReader(""))
	require.NoError(t, err)
	assert.Len(t, result.Messages, 0)
	assert.Len(t, result.Speakers, 0)
}

func TestParseChatLog_MalformedLines(t *testing.T) {
	chatContent := `2025-09-09 09:07 : Alice : Good line.
This line has no timestamp
Another bad line
2025-09-09 09:10 : Bob : Another good line.
`

	result, err := ParseChatLog(strings.NewReader(chatContent))
	require.NoError(t, err)

	// Should have 2 valid messages
	assert.Len(t, result.Messages, 2)
}
