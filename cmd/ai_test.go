// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/otherjamesbrown/penf-cli/client"
	"github.com/otherjamesbrown/penf-cli/config"
)

// mockAIConfig creates a mock configuration for AI command testing.
func mockAIConfig() *config.CLIConfig {
	return &config.CLIConfig{
		ServerAddress: "localhost:50051",
		Timeout:       30 * time.Second,
		OutputFormat:  config.OutputFormatText,
		TenantID:      "tenant-test-001",
		Debug:         false,
		Insecure:      true,
	}
}

// createAITestDeps creates test dependencies for AI commands.
func createAITestDeps(cfg *config.CLIConfig) *AICommandDeps {
	return &AICommandDeps{
		Config:       cfg,
		OutputFormat: cfg.OutputFormat,
		LoadConfig: func() (*config.CLIConfig, error) {
			return cfg, nil
		},
		InitClient: func(c *config.CLIConfig) (*client.GRPCClient, error) {
			return nil, nil
		},
	}
}

func TestNewAICommand(t *testing.T) {
	deps := createAITestDeps(mockAIConfig())
	cmd := NewAICommand(deps)

	assert.NotNil(t, cmd)
	assert.Equal(t, "ai", cmd.Use)
	assert.Contains(t, cmd.Short, "AI-powered")

	// Check subcommands exist.
	subcommands := cmd.Commands()
	expectedSubcmds := []string{"query", "summarize", "analyze"}

	for _, expected := range expectedSubcmds {
		found := false
		for _, sub := range subcommands {
			if strings.HasPrefix(sub.Use, expected) {
				found = true
				break
			}
		}
		assert.True(t, found, "expected subcommand %q not found", expected)
	}
}

func TestNewAICommand_WithNilDeps(t *testing.T) {
	cmd := NewAICommand(nil)
	assert.NotNil(t, cmd)
	assert.Equal(t, "ai", cmd.Use)
}

func TestAICommand_QuerySubcommand(t *testing.T) {
	deps := createAITestDeps(mockAIConfig())
	cmd := NewAICommand(deps)

	// Find query subcommand.
	queryCmd, _, err := cmd.Find([]string{"query"})
	require.NoError(t, err)
	require.NotNil(t, queryCmd)

	assert.Contains(t, queryCmd.Use, "query")
	assert.Contains(t, queryCmd.Short, "question")

	// Check flags.
	flags := []string{"model", "max-tokens", "temperature", "output", "verbose", "context"}
	for _, flagName := range flags {
		flag := queryCmd.Flags().Lookup(flagName)
		assert.NotNil(t, flag, "query command missing flag: %s", flagName)
	}
}

func TestAICommand_SummarizeSubcommand(t *testing.T) {
	deps := createAITestDeps(mockAIConfig())
	cmd := NewAICommand(deps)

	// Find summarize subcommand.
	summarizeCmd, _, err := cmd.Find([]string{"summarize"})
	require.NoError(t, err)
	require.NotNil(t, summarizeCmd)

	assert.Contains(t, summarizeCmd.Use, "summarize")
	assert.Contains(t, summarizeCmd.Short, "summary")

	// Check flags.
	flags := []string{"length", "model", "output", "verbose"}
	for _, flagName := range flags {
		flag := summarizeCmd.Flags().Lookup(flagName)
		assert.NotNil(t, flag, "summarize command missing flag: %s", flagName)
	}
}

func TestAICommand_AnalyzeSubcommand(t *testing.T) {
	deps := createAITestDeps(mockAIConfig())
	cmd := NewAICommand(deps)

	// Find analyze subcommand.
	analyzeCmd, _, err := cmd.Find([]string{"analyze"})
	require.NoError(t, err)
	require.NotNil(t, analyzeCmd)

	assert.Contains(t, analyzeCmd.Use, "analyze")
	assert.Contains(t, analyzeCmd.Short, "analysis")

	// Check flags.
	flags := []string{"type", "model", "output", "verbose"}
	for _, flagName := range flags {
		flag := analyzeCmd.Flags().Lookup(flagName)
		assert.NotNil(t, flag, "analyze command missing flag: %s", flagName)
	}
}

