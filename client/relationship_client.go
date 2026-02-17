// Package client provides gRPC clients for Penfold services.
package client

import (
	"context"
	"fmt"
	"sync"
	"time"

	relationshipv1 "github.com/otherjamesbrown/penf-cli/api/proto/relationship/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
)

// RelationshipClient manages the connection to the Penfold Relationship service.
type RelationshipClient struct {
	conn       *grpc.ClientConn
	client     relationshipv1.RelationshipServiceClient
	serverAddr string
	options    *ClientOptions
	mu         sync.RWMutex
	connected  bool
}

// NewRelationshipClient creates a new RelationshipClient with the given options.
// Call Connect() to establish the connection.
func NewRelationshipClient(serverAddr string, opts *ClientOptions) *RelationshipClient {
	if opts == nil {
		opts = DefaultOptions()
	}

	return &RelationshipClient{
		serverAddr: serverAddr,
		options:    opts,
	}
}

// Connect establishes a connection to the Relationship service.
func (c *RelationshipClient) Connect(ctx context.Context) error {
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
		return fmt.Errorf("connecting to relationship service at %s: %w", c.serverAddr, err)
	}

	c.conn = conn
	c.client = relationshipv1.NewRelationshipServiceClient(conn)
	c.connected = true

	return nil
}

// buildDialOptions constructs the gRPC dial options from client configuration.
func (c *RelationshipClient) buildDialOptions() []grpc.DialOption {
	opts := []grpc.DialOption{
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second,
			Timeout:             3 * time.Second,
			PermitWithoutStream: true,
		}),
		// Block on dial to detect connection failures early.
		grpc.WithBlock(),
	}

	if c.options.Insecure {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else if c.options.TLSConfig != nil {
		creds := credentials.NewTLS(c.options.TLSConfig)
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	return opts
}

// Close closes the connection to the Relationship service.
func (c *RelationshipClient) Close() error {
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
		return fmt.Errorf("closing relationship connection: %w", err)
	}

	return nil
}

// IsConnected returns true if the client has an active connection.
func (c *RelationshipClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected && c.conn != nil
}

