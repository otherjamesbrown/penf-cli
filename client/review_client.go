// Package client provides gRPC clients for Penfold services.
package client

import (
	"context"
	"fmt"
	"sync"
	"time"

	reviewv1 "github.com/otherjamesbrown/penf-cli/api/proto/review/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ReviewClient manages the connection to the Penfold Review service.
type ReviewClient struct {
	conn       *grpc.ClientConn
	client     reviewv1.ReviewServiceClient
	serverAddr string
	options    *ClientOptions
	mu         sync.RWMutex
	connected  bool
}

// NewReviewClient creates a new ReviewClient with the given options.
// Call Connect() to establish the connection.
func NewReviewClient(serverAddr string, opts *ClientOptions) *ReviewClient {
	if opts == nil {
		opts = DefaultOptions()
	}

	return &ReviewClient{
		serverAddr: serverAddr,
		options:    opts,
	}
}

// Connect establishes a connection to the Review service.
func (c *ReviewClient) Connect(ctx context.Context) error {
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
		return fmt.Errorf("connecting to review service at %s: %w", c.serverAddr, err)
	}

	c.conn = conn
	c.client = reviewv1.NewReviewServiceClient(conn)
	c.connected = true

	return nil
}

// buildDialOptions constructs the gRPC dial options from client configuration.
func (c *ReviewClient) buildDialOptions() []grpc.DialOption {
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

// Close closes the connection to the Review service.
func (c *ReviewClient) Close() error {
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
		return fmt.Errorf("closing review connection: %w", err)
	}

	return nil
}

// IsConnected returns true if the client has an active connection.
func (c *ReviewClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected && c.conn != nil
}

// contextWithTenant returns a context with the tenant ID in metadata.
func (c *ReviewClient) contextWithTenant(ctx context.Context, tenantID string) context.Context {
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

// Domain types for the client

// ReviewItem represents a single item requiring user review.
type ReviewItem struct {
	ID             string
	ContentType    string
	ContentSummary string
	Source         string
	Priority       string
	Status         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	ContentID      string
	Category       string
	Metadata       map[string]string
}

// ReviewAction records an action taken on a review item.
type ReviewAction struct {
	ID             string
	ItemID         string
	ActionType     string
	PreviousStatus string
	NewStatus      string
	PerformedAt    time.Time
	Reason         string
	Undoable       bool
}

// ReviewCategory groups review items by category.
type ReviewCategory struct {
	Name        string
	DisplayName string
	Items       []*ReviewItem
	ItemCount   int32
}

// DailyReview represents the complete daily review for a user.
type DailyReview struct {
	Date              string
	Categories        []*ReviewCategory
	TotalPending      int32
	HighPriorityCount int32
	ProcessedToday    int32
}

// ReviewSession represents a user's review session.
type ReviewSession struct {
	ID                    string
	Status                string
	StartedAt             time.Time
	PausedAt              *time.Time
	EndedAt               *time.Time
	TotalReviewed         int32
	ApprovedCount         int32
	RejectedCount         int32
	DeferredCount         int32
	ActiveDurationSeconds int64
}

// ReviewStats contains review statistics.
type ReviewStats struct {
	PendingCount         int64
	ApprovedCount        int64
	RejectedCount        int64
	DeferredCount        int64
	ByPriority           map[string]int64
	ByContentType        map[string]int64
	BySource             map[string]int64
	ByCategory           map[string]int64
	AvgReviewTimeSeconds float64
}

// GetDailyReview retrieves today's review items grouped by category.
func (c *ReviewClient) GetDailyReview(ctx context.Context, tenantID string, date string, includeProcessed bool) (*DailyReview, error) {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("review client not connected")
	}

	ctx = c.contextWithTenant(ctx, tenantID)

	protoReq := &reviewv1.GetDailyReviewRequest{
		IncludeProcessed: includeProcessed,
	}
	if tenantID != "" {
		protoReq.TenantId = &tenantID
	}
	if date != "" {
		protoReq.Date = &date
	}

	resp, err := client.GetDailyReview(ctx, protoReq)
	if err != nil {
		return nil, fmt.Errorf("get daily review request failed: %w", err)
	}

	return protoDailyReviewToDailyReview(resp.Review), nil
}