// Note: Connection-requiring tests (TestRunAIQuery_ConnectionRequired, TestRunAIQuery_OutputFormatParsing)
// have been moved to tests/integration/cli_ai_test.go where they will fail properly if the backend is down.

func TestRunAIQuery_InvalidOutputFormat(t *testing.T) {
	cfg := mockAIConfig()
	deps := createAITestDeps(cfg)

	// Reset global flags.
	oldOutput := aiOutput
	aiOutput = "invalid"
	defer func() {
		aiOutput = oldOutput
	}()

	ctx := context.Background()
	err := runAIQuery(ctx, deps, "What are the Q4 objectives?")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid output format")
}

// Note: TestRunAISummarize_ConnectionRequired moved to tests/integration/cli_ai_test.go

func TestRunAISummarize_InvalidLength(t *testing.T) {
	cfg := mockAIConfig()
	deps := createAITestDeps(cfg)

	// Reset global flags.
	oldOutput := aiOutput
	aiOutput = ""
	defer func() {
		aiOutput = oldOutput
	}()

	ctx := context.Background()
	err := runAISummarize(ctx, deps, "doc-123", "invalid")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid summary length")
}

// Note: TestRunAISummarize_AllLengthsValidation and TestRunAIAnalyze_ConnectionRequired
// moved to tests/integration/cli_ai_test.go

func TestRunAIAnalyze_InvalidType(t *testing.T) {
	cfg := mockAIConfig()
	deps := createAITestDeps(cfg)

	oldOutput := aiOutput
	aiOutput = ""
	defer func() {
		aiOutput = oldOutput
	}()

	ctx := context.Background()
	err := runAIAnalyze(ctx, deps, "doc-123", "invalid")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid analysis type")
}

// Note: TestRunAIAnalyze_AllTypes moved to tests/integration/cli_ai_test.go

// Note: Tests for executeAIQuery, executeAISummarize, executeAIAnalyze have been removed
// because these mock functions were replaced with real gRPC calls. Integration tests
// should be used to test the actual gRPC endpoints.

func TestFormatAnalysisResponse_FullAnalysis(t *testing.T) {
	resp := &client.AnalyzeResponse{
		ResponseID:   "test-id",
		ContentID:    "doc-123",
		AnalysisType: "ANALYSIS_TYPE_FULL",
		ContentType:  "document",
		Summary:      "Test summary of the document.",
		Sentiment: &client.SentimentResult{
			Score:      0.72,
			Label:      "positive",
			Confidence: 0.85,
			Indicators: []string{"excellent", "on track"},
		},
		Entities: []client.ExtractedEntity{
			{Name: "Alice", EntityType: "person", MentionCount: 3, Role: "Lead"},
			{Name: "Acme Corp", EntityType: "organization", MentionCount: 2},
		},
		Topics: []client.TopicResult{
			{Topic: "Architecture", Confidence: 0.9, Keywords: []string{"API", "design"}},
		},
		ActionItems: []client.ActionItem{
			{Description: "Complete review", Priority: "high", Assignee: "Bob", DueDate: "2024-10-25"},
		},
		Insights:  []string{"Good progress overall", "Consider adding more tests"},
		ModelUsed: "llama-3.1-8b",
	}

	result := formatAnalysisResponse(resp, "full")

	assert.Contains(t, result, "Summary")
	assert.Contains(t, result, "Test summary")
	assert.Contains(t, result, "Sentiment Analysis")
	assert.Contains(t, result, "positive")
	assert.Contains(t, result, "Entities Extracted")
	assert.Contains(t, result, "Alice")
	assert.Contains(t, result, "Topics Identified")
	assert.Contains(t, result, "Architecture")
	assert.Contains(t, result, "Action Items")
	assert.Contains(t, result, "Complete review")
	assert.Contains(t, result, "Insights")
}

func TestFormatAnalysisResponse_SentimentOnly(t *testing.T) {
	resp := &client.AnalyzeResponse{
		Summary: "Test summary.",
		Sentiment: &client.SentimentResult{
			Score:      -0.5,
			Label:      "negative",
			Confidence: 0.9,
		},
	}

	result := formatAnalysisResponse(resp, "sentiment")

	assert.Contains(t, result, "Sentiment Analysis")
	assert.Contains(t, result, "negative")
	assert.NotContains(t, result, "Entities")
	assert.NotContains(t, result, "Topics")
}