// contextWithTenant returns a context with the tenant ID in metadata.
func (c *RelationshipClient) contextWithTenant(ctx context.Context, tenantID string) context.Context {
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

// RelEntity represents an entity in the relationship graph.
type RelEntity struct {
	ID            string
	Name          string
	Type          string
	CanonicalName string
	Aliases       []string
	Confidence    float32
	SourceCount   int32
	RelationCount int32
	SentCount     int32
	ReceivedCount int32
	FirstSeen     time.Time
	LastSeen      time.Time
	Metadata      map[string]string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Relationship represents a relationship between entities.
type Relationship struct {
	ID               string
	SourceEntity     *RelEntity
	TargetEntity     *RelEntity
	RelationshipType string
	Confidence       float32
	Status           string
	Evidence         []Evidence
	Notes            string
	TenantID         string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// Evidence represents supporting evidence for a relationship.
type Evidence struct {
	SourceID               string
	SourceType             string
	Excerpt                string
	DiscoveredAt           time.Time
	ConfidenceContribution float32
}

// NetworkStats contains statistics about the relationship network.
type NetworkStats struct {
	TotalNodes             int32
	TotalEdges             int32
	Density                float32
	AvgConnections         float32
	ClusterCount           int32
	EntityTypeCounts       map[string]int32
	RelationshipTypeCounts map[string]int32
}

// NetworkCluster represents a cluster in the relationship network.
type NetworkCluster struct {
	ID          string
	Name        string
	EntityCount int32
	TopEntities []*RelEntity
	Density     float32
}

// RelationshipConflict represents a detected conflict in the relationship graph.
type RelationshipConflict struct {
	ID              string
	TenantID        string
	Type            string
	Description     string
	SuggestedAction string
	Status          string
	Resolution      string
	ResolvedBy      string
	ResolvedAt      *time.Time
	CreatedAt       time.Time
	Relationships   []*Relationship
}

// ListRelationshipsRequest represents a request to list relationships.
type ListRelationshipsRequest struct {
	TenantID         string
	EntityID         string
	RelationshipType relationshipv1.RelationshipType
	Status           relationshipv1.RelationshipStatus
	MinConfidence    float32
	SourceEntityType relationshipv1.EntityType
	TargetEntityType relationshipv1.EntityType
	PageSize         int32
}

// ListRelationships returns a list of relationships matching the filter.
func (c *RelationshipClient) ListRelationships(ctx context.Context, req *ListRelationshipsRequest) ([]*Relationship, int64, error) {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	if client == nil {
		return nil, 0, fmt.Errorf("relationship client not connected")
	}

	ctx = c.contextWithTenant(ctx, req.TenantID)

	protoReq := &relationshipv1.ListRelationshipsRequest{
		TenantId: req.TenantID,
		PageSize: req.PageSize,
	}

	if req.EntityID != "" {
		protoReq.EntityId = &req.EntityID
	}

	if req.RelationshipType != relationshipv1.RelationshipType_RELATIONSHIP_TYPE_UNSPECIFIED {
		protoReq.RelationshipType = &req.RelationshipType
	}

	if req.Status != relationshipv1.RelationshipStatus_RELATIONSHIP_STATUS_UNSPECIFIED {
		protoReq.Status = &req.Status
	}

	if req.MinConfidence > 0 {
		protoReq.MinConfidence = &req.MinConfidence
	}

	if req.SourceEntityType != relationshipv1.EntityType_ENTITY_TYPE_UNSPECIFIED {
		protoReq.SourceEntityType = &req.SourceEntityType
	}

	if req.TargetEntityType != relationshipv1.EntityType_ENTITY_TYPE_UNSPECIFIED {
		protoReq.TargetEntityType = &req.TargetEntityType
	}

	resp, err := client.ListRelationships(ctx, protoReq)
	if err != nil {
		return nil, 0, fmt.Errorf("list relationships request failed: %w", err)
	}

	relationships := make([]*Relationship, len(resp.Relationships))
	for i, r := range resp.Relationships {
		relationships[i] = protoToRelationship(r)
	}

	var totalCount int64
	if resp.TotalCount != nil {
		totalCount = *resp.TotalCount
	}

	return relationships, totalCount, nil
}

// GetRelationship retrieves a single relationship by ID.
func (c *RelationshipClient) GetRelationship(ctx context.Context, tenantID, relationshipID string) (*Relationship, error) {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("relationship client not connected")
	}

	ctx = c.contextWithTenant(ctx, tenantID)

	resp, err := client.GetRelationship(ctx, &relationshipv1.GetRelationshipRequest{
		TenantId:       tenantID,
		RelationshipId: relationshipID,
	})
	if err != nil {
		return nil, fmt.Errorf("get relationship request failed: %w", err)
	}

	return protoToRelationship(resp), nil
}

// SearchRelationships searches relationships by entity name.
func (c *RelationshipClient) SearchRelationships(ctx context.Context, tenantID, query string, limit int32) ([]*Relationship, error) {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("relationship client not connected")
	}

	ctx = c.contextWithTenant(ctx, tenantID)

	resp, err := client.SearchRelationships(ctx, &relationshipv1.SearchRelationshipsRequest{
		TenantId: tenantID,
		Query:    query,
		Limit:    limit,
	})
	if err != nil {
		return nil, fmt.Errorf("search relationships request failed: %w", err)
	}

	relationships := make([]*Relationship, len(resp.Relationships))
	for i, r := range resp.Relationships {
		relationships[i] = protoToRelationship(r)
	}

	return relationships, nil
}

// ListEntitiesRequest represents a request to list entities.
type ListEntitiesRequest struct {
	TenantID      string
	EntityType    relationshipv1.EntityType
	Search        string
	MinConfidence float32
	PageSize      int32
	Offset        int32
}

// ListEntities returns a list of entities matching the filter.
func (c *RelationshipClient) ListEntities(ctx context.Context, req *ListEntitiesRequest) ([]*RelEntity, int64, error) {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	if client == nil {
		return nil, 0, fmt.Errorf("relationship client not connected")
	}

	ctx = c.contextWithTenant(ctx, req.TenantID)

	protoReq := &relationshipv1.ListEntitiesRequest{
		TenantId: req.TenantID,
		PageSize: req.PageSize,
		Offset:   req.Offset,
	}

	if req.EntityType != relationshipv1.EntityType_ENTITY_TYPE_UNSPECIFIED {
		protoReq.EntityType = &req.EntityType
	}

	if req.Search != "" {
		protoReq.Search = &req.Search
	}

	if req.MinConfidence > 0 {
		protoReq.MinConfidence = &req.MinConfidence
	}

	resp, err := client.ListEntities(ctx, protoReq)
	if err != nil {
		return nil, 0, fmt.Errorf("list entities request failed: %w", err)
	}

	entities := make([]*RelEntity, len(resp.Entities))
	for i, e := range resp.Entities {
		entities[i] = protoToEntity(e)
	}

	return entities, resp.TotalCount, nil
}

// GetEntity retrieves a single entity by ID.
func (c *RelationshipClient) GetEntity(ctx context.Context, tenantID, entityID string) (*RelEntity, error) {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("relationship client not connected")
	}

	ctx = c.contextWithTenant(ctx, tenantID)

	resp, err := client.GetEntity(ctx, &relationshipv1.GetEntityRequest{
		TenantId: tenantID,
		EntityId: entityID,
	})
	if err != nil {
		return nil, fmt.Errorf("get entity request failed: %w", err)
	}

	return protoToEntity(resp), nil
}

