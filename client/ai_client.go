// Package client provides gRPC clients for Penfold services.
package client

import (
	"context"
	"fmt"
	"sync"
	"time"

	aiv1 "github.com/otherjamesbrown/penf-cli/api/proto/ai/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
)

// AIClient manages the connection to the Penfold AI service through the Gateway.
type AIClient struct {
	conn       *grpc.ClientConn
	client     aiv1.AICoordinatorServiceClient
	serverAddr string
	options    *ClientOptions
	mu         sync.RWMutex
	connected  bool
}

// NewAIClient creates a new AIClient with the given options.
// Call Connect() to establish the connection.
func NewAIClient(serverAddr string, opts *ClientOptions) *AIClient {
	if opts == nil {
		opts = DefaultOptions()
	}

	return &AIClient{
		serverAddr: serverAddr,
		options:    opts,
	}
}

// Connect establishes a connection to the AI service via the Gateway.
func (c *AIClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected && c.conn != nil {
		return nil // Already connected.
	}

	// Create connection context with timeout.
	connectCtx, cancel := context.WithTimeout(ctx, c.options.ConnectTimeout)
	defer cancel()

	// Build dial options.
	dialOpts := c.buildDialOptions()

	// Establish connection.
	conn, err := grpc.DialContext(connectCtx, c.serverAddr, dialOpts...)
	if err != nil {
		return fmt.Errorf("connecting to AI service at %s: %w", c.serverAddr, err)
	}

	c.conn = conn
	c.client = aiv1.NewAICoordinatorServiceClient(conn)
	c.connected = true

	return nil
}

// buildDialOptions constructs the gRPC dial options from client configuration.
func (c *AIClient) buildDialOptions() []grpc.DialOption {
	opts := []grpc.DialOption{
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                c.options.KeepaliveTime,
			Timeout:             c.options.KeepaliveTimeout,
			PermitWithoutStream: true,
		}),
		grpc.WithDefaultCallOptions(
			grpc.WaitForReady(true),
		),
		// Block on dial to detect connection failures early.
		// Without this, DialContext returns immediately and connection happens lazily,
		// which can cause confusing hangs when the server is unreachable.
		grpc.WithBlock(),
	}

	// Configure credentials.
	if c.options.Insecure {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else if c.options.TLSConfig != nil {
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(c.options.TLSConfig)))
	} else {
		// Default to insecure if no TLS config provided
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	return opts
}

// Close closes the connection to the AI service.
func (c *AIClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected || c.conn == nil {
		return nil
	}

	err := c.conn.Close()
	c.conn = nil
	c.client = nil
	c.connected = false

	if err != nil {
		return fmt.Errorf("closing AI connection: %w", err)
	}

	return nil
}

// IsConnected returns true if the client has an active connection.
func (c *AIClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected && c.conn != nil
}

// contextWithTenant returns a context with the tenant ID in metadata.
func (c *AIClient) contextWithTenant(ctx context.Context, tenantID string) context.Context {
	if tenantID == "" {
		c.mu.RLock()
		tenantID = c.options.TenantID
		c.mu.RUnlock()
	}
	if tenantID == "" {
		return ctx
	}
	md := metadata.Pairs("x-tenant-id", tenantID)
	return metadata.NewOutgoingContext(ctx, md)
}

// QueryRequest represents a request for RAG-style question answering.
type QueryRequest struct {
	Question     string
	TenantID     string
	ContextLimit int32
	Model        string
	MaxTokens    int32
	Temperature  float32
}

// QueryResponse represents the response from a query operation.
type QueryResponse struct {
	ResponseID   string
	Answer       string
	Sources      []QuerySource
	ModelUsed    string
	InputTokens  int32
	OutputTokens int32
	LatencyMs    float64
}

// QuerySource represents a source document used in answering a query.
type QuerySource struct {
	SourceID    string
	Title       string
	ContentType string
	Relevance   float32
	Snippet     string
}

// Query performs RAG-style question answering over the knowledge base.
func (c *AIClient) Query(ctx context.Context, req *QueryRequest) (*QueryResponse, error) {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("AI client not connected")
	}

	ctx = c.contextWithTenant(ctx, req.TenantID)

	// Build the proto request.
	protoReq := &aiv1.QueryRequest{
		Question: req.Question,
	}
	if req.TenantID != "" {
		protoReq.TenantId = &req.TenantID
	}
	if req.ContextLimit > 0 {
		protoReq.ContextLimit = &req.ContextLimit
	}
	if req.Model != "" {
		protoReq.Model = &req.Model
	}
	if req.MaxTokens > 0 {
		protoReq.MaxTokens = &req.MaxTokens
	}
	if req.Temperature > 0 {
		protoReq.Temperature = &req.Temperature
	}

	// Apply a default timeout only if the parent context has no deadline.
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 10*time.Minute)
		defer cancel()
	}

	resp, err := client.Query(ctx, protoReq)
	if err != nil {
		return nil, fmt.Errorf("query request failed: %w", err)
	}

	return c.convertQueryResponse(resp), nil
}