// GetReviewItem retrieves a single review item by ID.
func (c *ReviewClient) GetReviewItem(ctx context.Context, tenantID, itemID string) (*ReviewItem, []*ReviewAction, error) {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	if client == nil {
		return nil, nil, fmt.Errorf("review client not connected")
	}

	ctx = c.contextWithTenant(ctx, tenantID)

	protoReq := &reviewv1.GetReviewItemRequest{
		Id: itemID,
	}
	if tenantID != "" {
		protoReq.TenantId = &tenantID
	}

	resp, err := client.GetReviewItem(ctx, protoReq)
	if err != nil {
		return nil, nil, fmt.Errorf("get review item request failed: %w", err)
	}

	actions := make([]*ReviewAction, len(resp.RecentActions))
	for i, a := range resp.RecentActions {
		actions[i] = protoToReviewAction(a)
	}

	return protoToReviewItem(resp.Item), actions, nil
}

// ListReviewItemsRequest represents a request to list review items.
type ListReviewItemsRequest struct {
	TenantID      string
	Statuses      []string
	Priorities    []string
	ContentTypes  []string
	Sources       []string
	Categories    []string
	CreatedAfter  *time.Time
	CreatedBefore *time.Time
	PageSize      int32
	PageToken     string
	SortOrder     string
}

// ListReviewItems returns a filtered, paginated list of review items.
func (c *ReviewClient) ListReviewItems(ctx context.Context, req *ListReviewItemsRequest) ([]*ReviewItem, string, int64, error) {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	if client == nil {
		return nil, "", 0, fmt.Errorf("review client not connected")
	}

	ctx = c.contextWithTenant(ctx, req.TenantID)

	protoReq := &reviewv1.ListReviewItemsRequest{
		ContentTypes: req.ContentTypes,
		Sources:      req.Sources,
		Categories:   req.Categories,
		PageSize:     req.PageSize,
		PageToken:    req.PageToken,
		SortOrder:    req.SortOrder,
	}
	if req.TenantID != "" {
		protoReq.TenantId = &req.TenantID
	}

	// Convert string statuses to proto enum
	for _, s := range req.Statuses {
		status := stringToReviewStatus(s)
		if status != reviewv1.ReviewStatus_REVIEW_STATUS_UNSPECIFIED {
			protoReq.Statuses = append(protoReq.Statuses, status)
		}
	}

	// Convert string priorities to proto enum
	for _, p := range req.Priorities {
		priority := stringToPriority(p)
		if priority != reviewv1.Priority_PRIORITY_UNSPECIFIED {
			protoReq.Priorities = append(protoReq.Priorities, priority)
		}
	}

	if req.CreatedAfter != nil {
		protoReq.CreatedAfter = timestamppb.New(*req.CreatedAfter)
	}
	if req.CreatedBefore != nil {
		protoReq.CreatedBefore = timestamppb.New(*req.CreatedBefore)
	}

	resp, err := client.ListReviewItems(ctx, protoReq)
	if err != nil {
		return nil, "", 0, fmt.Errorf("list review items request failed: %w", err)
	}

	items := make([]*ReviewItem, len(resp.Items))
	for i, item := range resp.Items {
		items[i] = protoToReviewItem(item)
	}

	var totalCount int64
	if resp.TotalCount != nil {
		totalCount = *resp.TotalCount
	}

	return items, resp.NextPageToken, totalCount, nil
}

// ApproveItem marks a review item as approved.
func (c *ReviewClient) ApproveItem(ctx context.Context, tenantID, itemID, note string) (*ReviewItem, *ReviewAction, error) {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	if client == nil {
		return nil, nil, fmt.Errorf("review client not connected")
	}

	ctx = c.contextWithTenant(ctx, tenantID)

	protoReq := &reviewv1.ApproveItemRequest{
		Id:   itemID,
		Note: note,
	}
	if tenantID != "" {
		protoReq.TenantId = &tenantID
	}

	resp, err := client.ApproveItem(ctx, protoReq)
	if err != nil {
		return nil, nil, fmt.Errorf("approve item request failed: %w", err)
	}

	return protoToReviewItem(resp.Item), protoToReviewAction(resp.Action), nil
}