// MergeEntities merges two entities into one.
func (c *RelationshipClient) MergeEntities(ctx context.Context, tenantID, primaryEntityID, mergedEntityID string) (*RelEntity, int32, error) {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	if client == nil {
		return nil, 0, fmt.Errorf("relationship client not connected")
	}

	ctx = c.contextWithTenant(ctx, tenantID)

	resp, err := client.MergeEntities(ctx, &relationshipv1.MergeEntitiesRequest{
		TenantId:        tenantID,
		PrimaryEntityId: primaryEntityID,
		MergedEntityId:  mergedEntityID,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("merge entities request failed: %w", err)
	}

	return protoToEntity(resp.PrimaryEntity), resp.RelationshipsTransferred, nil
}

// CreateRelationship manually creates a new relationship between two entities.
func (c *RelationshipClient) CreateRelationship(ctx context.Context, tenantID, fromEntityID, toEntityID string, relType relationshipv1.RelationshipType, subtype string) (*Relationship, error) {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("relationship client not connected")
	}

	ctx = c.contextWithTenant(ctx, tenantID)

	req := &relationshipv1.CreateRelationshipRequest{
		TenantId:     tenantID,
		FromEntityId: fromEntityID,
		ToEntityId:   toEntityID,
		Type:         relType,
	}

	if subtype != "" {
		req.Subtype = &subtype
	}

	resp, err := client.CreateRelationship(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("create relationship request failed: %w", err)
	}

	return protoToRelationship(resp.Relationship), nil
}

// GetNetworkStats retrieves statistics about the relationship network.
func (c *RelationshipClient) GetNetworkStats(ctx context.Context, tenantID string) (*NetworkStats, error) {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("relationship client not connected")
	}

	ctx = c.contextWithTenant(ctx, tenantID)

	resp, err := client.GetNetworkStats(ctx, &relationshipv1.GetNetworkStatsRequest{
		TenantId: tenantID,
	})
	if err != nil {
		return nil, fmt.Errorf("get network stats request failed: %w", err)
	}

	return &NetworkStats{
		TotalNodes:             resp.TotalNodes,
		TotalEdges:             resp.TotalEdges,
		Density:                resp.Density,
		AvgConnections:         resp.AvgConnections,
		ClusterCount:           resp.ClusterCount,
		EntityTypeCounts:       resp.EntityTypeCounts,
		RelationshipTypeCounts: resp.RelationshipTypeCounts,
	}, nil
}

