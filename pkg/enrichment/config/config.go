// Package config provides tenant configuration management for the enrichment pipeline.
package config

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// TenantConfig holds the resolved configuration for a tenant.
type TenantConfig struct {
	TenantID             string
	TenantSlug           string
	InternalDomains      []string
	BotPatterns          []string
	DistributionPatterns []string
	RoleAccountPatterns  []string
	IgnorePatterns       []string
	ProcessingRules      []ProcessingRule
	Integrations         map[string]Integration
	cachedAt             time.Time
}

// ProcessingRule defines a custom classification rule.
type ProcessingRule struct {
	ID                    int64
	Name                  string
	Priority              int
	MatchConditions       MatchConditions
	ClassificationOverride ClassificationOverride
	Enabled               bool
}

// MatchConditions defines conditions for a processing rule.
type MatchConditions struct {
	FromContains     string            `json:"from_contains,omitempty"`
	FromMatches      string            `json:"from_matches,omitempty"`
	ToContains       string            `json:"to_contains,omitempty"`
	SubjectContains  string            `json:"subject_contains,omitempty"`
	SubjectStartsWith string           `json:"subject_starts_with,omitempty"`
	HasHeader        string            `json:"has_header,omitempty"`
	HeaderContains   map[string]string `json:"header_contains,omitempty"`
}

// ClassificationOverride specifies the classification to apply when a rule matches.
type ClassificationOverride struct {
	ContentSubtype    string `json:"content_subtype,omitempty"`
	ProcessingProfile string `json:"processing_profile,omitempty"`
}

// Integration represents an external integration configuration.
type Integration struct {
	ID             int64
	Type           string // jira, google_workspace, slack, etc.
	Name           string
	InstanceURL    string
	Config         map[string]interface{}
	CredentialsKey string
	Enabled        bool
	SyncStatus     string
	LastSyncAt     *time.Time
}

// ConfigRepository provides data access for tenant configuration.
type ConfigRepository interface {
	// GetTenant retrieves a tenant by ID.
	GetTenant(ctx context.Context, tenantID string) (*Tenant, error)
	// GetTenantBySlug retrieves a tenant by slug.
	GetTenantBySlug(ctx context.Context, slug string) (*Tenant, error)
	// GetTenantDomains retrieves domains for a tenant.
	GetTenantDomains(ctx context.Context, tenantID string) ([]TenantDomain, error)
	// GetTenantEmailPatterns retrieves email patterns for a tenant.
	GetTenantEmailPatterns(ctx context.Context, tenantID string) ([]TenantEmailPattern, error)
	// GetTenantProcessingRules retrieves processing rules for a tenant.
	GetTenantProcessingRules(ctx context.Context, tenantID string) ([]TenantProcessingRule, error)
	// GetTenantIntegrations retrieves integrations for a tenant.
	GetTenantIntegrations(ctx context.Context, tenantID string) ([]TenantIntegration, error)
}

// Tenant represents a tenant record.
type Tenant struct {
	ID        string
	Name      string
	Slug      string
	IsActive  bool
	CreatedAt time.Time
}

// TenantDomain represents a domain record.
type TenantDomain struct {
	ID         int64
	TenantID   string
	Domain     string
	DomainType string // internal, external_known, external_unknown
	Notes      string
}

// TenantEmailPattern represents an email pattern record.
type TenantEmailPattern struct {
	ID          int64
	TenantID    string
	Pattern     string
	PatternType string // bot, distribution_list, role_account, ignore
	Priority    int
	Enabled     bool
}

// TenantProcessingRule represents a processing rule record.
type TenantProcessingRule struct {
	ID                     int64
	TenantID               string
	RuleName               string
	Priority               int
	MatchConditions        MatchConditions
	ClassificationOverride ClassificationOverride
	Enabled                bool
}

// TenantIntegration represents an integration record.
type TenantIntegration struct {
	ID             int64
	TenantID       string
	IntegrationType string
	Name           string
	InstanceURL    string
	Config         map[string]interface{}
	CredentialsKey string
	Enabled        bool
	SyncStatus     string
	LastSyncAt     *time.Time
}