// convertQueryResponse converts the proto response to our QueryResponse type.
func (c *AIClient) convertQueryResponse(resp *aiv1.QueryResponse) *QueryResponse {
	if resp == nil {
		return &QueryResponse{}
	}

	result := &QueryResponse{
		ResponseID: resp.GetResponseId(),
		Answer:     resp.GetAnswer(),
		ModelUsed:  resp.GetModelUsed(),
	}

	if resp.InputTokens != nil {
		result.InputTokens = *resp.InputTokens
	}
	if resp.OutputTokens != nil {
		result.OutputTokens = *resp.OutputTokens
	}
	if resp.LatencyMs != nil {
		result.LatencyMs = *resp.LatencyMs
	}

	for _, src := range resp.GetSources() {
		source := QuerySource{
			SourceID:    src.GetSourceId(),
			Title:       src.GetTitle(),
			ContentType: src.GetContentType(),
			Relevance:   src.GetRelevance(),
		}
		if src.Snippet != nil {
			source.Snippet = *src.Snippet
		}
		result.Sources = append(result.Sources, source)
	}

	return result
}

// SummarizeRequest represents a request to summarize content by ID.
type SummarizeRequest struct {
	ContentID string
	TenantID  string
	Length    string // "brief", "standard", "detailed"
	Model     string
}

// SummarizeResponse represents the response from a summarize operation.
type SummarizeResponse struct {
	ResponseID   string
	ContentID    string
	Summary      string
	KeyPoints    []string
	ModelUsed    string
	ContentType  string
	InputTokens  int32
	OutputTokens int32
	LatencyMs    float64
}

// Summarize generates a summary of content by ID.
func (c *AIClient) Summarize(ctx context.Context, req *SummarizeRequest) (*SummarizeResponse, error) {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("AI client not connected")
	}

	ctx = c.contextWithTenant(ctx, req.TenantID)

	// Build the proto request.
	protoReq := &aiv1.SummarizeByIDRequest{
		ContentId: req.ContentID,
	}
	if req.TenantID != "" {
		protoReq.TenantId = &req.TenantID
	}
	if req.Length != "" {
		protoReq.Length = &req.Length
	}
	if req.Model != "" {
		protoReq.Model = &req.Model
	}

	// Apply a default timeout only if the parent context has no deadline.
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 10*time.Minute)
		defer cancel()
	}

	resp, err := client.SummarizeByID(ctx, protoReq)
	if err != nil {
		return nil, fmt.Errorf("summarize request failed: %w", err)
	}

	return c.convertSummarizeResponse(resp), nil
}

// convertSummarizeResponse converts the proto response to our SummarizeResponse type.
func (c *AIClient) convertSummarizeResponse(resp *aiv1.SummarizeByIDResponse) *SummarizeResponse {
	if resp == nil {
		return &SummarizeResponse{}
	}

	result := &SummarizeResponse{
		ResponseID:  resp.GetResponseId(),
		ContentID:   resp.GetContentId(),
		Summary:     resp.GetSummary(),
		KeyPoints:   resp.GetKeyPoints(),
		ModelUsed:   resp.GetModelUsed(),
		ContentType: resp.GetContentType(),
	}

	if resp.InputTokens != nil {
		result.InputTokens = *resp.InputTokens
	}
	if resp.OutputTokens != nil {
		result.OutputTokens = *resp.OutputTokens
	}
	if resp.LatencyMs != nil {
		result.LatencyMs = *resp.LatencyMs
	}

	return result
}

// AnalyzeRequest represents a request to analyze content by ID.
type AnalyzeRequest struct {
	ContentID    string
	TenantID     string
	AnalysisType string // "sentiment", "entities", "topics", "action", "full"
	Model        string
}

// AnalyzeResponse represents the response from an analyze operation.
type AnalyzeResponse struct {
	ResponseID   string
	ContentID    string
	AnalysisType string
	ContentType  string
	Summary      string
	Sentiment    *SentimentResult
	Entities     []ExtractedEntity
	Topics       []TopicResult
	ActionItems  []ActionItem
	Insights     []string
	ModelUsed    string
	InputTokens  int32
	OutputTokens int32
	LatencyMs    float64
}

// SentimentResult contains sentiment analysis results.
type SentimentResult struct {
	Score      float32
	Label      string
	Confidence float32
	Indicators []string
}

// ExtractedEntity represents an entity found in content.
type ExtractedEntity struct {
	Name         string
	EntityType   string
	MentionCount int32
	Role         string
}

// TopicResult represents a topic identified in content.
type TopicResult struct {
	Topic      string
	Confidence float32
	Keywords   []string
}

// ActionItem represents an action item extracted from content.
type ActionItem struct {
	Description string
	Priority    string
	Assignee    string
	DueDate     string
}