// GetCentralEntities retrieves the most connected entities in the network.
func (c *RelationshipClient) GetCentralEntities(ctx context.Context, tenantID string, limit int32) ([]*RelEntity, error) {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("relationship client not connected")
	}

	ctx = c.contextWithTenant(ctx, tenantID)

	resp, err := client.GetCentralEntities(ctx, &relationshipv1.GetCentralEntitiesRequest{
		TenantId: tenantID,
		Limit:    limit,
	})
	if err != nil {
		return nil, fmt.Errorf("get central entities request failed: %w", err)
	}

	entities := make([]*RelEntity, len(resp.Entities))
	for i, e := range resp.Entities {
		entities[i] = protoToEntity(e)
	}

	return entities, nil
}

// GetClusters retrieves clusters in the relationship network.
func (c *RelationshipClient) GetClusters(ctx context.Context, tenantID string) ([]*NetworkCluster, error) {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("relationship client not connected")
	}

	ctx = c.contextWithTenant(ctx, tenantID)

	resp, err := client.GetClusters(ctx, &relationshipv1.GetClustersRequest{
		TenantId: tenantID,
	})
	if err != nil {
		return nil, fmt.Errorf("get clusters request failed: %w", err)
	}

	clusters := make([]*NetworkCluster, len(resp.Clusters))
	for i, c := range resp.Clusters {
		topEntities := make([]*RelEntity, len(c.TopEntities))
		for j, e := range c.TopEntities {
			topEntities[j] = protoToEntity(e)
		}

		clusters[i] = &NetworkCluster{
			ID:          c.Id,
			Name:        c.Name,
			EntityCount: c.EntityCount,
			TopEntities: topEntities,
			Density:     c.Density,
		}
	}

	return clusters, nil
}

// ListConflictsRequest represents a request to list conflicts.
type ListConflictsRequest struct {
	TenantID string
	Status   relationshipv1.ConflictStatus
	Limit    int32
	Offset   int32
}

// ListConflicts returns a list of conflicts matching the filter.
func (c *RelationshipClient) ListConflicts(ctx context.Context, req *ListConflictsRequest) ([]*RelationshipConflict, int64, error) {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	if client == nil {
		return nil, 0, fmt.Errorf("relationship client not connected")
	}

	ctx = c.contextWithTenant(ctx, req.TenantID)

	protoReq := &relationshipv1.ListConflictsRequest{
		TenantId: req.TenantID,
		Limit:    req.Limit,
		Offset:   req.Offset,
	}

	if req.Status != relationshipv1.ConflictStatus_CONFLICT_STATUS_UNSPECIFIED {
		protoReq.Status = &req.Status
	}

	resp, err := client.ListConflicts(ctx, protoReq)
	if err != nil {
		return nil, 0, fmt.Errorf("list conflicts request failed: %w", err)
	}

	conflicts := make([]*RelationshipConflict, len(resp.Conflicts))
	for i, c := range resp.Conflicts {
		conflicts[i] = protoToConflict(c)
	}

	return conflicts, resp.TotalCount, nil
}

// GetConflict retrieves a single conflict by ID.
func (c *RelationshipClient) GetConflict(ctx context.Context, tenantID, conflictID string) (*RelationshipConflict, error) {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("relationship client not connected")
	}

	ctx = c.contextWithTenant(ctx, tenantID)

	resp, err := client.GetConflict(ctx, &relationshipv1.GetConflictRequest{
		TenantId:   tenantID,
		ConflictId: conflictID,
	})
	if err != nil {
		return nil, fmt.Errorf("get conflict request failed: %w", err)
	}

	return protoToConflict(resp), nil
}

// ResolveConflictRequest represents a request to resolve a conflict.
type ResolveConflictRequest struct {
	TenantID   string
	ConflictID string
	Strategy   relationshipv1.ConflictResolutionStrategy
	ResolvedBy string
	Notes      string
}