func TestOutputAIResponse_JSON(t *testing.T) {
	response := &AIResponse{
		ID:         "test-id",
		Operation:  "query",
		Query:      "test query",
		Response:   "test response",
		Model:      "test-model",
		TokensUsed: 100,
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputAIResponse(config.OutputFormatJSON, response, false)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	assert.NoError(t, err)

	var decoded AIResponse
	err = json.Unmarshal([]byte(output), &decoded)
	assert.NoError(t, err)
	assert.Equal(t, "test-id", decoded.ID)
}

func TestOutputAIResponse_YAML(t *testing.T) {
	response := &AIResponse{
		ID:         "test-id",
		Operation:  "query",
		Query:      "test query",
		Response:   "test response",
		Model:      "test-model",
		TokensUsed: 100,
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputAIResponse(config.OutputFormatYAML, response, false)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	assert.NoError(t, err)

	var decoded AIResponse
	err = yaml.Unmarshal([]byte(output), &decoded)
	assert.NoError(t, err)
	assert.Equal(t, "test-id", decoded.ID)
}

func TestOutputAIResponse_Text(t *testing.T) {
	response := &AIResponse{
		ID:         "test-id",
		Operation:  "query",
		Query:      "test query",
		Response:   "test response",
		Model:      "test-model",
		TokensUsed: 100,
		Sources: []AISource{
			{ID: "src-1", Title: "Source 1", ContentType: "document", Relevance: 0.95},
		},
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputAIResponse(config.OutputFormatText, response, false)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "AI Query")
	assert.Contains(t, output, "test query")
	assert.Contains(t, output, "test response")
	assert.Contains(t, output, "Sources")
}

func TestOutputAIResponse_TextVerbose(t *testing.T) {
	response := &AIResponse{
		ID:         "test-id",
		Operation:  "query",
		Query:      "test query",
		Response:   "test response",
		Model:      "test-model",
		TokensUsed: 100,
		LatencyMs:  50.0,
		Metadata: map[string]string{
			"key1": "value1",
		},
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputAIResponse(config.OutputFormatText, response, true)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "Model:")
	assert.Contains(t, output, "Tokens:")
	assert.Contains(t, output, "Latency:")
	assert.Contains(t, output, "Metadata:")
}

func TestAIResponse_JSONSerialization(t *testing.T) {
	response := AIResponse{
		ID:          "test-id",
		Operation:   "query",
		Query:       "test query",
		ContentID:   "content-123",
		Response:    "test response",
		Model:       "test-model",
		TokensUsed:  100,
		LatencyMs:   50.5,
		CompletedAt: time.Now(),
		Sources: []AISource{
			{ID: "src-1", Title: "Source 1", ContentType: "document", Relevance: 0.95},
		},
		Metadata: map[string]string{
			"key1": "value1",
		},
	}

	data, err := json.Marshal(response)
	require.NoError(t, err)

	var decoded AIResponse
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, response.ID, decoded.ID)
	assert.Equal(t, response.Operation, decoded.Operation)
	assert.Equal(t, response.Query, decoded.Query)
	assert.Equal(t, response.ContentID, decoded.ContentID)
	assert.Equal(t, response.Response, decoded.Response)
	assert.Equal(t, response.Model, decoded.Model)
	assert.Equal(t, response.TokensUsed, decoded.TokensUsed)
	assert.Len(t, decoded.Sources, 1)
	assert.Equal(t, "src-1", decoded.Sources[0].ID)
}

func TestAISource_JSONSerialization(t *testing.T) {
	source := AISource{
		ID:          "src-1",
		Title:       "Test Source",
		ContentType: "document",
		Relevance:   0.95,
	}

	data, err := json.Marshal(source)
	require.NoError(t, err)

	var decoded AISource
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, source.ID, decoded.ID)
	assert.Equal(t, source.Title, decoded.Title)
	assert.Equal(t, source.ContentType, decoded.ContentType)
	assert.Equal(t, source.Relevance, decoded.Relevance)
}

func TestDefaultAIDeps(t *testing.T) {
	deps := DefaultAIDeps()

	assert.NotNil(t, deps)
	assert.NotNil(t, deps.LoadConfig)
	assert.NotNil(t, deps.InitClient)
}
