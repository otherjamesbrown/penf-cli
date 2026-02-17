package config

import (
	"testing"
)

func TestTenantConfig_IsInternalDomain(t *testing.T) {
	config := &TenantConfig{
		InternalDomains: []string{"company.com", "corp.company.com"},
	}

	tests := []struct {
		domain string
		want   bool
	}{
		{"company.com", true},
		{"COMPANY.COM", true}, // case insensitive
		{"corp.company.com", true},
		{"sub.company.com", true}, // subdomain matching
		{"external.com", false},
		{"notcompany.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.domain, func(t *testing.T) {
			got := config.IsInternalDomain(tt.domain)
			if got != tt.want {
				t.Errorf("IsInternalDomain(%s) = %v, want %v", tt.domain, got, tt.want)
			}
		})
	}
}

func TestTenantConfig_MatchesPattern(t *testing.T) {
	config := &TenantConfig{
		BotPatterns:          []string{"*-jira@*", "noreply@*", "bot-*@company.com"},
		DistributionPatterns: []string{"team-*@*", "all-*@company.com"},
		RoleAccountPatterns:  []string{"support@*", "sales@company.com"},
	}

	tests := []struct {
		email       string
		patternType string
		want        bool
	}{
		// Bot patterns
		{"team-jira@company.atlassian.net", "bot", true},
		{"noreply@company.com", "bot", true},
		{"bot-alerts@company.com", "bot", true},
		{"john@company.com", "bot", false},

		// Distribution patterns
		{"team-engineering@company.com", "distribution_list", true},
		{"all-staff@company.com", "distribution_list", true},
		{"john@company.com", "distribution_list", false},

		// Role account patterns
		{"support@company.com", "role_account", true},
		{"support@other.com", "role_account", true},
		{"sales@company.com", "role_account", true},
		{"sales@other.com", "role_account", false}, // sales only matches company.com
	}

	for _, tt := range tests {
		t.Run(tt.email+"_"+tt.patternType, func(t *testing.T) {
			got := config.MatchesPattern(tt.email, tt.patternType)
			if got != tt.want {
				t.Errorf("MatchesPattern(%s, %s) = %v, want %v", tt.email, tt.patternType, got, tt.want)
			}
		})
	}
}

func TestTenantConfig_GetMatchingRule(t *testing.T) {
	config := &TenantConfig{
		ProcessingRules: []ProcessingRule{
			{
				ID:       1,
				Name:     "skip-alerts",
				Priority: 10,
				MatchConditions: MatchConditions{
					FromContains: "alerts@",
				},
				ClassificationOverride: ClassificationOverride{
					ContentSubtype:    "notification/internal",
					ProcessingProfile: "metadata_only",
				},
			},
			{
				ID:       2,
				Name:     "jira-priority",
				Priority: 20,
				MatchConditions: MatchConditions{
					FromContains:    "jira@",
					SubjectContains: "[URGENT]",
				},
				ClassificationOverride: ClassificationOverride{
					ProcessingProfile: "full_ai",
				},
			},
		},
	}

	tests := []struct {
		name    string
		from    string
		to      string
		subject string
		headers map[string]string
		wantID  *int64
	}{
		{
			name:    "matches alerts rule",
			from:    "alerts@company.com",
			subject: "Server down",
			wantID:  intPtr(1),
		},
		{
			name:    "matches jira priority rule",
			from:    "jira@company.atlassian.net",
			subject: "[URGENT] OUT-123 needs attention",
			wantID:  intPtr(2),
		},
		{
			name:    "no match - missing subject",
			from:    "jira@company.atlassian.net",
			subject: "OUT-123 updated",
			wantID:  nil, // jira rule requires [URGENT] in subject
		},
		{
			name:    "no match",
			from:    "john@company.com",
			subject: "Hello",
			wantID:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := config.GetMatchingRule(tt.from, tt.to, tt.subject, tt.headers)
			if tt.wantID == nil {
				if got != nil {
					t.Errorf("GetMatchingRule() = %+v, want nil", got)
				}
			} else {
				if got == nil {
					t.Errorf("GetMatchingRule() = nil, want rule ID %d", *tt.wantID)
				} else if got.ID != *tt.wantID {
					t.Errorf("GetMatchingRule().ID = %d, want %d", got.ID, *tt.wantID)
				}
			}
		})
	}
}