// RejectItem marks a review item as rejected.
func (c *ReviewClient) RejectItem(ctx context.Context, tenantID, itemID, reason string) (*ReviewItem, *ReviewAction, error) {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	if client == nil {
		return nil, nil, fmt.Errorf("review client not connected")
	}

	ctx = c.contextWithTenant(ctx, tenantID)

	protoReq := &reviewv1.RejectItemRequest{
		Id:     itemID,
		Reason: reason,
	}
	if tenantID != "" {
		protoReq.TenantId = &tenantID
	}

	resp, err := client.RejectItem(ctx, protoReq)
	if err != nil {
		return nil, nil, fmt.Errorf("reject item request failed: %w", err)
	}

	return protoToReviewItem(resp.Item), protoToReviewAction(resp.Action), nil
}

// UndoAction reverses the last action on a review item.
func (c *ReviewClient) UndoAction(ctx context.Context, tenantID, itemID string) (*ReviewItem, *ReviewAction, bool, error) {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	if client == nil {
		return nil, nil, false, fmt.Errorf("review client not connected")
	}

	ctx = c.contextWithTenant(ctx, tenantID)

	protoReq := &reviewv1.UndoActionRequest{
		Id: itemID,
	}
	if tenantID != "" {
		protoReq.TenantId = &tenantID
	}

	resp, err := client.UndoAction(ctx, protoReq)
	if err != nil {
		return nil, nil, false, fmt.Errorf("undo action request failed: %w", err)
	}

	return protoToReviewItem(resp.Item), protoToReviewAction(resp.UndoneAction), resp.CanUndoMore, nil
}

// GetReviewStats retrieves review statistics.
func (c *ReviewClient) GetReviewStats(ctx context.Context, tenantID string, fromTime, toTime *time.Time) (*ReviewStats, error) {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("review client not connected")
	}

	ctx = c.contextWithTenant(ctx, tenantID)

	protoReq := &reviewv1.GetReviewStatsRequest{}
	if tenantID != "" {
		protoReq.TenantId = &tenantID
	}
	if fromTime != nil {
		protoReq.FromTime = timestamppb.New(*fromTime)
	}
	if toTime != nil {
		protoReq.ToTime = timestamppb.New(*toTime)
	}

	resp, err := client.GetReviewStats(ctx, protoReq)
	if err != nil {
		return nil, fmt.Errorf("get review stats request failed: %w", err)
	}

	return &ReviewStats{
		PendingCount:         resp.PendingCount,
		ApprovedCount:        resp.ApprovedCount,
		RejectedCount:        resp.RejectedCount,
		DeferredCount:        resp.DeferredCount,
		ByPriority:           resp.ByPriority,
		ByContentType:        resp.ByContentType,
		BySource:             resp.BySource,
		ByCategory:           resp.ByCategory,
		AvgReviewTimeSeconds: resp.AvgReviewTimeSeconds,
	}, nil
}

// Session management methods

// StartSession starts a new review session.
func (c *ReviewClient) StartSession(ctx context.Context, tenantID string) (*ReviewSession, bool, error) {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	if client == nil {
		return nil, false, fmt.Errorf("review client not connected")
	}

	ctx = c.contextWithTenant(ctx, tenantID)

	protoReq := &reviewv1.StartSessionRequest{}
	if tenantID != "" {
		protoReq.TenantId = &tenantID
	}

	resp, err := client.StartSession(ctx, protoReq)
	if err != nil {
		return nil, false, fmt.Errorf("start session request failed: %w", err)
	}

	return protoToReviewSession(resp.Session), resp.PreviousSessionEnded, nil
}