// ResolveConflict resolves a relationship conflict.
func (c *RelationshipClient) ResolveConflict(ctx context.Context, req *ResolveConflictRequest) (*RelationshipConflict, int32, error) {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	if client == nil {
		return nil, 0, fmt.Errorf("relationship client not connected")
	}

	ctx = c.contextWithTenant(ctx, req.TenantID)

	protoReq := &relationshipv1.ResolveConflictRequest{
		TenantId:   req.TenantID,
		ConflictId: req.ConflictID,
		Strategy:   req.Strategy,
	}

	if req.ResolvedBy != "" {
		protoReq.ResolvedBy = &req.ResolvedBy
	}

	if req.Notes != "" {
		protoReq.Notes = &req.Notes
	}

	resp, err := client.ResolveConflict(ctx, protoReq)
	if err != nil {
		return nil, 0, fmt.Errorf("resolve conflict request failed: %w", err)
	}

	return protoToConflict(resp.Conflict), resp.RelationshipsUpdated, nil
}

// NetworkGraph represents the relationship network for visualization.
type NetworkGraph struct {
	Nodes    []*GraphNode
	Edges    []*GraphEdge
	Metadata *GraphMetadata
}

// GraphNode represents an entity in the network graph.
type GraphNode struct {
	ID         string
	Label      string
	Type       string
	Degree     int32
	Properties map[string]string
}

// GraphEdge represents a relationship in the network graph.
type GraphEdge struct {
	ID               string
	Source           string
	Target           string
	RelationshipType string
	Weight           float32
	Label            string
	Properties       map[string]string
}

// GraphMetadata provides information about the network graph.
type GraphMetadata struct {
	TotalNodes     int32
	TotalEdges     int32
	Truncated      bool
	CenterEntityID string
	Depth          int32
}

// DiscoveryResult contains discovered relationships from content analysis.
type DiscoveryResult struct {
	Relationships   []*Relationship
	TotalDiscovered int32
	Metadata        *DiscoveryMetadata
}

// DiscoveryMetadata provides information about the discovery process.
type DiscoveryMetadata struct {
	ProcessingTimeMs int64
	ModelName        string
	EntitiesAnalyzed int32
	JobID            string
}

// ValidateRelationshipRequest represents a request to validate a relationship.
type ValidateRelationshipRequest struct {
	TenantID       string
	RelationshipID string
	Action         relationshipv1.ValidationAction
	Notes          string
}

// ValidateRelationshipResult contains the result of a validation operation.
type ValidateRelationshipResult struct {
	Relationship *Relationship
	Success      bool
	Message      string
}

// GetNetworkGraphOptions configures network graph retrieval.
type GetNetworkGraphOptions struct {
	CenterEntityID string
	Depth          int32
	MaxNodes       int32
	ConfirmedOnly  bool
	MinConfidence  float32
}

// GetNetworkGraph retrieves the relationship network graph for visualization.
func (c *RelationshipClient) GetNetworkGraph(ctx context.Context, tenantID string, opts *GetNetworkGraphOptions) (*NetworkGraph, error) {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("relationship client not connected")
	}

	ctx = c.contextWithTenant(ctx, tenantID)

	req := &relationshipv1.GetNetworkGraphRequest{
		TenantId: tenantID,
	}

	if opts != nil {
		if opts.CenterEntityID != "" {
			req.CenterEntityId = &opts.CenterEntityID
		}
		if opts.Depth > 0 {
			req.Depth = opts.Depth
		}
		if opts.MaxNodes > 0 {
			req.MaxNodes = opts.MaxNodes
		}
		if opts.ConfirmedOnly {
			req.ConfirmedOnly = opts.ConfirmedOnly
		}
		if opts.MinConfidence > 0 {
			req.MinConfidence = &opts.MinConfidence
		}
	}

	resp, err := client.GetNetworkGraph(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("get network graph request failed: %w", err)
	}

	return protoToNetworkGraph(resp), nil
}

// DiscoverOptions configures relationship discovery.
type DiscoverOptions struct {
	MinConfidence    float32
	MaxRelationships int32
	IncludeExisting  bool
}