func TestTenantConfig_GetIntegration(t *testing.T) {
	config := &TenantConfig{
		Integrations: map[string]Integration{
			"jira": {
				ID:          1,
				Type:        "jira",
				Name:        "Company Jira",
				InstanceURL: "company.atlassian.net",
				Enabled:     true,
			},
		},
	}

	// Found
	jira := config.GetIntegration("jira")
	if jira == nil {
		t.Error("GetIntegration(jira) = nil, want integration")
	} else if jira.InstanceURL != "company.atlassian.net" {
		t.Errorf("GetIntegration(jira).InstanceURL = %s, want company.atlassian.net", jira.InstanceURL)
	}

	// Not found
	slack := config.GetIntegration("slack")
	if slack != nil {
		t.Errorf("GetIntegration(slack) = %+v, want nil", slack)
	}
}

func TestMatchGlob(t *testing.T) {
	tests := []struct {
		s       string
		pattern string
		want    bool
	}{
		{"hello", "hello", true},
		{"hello", "world", false},
		{"hello@world.com", "*@world.com", true},
		{"team-jira@company.atlassian.net", "*-jira@*", true},
		{"noreply@company.com", "noreply@*", true},
		{"bot-alerts@company.com", "bot-*@company.com", true},
		{"alerts@company.com", "bot-*@company.com", false},
		{"test", "*", true},
		{"", "*", true},
		{"hello", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.s+"_"+tt.pattern, func(t *testing.T) {
			got := matchGlob(tt.s, tt.pattern)
			if got != tt.want {
				t.Errorf("matchGlob(%q, %q) = %v, want %v", tt.s, tt.pattern, got, tt.want)
			}
		})
	}
}

func TestMatchesRule(t *testing.T) {
	tests := []struct {
		name    string
		cond    MatchConditions
		from    string
		to      string
		subject string
		headers map[string]string
		want    bool
	}{
		{
			name: "from_contains match",
			cond: MatchConditions{FromContains: "alerts"},
			from: "alerts@company.com",
			want: true,
		},
		{
			name: "from_contains no match",
			cond: MatchConditions{FromContains: "alerts"},
			from: "john@company.com",
			want: false,
		},
		{
			name: "subject_contains match",
			cond: MatchConditions{SubjectContains: "URGENT"},
			subject: "[URGENT] Please review",
			want:    true,
		},
		{
			name: "subject_starts_with match",
			cond: MatchConditions{SubjectStartsWith: "Re:"},
			subject: "Re: Discussion",
			want:    true,
		},
		{
			name:    "has_header match",
			cond:    MatchConditions{HasHeader: "X-Custom"},
			headers: map[string]string{"X-Custom": "value"},
			want:    true,
		},
		{
			name:    "has_header no match",
			cond:    MatchConditions{HasHeader: "X-Custom"},
			headers: map[string]string{"X-Other": "value"},
			want:    false,
		},
		{
			name: "multiple conditions all match",
			cond: MatchConditions{
				FromContains:    "jira",
				SubjectContains: "[PROJ-",
			},
			from:    "jira@company.atlassian.net",
			subject: "[PROJ-123] Updated",
			want:    true,
		},
		{
			name: "multiple conditions partial match",
			cond: MatchConditions{
				FromContains:    "jira",
				SubjectContains: "[URGENT]",
			},
			from:    "jira@company.atlassian.net",
			subject: "[PROJ-123] Updated", // missing [URGENT]
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesRule(tt.cond, tt.from, tt.to, tt.subject, tt.headers)
			if got != tt.want {
				t.Errorf("matchesRule() = %v, want %v", got, tt.want)
			}
		})
	}
}

func intPtr(i int64) *int64 {
	return &i
}