// PauseSession pauses the current session.
func (c *ReviewClient) PauseSession(ctx context.Context, tenantID, sessionID string) (*ReviewSession, error) {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("review client not connected")
	}

	ctx = c.contextWithTenant(ctx, tenantID)

	protoReq := &reviewv1.PauseSessionRequest{
		SessionId: sessionID,
	}
	if tenantID != "" {
		protoReq.TenantId = &tenantID
	}

	resp, err := client.PauseSession(ctx, protoReq)
	if err != nil {
		return nil, fmt.Errorf("pause session request failed: %w", err)
	}

	return protoToReviewSession(resp.Session), nil
}

// ResumeSession resumes a paused session.
func (c *ReviewClient) ResumeSession(ctx context.Context, tenantID, sessionID string) (*ReviewSession, error) {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("review client not connected")
	}

	ctx = c.contextWithTenant(ctx, tenantID)

	protoReq := &reviewv1.ResumeSessionRequest{
		SessionId: sessionID,
	}
	if tenantID != "" {
		protoReq.TenantId = &tenantID
	}

	resp, err := client.ResumeSession(ctx, protoReq)
	if err != nil {
		return nil, fmt.Errorf("resume session request failed: %w", err)
	}

	return protoToReviewSession(resp.Session), nil
}

// EndSession ends the current session.
func (c *ReviewClient) EndSession(ctx context.Context, tenantID, sessionID string) (*ReviewSession, string, error) {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	if client == nil {
		return nil, "", fmt.Errorf("review client not connected")
	}

	ctx = c.contextWithTenant(ctx, tenantID)

	protoReq := &reviewv1.EndSessionRequest{
		SessionId: sessionID,
	}
	if tenantID != "" {
		protoReq.TenantId = &tenantID
	}

	resp, err := client.EndSession(ctx, protoReq)
	if err != nil {
		return nil, "", fmt.Errorf("end session request failed: %w", err)
	}

	return protoToReviewSession(resp.Session), resp.Summary, nil
}

// GetCurrentSession retrieves the current active or paused session.
func (c *ReviewClient) GetCurrentSession(ctx context.Context, tenantID string) (*ReviewSession, bool, error) {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	if client == nil {
		return nil, false, fmt.Errorf("review client not connected")
	}

	ctx = c.contextWithTenant(ctx, tenantID)

	protoReq := &reviewv1.GetCurrentSessionRequest{}
	if tenantID != "" {
		protoReq.TenantId = &tenantID
	}

	resp, err := client.GetCurrentSession(ctx, protoReq)
	if err != nil {
		return nil, false, fmt.Errorf("get current session request failed: %w", err)
	}

	return protoToReviewSession(resp.Session), resp.HasSession, nil
}

// GetSessionHistory retrieves past review sessions.
func (c *ReviewClient) GetSessionHistory(ctx context.Context, tenantID string, limit, offset int32) ([]*ReviewSession, int64, error) {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	if client == nil {
		return nil, 0, fmt.Errorf("review client not connected")
	}

	ctx = c.contextWithTenant(ctx, tenantID)

	protoReq := &reviewv1.GetSessionHistoryRequest{
		Limit:  limit,
		Offset: offset,
	}
	if tenantID != "" {
		protoReq.TenantId = &tenantID
	}

	resp, err := client.GetSessionHistory(ctx, protoReq)
	if err != nil {
		return nil, 0, fmt.Errorf("get session history request failed: %w", err)
	}

	sessions := make([]*ReviewSession, len(resp.Sessions))
	for i, s := range resp.Sessions {
		sessions[i] = protoToReviewSession(s)
	}

	return sessions, resp.TotalCount, nil
}

// Conversion helpers

func protoToReviewItem(item *reviewv1.ReviewItem) *ReviewItem {
	if item == nil {
		return nil
	}

	ri := &ReviewItem{
		ID:             item.Id,
		ContentType:    item.ContentType,
		ContentSummary: item.ContentSummary,
		Source:         item.Source,
		Priority:       item.Priority.String(),
		Status:         item.Status.String(),
		ContentID:      item.ContentId,
		Category:       item.Category,
		Metadata:       item.Metadata,
	}

	if item.CreatedAt != nil {
		ri.CreatedAt = item.CreatedAt.AsTime()
	}
	if item.UpdatedAt != nil {
		ri.UpdatedAt = item.UpdatedAt.AsTime()
	}

	return ri
}