// ConfigResolver resolves and caches tenant configuration.
type ConfigResolver struct {
	repo     ConfigRepository
	cache    map[string]*TenantConfig
	cacheTTL time.Duration
	mu       sync.RWMutex
}

// NewConfigResolver creates a new config resolver.
func NewConfigResolver(repo ConfigRepository, opts ...ConfigResolverOption) *ConfigResolver {
	r := &ConfigResolver{
		repo:     repo,
		cache:    make(map[string]*TenantConfig),
		cacheTTL: 5 * time.Minute,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// ConfigResolverOption configures the config resolver.
type ConfigResolverOption func(*ConfigResolver)

// WithCacheTTL sets the cache TTL.
func WithCacheTTL(ttl time.Duration) ConfigResolverOption {
	return func(r *ConfigResolver) {
		r.cacheTTL = ttl
	}
}

// GetConfig retrieves the configuration for a tenant (cached).
func (r *ConfigResolver) GetConfig(ctx context.Context, tenantID string) (*TenantConfig, error) {
	// Check cache first
	r.mu.RLock()
	if cached, ok := r.cache[tenantID]; ok && time.Since(cached.cachedAt) < r.cacheTTL {
		r.mu.RUnlock()
		return cached, nil
	}
	r.mu.RUnlock()

	// Load from database
	config, err := r.loadConfig(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	// Update cache
	r.mu.Lock()
	r.cache[tenantID] = config
	r.mu.Unlock()

	return config, nil
}

// InvalidateCache invalidates the cache for a tenant.
func (r *ConfigResolver) InvalidateCache(tenantID string) {
	r.mu.Lock()
	delete(r.cache, tenantID)
	r.mu.Unlock()
}

// InvalidateAll invalidates all cached configurations.
func (r *ConfigResolver) InvalidateAll() {
	r.mu.Lock()
	r.cache = make(map[string]*TenantConfig)
	r.mu.Unlock()
}

func (r *ConfigResolver) loadConfig(ctx context.Context, tenantID string) (*TenantConfig, error) {
	tenant, err := r.repo.GetTenant(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant: %w", err)
	}
	if tenant == nil {
		return nil, fmt.Errorf("tenant not found: %s", tenantID)
	}

	config := &TenantConfig{
		TenantID:     tenantID,
		TenantSlug:   tenant.Slug,
		Integrations: make(map[string]Integration),
		cachedAt:     time.Now(),
	}

	// Load domains
	domains, err := r.repo.GetTenantDomains(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get domains: %w", err)
	}
	for _, d := range domains {
		if d.DomainType == "internal" {
			config.InternalDomains = append(config.InternalDomains, d.Domain)
		}
	}

	// Load email patterns
	patterns, err := r.repo.GetTenantEmailPatterns(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get email patterns: %w", err)
	}
	for _, p := range patterns {
		if !p.Enabled {
			continue
		}
		switch p.PatternType {
		case "bot":
			config.BotPatterns = append(config.BotPatterns, p.Pattern)
		case "distribution_list":
			config.DistributionPatterns = append(config.DistributionPatterns, p.Pattern)
		case "role_account":
			config.RoleAccountPatterns = append(config.RoleAccountPatterns, p.Pattern)
		case "ignore":
			config.IgnorePatterns = append(config.IgnorePatterns, p.Pattern)
		}
	}

	// Load processing rules
	rules, err := r.repo.GetTenantProcessingRules(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get processing rules: %w", err)
	}
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		config.ProcessingRules = append(config.ProcessingRules, ProcessingRule{
			ID:                     rule.ID,
			Name:                   rule.RuleName,
			Priority:               rule.Priority,
			MatchConditions:        rule.MatchConditions,
			ClassificationOverride: rule.ClassificationOverride,
			Enabled:                rule.Enabled,
		})
	}

	// Load integrations
	integrations, err := r.repo.GetTenantIntegrations(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get integrations: %w", err)
	}
	for _, i := range integrations {
		if !i.Enabled {
			continue
		}
		config.Integrations[i.IntegrationType] = Integration{
			ID:             i.ID,
			Type:           i.IntegrationType,
			Name:           i.Name,
			InstanceURL:    i.InstanceURL,
			Config:         i.Config,
			CredentialsKey: i.CredentialsKey,
			Enabled:        i.Enabled,
			SyncStatus:     i.SyncStatus,
			LastSyncAt:     i.LastSyncAt,
		}
	}

	return config, nil
}

// IsInternalDomain checks if a domain is internal for the tenant.
func (c *TenantConfig) IsInternalDomain(domain string) bool {
	domain = strings.ToLower(domain)
	for _, d := range c.InternalDomains {
		if strings.ToLower(d) == domain {
			return true
		}
		// Support subdomain matching (e.g., sub.company.com matches company.com)
		if strings.HasSuffix(domain, "."+strings.ToLower(d)) {
			return true
		}
	}
	return false
}

// MatchesPattern checks if an email matches any pattern of the given type.
func (c *TenantConfig) MatchesPattern(email string, patternType string) bool {
	email = strings.ToLower(email)

	var patterns []string
	switch patternType {
	case "bot":
		patterns = c.BotPatterns
	case "distribution_list":
		patterns = c.DistributionPatterns
	case "role_account":
		patterns = c.RoleAccountPatterns
	case "ignore":
		patterns = c.IgnorePatterns
	}

	for _, pattern := range patterns {
		if matchGlob(email, strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}

// GetMatchingRule returns the first processing rule that matches, or nil.
func (c *TenantConfig) GetMatchingRule(from, to, subject string, headers map[string]string) *ProcessingRule {
	for _, rule := range c.ProcessingRules {
		if matchesRule(rule.MatchConditions, from, to, subject, headers) {
			return &rule
		}
	}
	return nil
}

// GetIntegration returns the integration config for a type, or nil.
func (c *TenantConfig) GetIntegration(integrationType string) *Integration {
	if i, ok := c.Integrations[integrationType]; ok {
		return &i
	}
	return nil
}

func matchesRule(cond MatchConditions, from, to, subject string, headers map[string]string) bool {
	from = strings.ToLower(from)
	to = strings.ToLower(to)
	subject = strings.ToLower(subject)

	if cond.FromContains != "" && !strings.Contains(from, strings.ToLower(cond.FromContains)) {
		return false
	}
	if cond.FromMatches != "" && !matchGlob(from, strings.ToLower(cond.FromMatches)) {
		return false
	}
	if cond.ToContains != "" && !strings.Contains(to, strings.ToLower(cond.ToContains)) {
		return false
	}
	if cond.SubjectContains != "" && !strings.Contains(subject, strings.ToLower(cond.SubjectContains)) {
		return false
	}
	if cond.SubjectStartsWith != "" && !strings.HasPrefix(subject, strings.ToLower(cond.SubjectStartsWith)) {
		return false
	}
	if cond.HasHeader != "" {
		if _, ok := headers[cond.HasHeader]; !ok {
			return false
		}
	}
	if cond.HeaderContains != nil {
		for header, value := range cond.HeaderContains {
			headerVal, ok := headers[header]
			if !ok || !strings.Contains(strings.ToLower(headerVal), strings.ToLower(value)) {
				return false
			}
		}
	}

	return true
}

// matchGlob performs simple glob matching with * wildcard.
func matchGlob(s, pattern string) bool {
	// Simple glob matching supporting only * wildcard
	matched, _ := filepath.Match(pattern, s)
	if matched {
		return true
	}

	// Handle patterns like *-jira@* that filepath.Match doesn't handle well
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return s == pattern
	}

	// Check prefix
	if parts[0] != "" && !strings.HasPrefix(s, parts[0]) {
		return false
	}

	// Check suffix
	last := parts[len(parts)-1]
	if last != "" && !strings.HasSuffix(s, last) {
		return false
	}

	// Check middle parts
	remaining := s
	for i, part := range parts {
		if part == "" {
			continue
		}
		idx := strings.Index(remaining, part)
		if idx == -1 {
			return false
		}
		// For first part, must be at start (already checked above)
		// For last part, must be at end (already checked above)
		// For middle parts, just needs to be present in order
		if i == 0 && idx != 0 {
			return false
		}
		remaining = remaining[idx+len(part):]
	}

	return true
}