// DiscoverRelationships analyzes content to find relationships between entities.
func (c *RelationshipClient) DiscoverRelationships(ctx context.Context, tenantID, contentID string, opts *DiscoverOptions) (*DiscoveryResult, error) {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("relationship client not connected")
	}

	ctx = c.contextWithTenant(ctx, tenantID)

	req := &relationshipv1.DiscoverRelationshipsRequest{
		TenantId:  tenantID,
		ContentId: contentID,
	}

	if opts != nil {
		req.DiscoveryOptions = &relationshipv1.DiscoveryOptions{
			MinConfidence:    opts.MinConfidence,
			MaxRelationships: opts.MaxRelationships,
			IncludeExisting:  opts.IncludeExisting,
		}
	}

	resp, err := client.DiscoverRelationships(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("discover relationships request failed: %w", err)
	}

	relationships := make([]*Relationship, len(resp.Relationships))
	for i, r := range resp.Relationships {
		relationships[i] = protoToRelationship(r)
	}

	var metadata *DiscoveryMetadata
	if resp.Metadata != nil {
		metadata = &DiscoveryMetadata{
			ProcessingTimeMs: resp.Metadata.ProcessingTimeMs,
			ModelName:        resp.Metadata.ModelName,
			EntitiesAnalyzed: resp.Metadata.EntitiesAnalyzed,
			JobID:            resp.Metadata.JobId,
		}
	}

	return &DiscoveryResult{
		Relationships:   relationships,
		TotalDiscovered: resp.TotalDiscovered,
		Metadata:        metadata,
	}, nil
}

// ValidateRelationship allows users to confirm or reject a discovered relationship.
func (c *RelationshipClient) ValidateRelationship(ctx context.Context, req *ValidateRelationshipRequest) (*ValidateRelationshipResult, error) {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("relationship client not connected")
	}

	ctx = c.contextWithTenant(ctx, req.TenantID)

	protoReq := &relationshipv1.ValidateRelationshipRequest{
		TenantId:       req.TenantID,
		RelationshipId: req.RelationshipID,
		Action:         req.Action,
	}

	if req.Notes != "" {
		protoReq.Notes = &req.Notes
	}

	resp, err := client.ValidateRelationship(ctx, protoReq)
	if err != nil {
		return nil, fmt.Errorf("validate relationship request failed: %w", err)
	}

	return &ValidateRelationshipResult{
		Relationship: protoToRelationship(resp.Relationship),
		Success:      resp.Success,
		Message:      resp.Message,
	}, nil
}

// Conversion helpers

func protoToEntity(e *relationshipv1.Entity) *RelEntity {
	if e == nil {
		return nil
	}

	entity := &RelEntity{
		ID:            e.Id,
		Name:          e.Name,
		Type:          e.Type.String(),
		Aliases:       e.Aliases,
		Confidence:    e.Confidence,
		SourceCount:   e.SourceCount,
		RelationCount: e.RelationCount,
		SentCount:     e.SentCount,
		ReceivedCount: e.ReceivedCount,
		Metadata:      e.Metadata,
	}

	if e.CanonicalName != nil {
		entity.CanonicalName = *e.CanonicalName
	}

	if e.CreatedAt != nil {
		entity.CreatedAt = e.CreatedAt.AsTime()
	}

	if e.UpdatedAt != nil {
		entity.UpdatedAt = e.UpdatedAt.AsTime()
	}

	if e.FirstSeen != nil {
		entity.FirstSeen = e.FirstSeen.AsTime()
	}

	if e.LastSeen != nil {
		entity.LastSeen = e.LastSeen.AsTime()
	}

	return entity
}