func protoToReviewAction(action *reviewv1.ReviewAction) *ReviewAction {
	if action == nil {
		return nil
	}

	ra := &ReviewAction{
		ID:             action.Id,
		ItemID:         action.ItemId,
		ActionType:     action.ActionType.String(),
		PreviousStatus: action.PreviousStatus.String(),
		NewStatus:      action.NewStatus.String(),
		Reason:         action.Reason,
		Undoable:       action.Undoable,
	}

	if action.PerformedAt != nil {
		ra.PerformedAt = action.PerformedAt.AsTime()
	}

	return ra
}

func protoDailyReviewToDailyReview(dr *reviewv1.DailyReview) *DailyReview {
	if dr == nil {
		return nil
	}

	categories := make([]*ReviewCategory, len(dr.Categories))
	for i, cat := range dr.Categories {
		items := make([]*ReviewItem, len(cat.Items))
		for j, item := range cat.Items {
			items[j] = protoToReviewItem(item)
		}
		categories[i] = &ReviewCategory{
			Name:        cat.Name,
			DisplayName: cat.DisplayName,
			Items:       items,
			ItemCount:   cat.ItemCount,
		}
	}

	return &DailyReview{
		Date:              dr.Date,
		Categories:        categories,
		TotalPending:      dr.TotalPending,
		HighPriorityCount: dr.HighPriorityCount,
		ProcessedToday:    dr.ProcessedToday,
	}
}

func protoToReviewSession(session *reviewv1.ReviewSession) *ReviewSession {
	if session == nil {
		return nil
	}

	rs := &ReviewSession{
		ID:                    session.Id,
		Status:                session.Status.String(),
		TotalReviewed:         session.TotalReviewed,
		ApprovedCount:         session.ApprovedCount,
		RejectedCount:         session.RejectedCount,
		DeferredCount:         session.DeferredCount,
		ActiveDurationSeconds: session.ActiveDurationSeconds,
	}

	if session.StartedAt != nil {
		rs.StartedAt = session.StartedAt.AsTime()
	}
	if session.PausedAt != nil {
		t := session.PausedAt.AsTime()
		rs.PausedAt = &t
	}
	if session.EndedAt != nil {
		t := session.EndedAt.AsTime()
		rs.EndedAt = &t
	}

	return rs
}

func stringToReviewStatus(s string) reviewv1.ReviewStatus {
	switch s {
	case "pending", "PENDING", "REVIEW_STATUS_PENDING":
		return reviewv1.ReviewStatus_REVIEW_STATUS_PENDING
	case "approved", "APPROVED", "REVIEW_STATUS_APPROVED":
		return reviewv1.ReviewStatus_REVIEW_STATUS_APPROVED
	case "rejected", "REJECTED", "REVIEW_STATUS_REJECTED":
		return reviewv1.ReviewStatus_REVIEW_STATUS_REJECTED
	case "deferred", "DEFERRED", "REVIEW_STATUS_DEFERRED":
		return reviewv1.ReviewStatus_REVIEW_STATUS_DEFERRED
	default:
		return reviewv1.ReviewStatus_REVIEW_STATUS_UNSPECIFIED
	}
}

func stringToPriority(s string) reviewv1.Priority {
	switch s {
	case "low", "LOW", "PRIORITY_LOW":
		return reviewv1.Priority_PRIORITY_LOW
	case "medium", "MEDIUM", "PRIORITY_MEDIUM":
		return reviewv1.Priority_PRIORITY_MEDIUM
	case "high", "HIGH", "PRIORITY_HIGH":
		return reviewv1.Priority_PRIORITY_HIGH
	case "urgent", "URGENT", "PRIORITY_URGENT":
		return reviewv1.Priority_PRIORITY_URGENT
	default:
		return reviewv1.Priority_PRIORITY_UNSPECIFIED
	}
}