// Analyze performs deep analysis on content by ID.
func (c *AIClient) Analyze(ctx context.Context, req *AnalyzeRequest) (*AnalyzeResponse, error) {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("AI client not connected")
	}

	ctx = c.contextWithTenant(ctx, req.TenantID)

	// Build the proto request.
	protoReq := &aiv1.AnalyzeByIDRequest{
		ContentId: req.ContentID,
	}
	if req.TenantID != "" {
		protoReq.TenantId = &req.TenantID
	}
	if req.Model != "" {
		protoReq.Model = &req.Model
	}

	// Map analysis type string to enum
	switch req.AnalysisType {
	case "sentiment":
		protoReq.AnalysisType = aiv1.AnalysisType_ANALYSIS_TYPE_SENTIMENT
	case "entities":
		protoReq.AnalysisType = aiv1.AnalysisType_ANALYSIS_TYPE_ENTITIES
	case "topics":
		protoReq.AnalysisType = aiv1.AnalysisType_ANALYSIS_TYPE_TOPICS
	case "action":
		protoReq.AnalysisType = aiv1.AnalysisType_ANALYSIS_TYPE_ACTION
	default:
		protoReq.AnalysisType = aiv1.AnalysisType_ANALYSIS_TYPE_FULL
	}

	// Apply a default timeout only if the parent context has no deadline.
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 10*time.Minute)
		defer cancel()
	}

	resp, err := client.AnalyzeByID(ctx, protoReq)
	if err != nil {
		return nil, fmt.Errorf("analyze request failed: %w", err)
	}

	return c.convertAnalyzeResponse(resp), nil
}

// convertAnalyzeResponse converts the proto response to our AnalyzeResponse type.
func (c *AIClient) convertAnalyzeResponse(resp *aiv1.AnalyzeByIDResponse) *AnalyzeResponse {
	if resp == nil {
		return &AnalyzeResponse{}
	}

	result := &AnalyzeResponse{
		ResponseID:   resp.GetResponseId(),
		ContentID:    resp.GetContentId(),
		AnalysisType: resp.GetAnalysisType().String(),
		ContentType:  resp.GetContentType(),
		Summary:      resp.GetSummary(),
		Insights:     resp.GetInsights(),
		ModelUsed:    resp.GetModelUsed(),
	}

	if resp.InputTokens != nil {
		result.InputTokens = *resp.InputTokens
	}
	if resp.OutputTokens != nil {
		result.OutputTokens = *resp.OutputTokens
	}
	if resp.LatencyMs != nil {
		result.LatencyMs = *resp.LatencyMs
	}

	// Convert sentiment
	if resp.GetSentiment() != nil {
		result.Sentiment = &SentimentResult{
			Score:      resp.GetSentiment().GetScore(),
			Label:      resp.GetSentiment().GetLabel(),
			Confidence: resp.GetSentiment().GetConfidence(),
			Indicators: resp.GetSentiment().GetIndicators(),
		}
	}

	// Convert entities
	for _, e := range resp.GetEntities() {
		entity := ExtractedEntity{
			Name:         e.GetName(),
			EntityType:   e.GetEntityType(),
			MentionCount: e.GetMentionCount(),
		}
		if e.Role != nil {
			entity.Role = *e.Role
		}
		result.Entities = append(result.Entities, entity)
	}

	// Convert topics
	for _, t := range resp.GetTopics() {
		result.Topics = append(result.Topics, TopicResult{
			Topic:      t.GetTopic(),
			Confidence: t.GetConfidence(),
			Keywords:   t.GetKeywords(),
		})
	}

	// Convert action items
	for _, a := range resp.GetActionItems() {
		item := ActionItem{
			Description: a.GetDescription(),
		}
		if a.Priority != nil {
			item.Priority = *a.Priority
		}
		if a.Assignee != nil {
			item.Assignee = *a.Assignee
		}
		if a.DueDate != nil {
			item.DueDate = *a.DueDate
		}
		result.ActionItems = append(result.ActionItems, item)
	}

	return result
}

// ListModels returns registered AI models.
func (c *AIClient) ListModels(ctx context.Context, tenantID string) ([]*aiv1.ModelInfo, error) {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("AI client not connected")
	}

	ctx = c.contextWithTenant(ctx, tenantID)

	resp, err := client.ListModels(ctx, &aiv1.ListModelsRequest{})
	if err != nil {
		return nil, fmt.Errorf("list models request failed: %w", err)
	}

	return resp.GetModels(), nil
}

// GetModelStatus returns the status of AI models.
func (c *AIClient) GetModelStatus(ctx context.Context, tenantID string) (*aiv1.GetModelStatusResponse, error) {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("AI client not connected")
	}

	ctx = c.contextWithTenant(ctx, tenantID)

	resp, err := client.GetModelStatus(ctx, &aiv1.GetModelStatusRequest{})
	if err != nil {
		return nil, fmt.Errorf("get model status request failed: %w", err)
	}

	return resp, nil
}