func protoToRelationship(r *relationshipv1.Relationship) *Relationship {
	if r == nil {
		return nil
	}

	rel := &Relationship{
		ID:               r.Id,
		SourceEntity:     protoToEntity(r.SourceEntity),
		TargetEntity:     protoToEntity(r.TargetEntity),
		RelationshipType: r.RelationshipType.String(),
		Confidence:       r.Confidence,
		Status:           r.Status.String(),
		TenantID:         r.TenantId,
	}

	if r.Notes != nil {
		rel.Notes = *r.Notes
	}

	if r.CreatedAt != nil {
		rel.CreatedAt = r.CreatedAt.AsTime()
	}

	if r.UpdatedAt != nil {
		rel.UpdatedAt = r.UpdatedAt.AsTime()
	}

	rel.Evidence = make([]Evidence, len(r.Evidence))
	for i, e := range r.Evidence {
		ev := Evidence{
			SourceID:               e.SourceId,
			SourceType:             e.SourceType,
			Excerpt:                e.Excerpt,
			ConfidenceContribution: e.ConfidenceContribution,
		}
		if e.DiscoveredAt != nil {
			ev.DiscoveredAt = e.DiscoveredAt.AsTime()
		}
		rel.Evidence[i] = ev
	}

	return rel
}

func protoToConflict(c *relationshipv1.Conflict) *RelationshipConflict {
	if c == nil {
		return nil
	}

	conflict := &RelationshipConflict{
		ID:              c.Id,
		TenantID:        c.TenantId,
		Type:            c.Type,
		Description:     c.Description,
		SuggestedAction: c.SuggestedAction,
		Status:          c.Status.String(),
	}

	if c.Resolution != nil {
		conflict.Resolution = *c.Resolution
	}

	if c.ResolvedBy != nil {
		conflict.ResolvedBy = *c.ResolvedBy
	}

	if c.ResolvedAt != nil {
		t := c.ResolvedAt.AsTime()
		conflict.ResolvedAt = &t
	}

	if c.CreatedAt != nil {
		conflict.CreatedAt = c.CreatedAt.AsTime()
	}

	conflict.Relationships = make([]*Relationship, len(c.Relationships))
	for i, r := range c.Relationships {
		conflict.Relationships[i] = protoToRelationship(r)
	}

	return conflict
}

func protoToNetworkGraph(g *relationshipv1.NetworkGraph) *NetworkGraph {
	if g == nil {
		return nil
	}

	nodes := make([]*GraphNode, len(g.Nodes))
	for i, n := range g.Nodes {
		nodes[i] = &GraphNode{
			ID:         n.Id,
			Label:      n.Label,
			Type:       n.Type.String(),
			Degree:     n.Degree,
			Properties: n.Properties,
		}
	}

	edges := make([]*GraphEdge, len(g.Edges))
	for i, e := range g.Edges {
		edges[i] = &GraphEdge{
			ID:               e.Id,
			Source:           e.Source,
			Target:           e.Target,
			RelationshipType: e.RelationshipType.String(),
			Weight:           e.Weight,
			Label:            e.Label,
			Properties:       e.Properties,
		}
	}

	var metadata *GraphMetadata
	if g.Metadata != nil {
		centerID := ""
		if g.Metadata.CenterEntityId != nil {
			centerID = *g.Metadata.CenterEntityId
		}
		metadata = &GraphMetadata{
			TotalNodes:     g.Metadata.TotalNodes,
			TotalEdges:     g.Metadata.TotalEdges,
			Truncated:      g.Metadata.Truncated,
			CenterEntityID: centerID,
			Depth:          g.Metadata.Depth,
		}
	}

	return &NetworkGraph{
		Nodes:    nodes,
		Edges:    edges,
		Metadata: metadata,
	}
}

// DuplicatePair represents a pair of potentially duplicate entities.
type DuplicatePair struct {
	EntityID1   string
	EntityID2   string
	EntityName1 string
	EntityName2 string
	Similarity  float32
	Signals     []string
}

// MergePreview contains a preview of merging two entities.
type MergePreview struct {
	MergedEntity              *RelEntity
	TransferringAliases       []string
	TransferringRelationships []string
	ConflictFields            []string
}

// AutoMergeResult contains results of auto-merge operation.
type AutoMergeResult struct {
	MergedCount  int32
	MergedPairs  []*DuplicatePair
	SkippedPairs []*SkippedPair
	WasDryRun    bool
}

// SkippedPair represents a duplicate pair that was skipped.
type SkippedPair struct {
	Pair   *DuplicatePair
	Reason string
}

// FindDuplicates finds pairs of entities that are likely duplicates.
func (c *RelationshipClient) FindDuplicates(ctx context.Context, tenantID string, minSimilarity float32) ([]*DuplicatePair, error) {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("relationship client not connected")
	}

	ctx = c.contextWithTenant(ctx, tenantID)

	req := &relationshipv1.FindDuplicatesRequest{
		TenantId: tenantID,
	}

	if minSimilarity > 0 {
		req.MinSimilarity = &minSimilarity
	}

	resp, err := client.FindDuplicates(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("find duplicates: %w", err)
	}

	pairs := make([]*DuplicatePair, len(resp.DuplicatePairs))
	for i, p := range resp.DuplicatePairs {
		pairs[i] = &DuplicatePair{
			EntityID1:   p.EntityId1,
			EntityID2:   p.EntityId2,
			EntityName1: p.EntityName1,
			EntityName2: p.EntityName2,
			Similarity:  p.Similarity,
			Signals:     p.Signals,
		}
	}

	return pairs, nil
}

// MergePreview shows what would happen if two entities were merged.
func (c *RelationshipClient) MergePreview(ctx context.Context, tenantID, entityID1, entityID2 string) (*MergePreview, error) {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("relationship client not connected")
	}

	ctx = c.contextWithTenant(ctx, tenantID)

	resp, err := client.MergePreview(ctx, &relationshipv1.MergePreviewRequest{
		TenantId:  tenantID,
		EntityId1: entityID1,
		EntityId2: entityID2,
	})
	if err != nil {
		return nil, fmt.Errorf("merge preview: %w", err)
	}

	return &MergePreview{
		MergedEntity:              protoToEntity(resp.MergedEntity),
		TransferringAliases:       resp.TransferringAliases,
		TransferringRelationships: resp.TransferringRelationships,
		ConflictFields:            resp.ConflictFields,
	}, nil
}

// AutoMergeDuplicates automatically merges high-confidence duplicate entities.
func (c *RelationshipClient) AutoMergeDuplicates(ctx context.Context, tenantID string, minSimilarity float32, dryRun bool) (*AutoMergeResult, error) {
	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("relationship client not connected")
	}

	ctx = c.contextWithTenant(ctx, tenantID)

	req := &relationshipv1.AutoMergeDuplicatesRequest{
		TenantId: tenantID,
		DryRun:   dryRun,
	}

	if minSimilarity > 0 {
		req.MinSimilarity = &minSimilarity
	}

	resp, err := client.AutoMergeDuplicates(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("auto merge duplicates: %w", err)
	}

	mergedPairs := make([]*DuplicatePair, len(resp.MergedPairs))
	for i, p := range resp.MergedPairs {
		mergedPairs[i] = &DuplicatePair{
			EntityID1:   p.EntityId1,
			EntityID2:   p.EntityId2,
			EntityName1: p.EntityName1,
			EntityName2: p.EntityName2,
			Similarity:  p.Similarity,
			Signals:     p.Signals,
		}
	}

	skippedPairs := make([]*SkippedPair, len(resp.SkippedPairs))
	for i, sp := range resp.SkippedPairs {
		skippedPairs[i] = &SkippedPair{
			Pair: &DuplicatePair{
				EntityID1:   sp.Pair.EntityId1,
				EntityID2:   sp.Pair.EntityId2,
				EntityName1: sp.Pair.EntityName1,
				EntityName2: sp.Pair.EntityName2,
				Similarity:  sp.Pair.Similarity,
				Signals:     sp.Pair.Signals,
			},
			Reason: sp.Reason,
		}
	}

	return &AutoMergeResult{
		MergedCount:  resp.MergedCount,
		MergedPairs:  mergedPairs,
		SkippedPairs: skippedPairs,
		WasDryRun:    resp.WasDryRun,
	}, nil
}
