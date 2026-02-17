package entities

import (
	"regexp"
	"strings"
	"unicode"
)

// NormalizeDisplayName normalizes a display name to canonical form.
// - "Eskelsen, Rick" → "Rick Eskelsen"
// - "  James  Brown  " → "James Brown"
// - "\"John Doe\"" → "John Doe"
func NormalizeDisplayName(name string) string {
	if name == "" {
		return ""
	}

	// Remove quotes
	name = strings.Trim(name, `"'`)

	// Handle "Last, First" format
	if strings.Contains(name, ",") {
		parts := strings.SplitN(name, ",", 2)
		if len(parts) == 2 {
			first := strings.TrimSpace(parts[1])
			last := strings.TrimSpace(parts[0])
			if first != "" && last != "" {
				name = first + " " + last
			}
		}
	}

	// Normalize whitespace
	name = strings.Join(strings.Fields(name), " ")

	// Title case each word
	name = titleCase(name)

	return name
}

// titleCase converts a string to title case, handling edge cases.
func titleCase(s string) string {
	words := strings.Fields(s)
	for i, word := range words {
		if len(word) == 0 {
			continue
		}
		// Convert to title case
		runes := []rune(strings.ToLower(word))
		runes[0] = unicode.ToUpper(runes[0])
		words[i] = string(runes)
	}
	return strings.Join(words, " ")
}

// ExtractDomain extracts the domain from an email address.
func ExtractDomain(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return ""
	}
	// Both local part and domain must be non-empty
	if parts[0] == "" || parts[1] == "" {
		return ""
	}
	return strings.ToLower(parts[1])
}

// Common patterns for account type detection
var (
	botPatterns = []string{
		"noreply",
		"no-reply",
		"donotreply",
		"do-not-reply",
		"mailer-daemon",
		"postmaster",
		"jira",
		"jenkins",
		"github",
		"gitlab",
		"circleci",
		"travis",
		"bot",
		"automation",
		"system",
		"alert",
		"notification",
		"gsd-", // Akamai service accounts (Get Stuff Done)
	}

	distributionPatterns = []string{
		"team-",
		"all-",
		"group-",
		"list-",
		"dl-",
		"dept-",
		"everyone",
		"staff",
		"employees",
	}

	rolePatterns = []string{
		"support",
		"sales",
		"info",
		"contact",
		"help",
		"admin",
		"security",
		"hr",
		"recruiting",
		"careers",
		"press",
		"media",
		"legal",
		"finance",
		"billing",
		"accounts",
		"facilitator",
		"prb-facilitator", // Akamai PRB facilitator role account
	}

	externalServiceDomains = []string{
		"docs.google.com",
		"calendar.google.com",
		"slack.com",
		"atlassian.net",
		"github.com",
		"gitlab.com",
		"circleci.com",
		"travis-ci.org",
		"travis-ci.com",
		"mailer.aha.io", // Aha! product management tool
	}
)

// DetectAccountType determines the account type based on email and display name.
func DetectAccountType(email, displayName string) AccountType {
	return DetectAccountTypeWithPatterns(email, displayName, nil)
}

// DetectAccountTypeWithPatterns determines the account type based on email and display name,
// using both hardcoded default patterns and optional extra patterns.
// The extraPatterns are merged with defaults (not replacing them).
// If extraPatterns is nil, behaves identically to DetectAccountType.
func DetectAccountTypeWithPatterns(email, displayName string, extraPatterns *AccountTypePatterns) AccountType {
	emailLower := strings.ToLower(email)
	domain := ExtractDomain(email)

	// Build merged pattern lists
	externalDomains := externalServiceDomains
	bots := botPatterns
	distributions := distributionPatterns
	roles := rolePatterns

	if extraPatterns != nil {
		if len(extraPatterns.ExternalDomains) > 0 {
			externalDomains = append(externalDomains, extraPatterns.ExternalDomains...)
		}
		if len(extraPatterns.BotPatterns) > 0 {
			bots = append(bots, extraPatterns.BotPatterns...)
		}
		if len(extraPatterns.DistributionPatterns) > 0 {
			distributions = append(distributions, extraPatterns.DistributionPatterns...)
		}
		if len(extraPatterns.RolePatterns) > 0 {
			roles = append(roles, extraPatterns.RolePatterns...)
		}
	}

	// Check for external service first
	for _, svcDomain := range externalDomains {
		if strings.HasSuffix(domain, svcDomain) || domain == svcDomain {
			return AccountTypeExternalService
		}
	}

	// Check local part patterns
	localPart := strings.ToLower(strings.Split(email, "@")[0])

	// Bot patterns
	for _, pattern := range bots {
		if strings.Contains(localPart, pattern) || strings.Contains(emailLower, pattern) {
			return AccountTypeBot
		}
	}

	// Distribution list patterns
	for _, pattern := range distributions {
		if strings.HasPrefix(localPart, pattern) {
			return AccountTypeDistribution
		}
	}

	// Role account patterns
	for _, pattern := range roles {
		if localPart == pattern || strings.HasPrefix(localPart, pattern+"-") {
			return AccountTypeRole
		}
	}

	// Default to person
	return AccountTypePerson
}

// NameSimilarity calculates similarity between two names (0.0 to 1.0).
// It compares first and last name components separately to avoid false positives
// where people share a first name but have different last names.
func NameSimilarity(a, b string) float64 {
	// Normalize both names
	a = strings.ToLower(NormalizeDisplayName(a))
	b = strings.ToLower(NormalizeDisplayName(b))

	if a == "" || b == "" {
		return 0.0
	}

	// Exact match
	if a == b {
		return 1.0
	}

	// Minimum length check: reject single-character matches (bug pf-96c91a)
	// Single-character names like "K" or "M" should not match real names
	if len(a) < 2 || len(b) < 2 {
		// Allow Levenshtein for very short strings (both < 2 chars)
		if len(a) < 2 && len(b) < 2 {
			return levenshteinSimilarity(a, b)
		}
		// One is too short, the other is longer - this is not a match
		return 0.0
	}

	// Check if one contains the other (substring match)
	// Apply length-based penalty: shorter substrings get lower scores
	var shorter, longer string
	if len(a) < len(b) {
		shorter, longer = a, b
	} else {
		shorter, longer = b, a
	}

	if strings.Contains(longer, shorter) {
		// For legitimate partial name matches (>= 4 chars), keep the old behavior (0.85)
		// Examples: "Rick" in "Rick Eskelsen", "Mike" in "Mike Johnson"
		if len(shorter) >= 4 {
			return 0.85
		}

		// For very short substrings (2-3 chars), apply length-based penalty
		// Examples:
		// - "Mi" in "Mike" (2 chars) -> 0.2
		// - "ike" in "Mike" (3 chars) -> 0.4
		// This prevents single initials and very short fragments from matching
		if len(shorter) == 2 {
			return 0.2
		}
		if len(shorter) == 3 {
			return 0.4
		}

		// Fallback (shouldn't reach here due to length check above)
		return 0.1
	}

	// Split into components (first, last)
	wordsA := strings.Fields(a)
	wordsB := strings.Fields(b)

	// Handle single-word names (no last name)
	if len(wordsA) == 1 || len(wordsB) == 1 {
		// If both are single words, compare them directly
		if len(wordsA) == 1 && len(wordsB) == 1 {
			return levenshteinSimilarity(wordsA[0], wordsB[0])
		}
		// One has multiple words, one has single word
		// Check if the single word matches any component
		single := wordsA[0]
		multi := wordsB
		if len(wordsB) == 1 {
			single = wordsB[0]
			multi = wordsA
		}
		for _, word := range multi {
			if word == single {
				return 0.85 // partial match
			}
		}
		// No exact match, use Levenshtein on full names
		return levenshteinSimilarity(a, b)
	}

	// Both names have at least 2 components
	// Assume: first word = first name, last word = last name
	firstA := wordsA[0]
	lastA := wordsA[len(wordsA)-1]
	firstB := wordsB[0]
	lastB := wordsB[len(wordsB)-1]

	// Compare first names
	firstSim := levenshteinSimilarity(firstA, firstB)

	// Compare last names
	lastSim := levenshteinSimilarity(lastA, lastB)

	// If last names are clearly different (low similarity), penalize heavily
	// Even if first names match exactly, different last names = different people
	// We use a threshold of 0.7 - last names need to be quite similar to be considered a match
	// Examples:
	// - "brisbane" vs "bussmann" = 0.50 (different people)
	// - "brown" vs "dement" = 0.17 (different people)
	// - "smith" vs "smyth" = 0.80 (likely same person, spelling variant)
	if lastSim < 0.7 {
		// Different last names - even with same first name, this is likely different people
		// Return a low score that combines first name similarity with a heavy penalty
		// With EntitySimilarity weights (0.73 name, 0.22 domain):
		// - If we return 0.3 here, final score = 0.3 * 0.73 + 0.22 = 0.439 (below 0.5 threshold)
		// - If we return 0.4 here, final score = 0.4 * 0.73 + 0.22 = 0.512 (above 0.5 threshold)
		// So we use 0.3 as the penalty multiplier to keep false positives below 0.5
		return firstSim * 0.3
	}

	// Both first and last names are similar
	// Weight them equally: 50% first name, 50% last name
	return (firstSim + lastSim) / 2.0
}

// levenshteinSimilarity calculates similarity based on Levenshtein distance.
func levenshteinSimilarity(a, b string) float64 {
	distance := levenshteinDistance(a, b)
	maxLen := max(len(a), len(b))
	if maxLen == 0 {
		return 1.0
	}
	return 1.0 - float64(distance)/float64(maxLen)
}

// levenshteinDistance calculates the Levenshtein distance between two strings.
func levenshteinDistance(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	// Create distance matrix
	matrix := make([][]int, len(a)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(b)+1)
	}

	// Initialize first row and column
	for i := 0; i <= len(a); i++ {
		matrix[i][0] = i
	}
	for j := 0; j <= len(b); j++ {
		matrix[0][j] = j
	}

	// Fill in the rest
	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			matrix[i][j] = min(
				matrix[i-1][j]+1,      // deletion
				matrix[i][j-1]+1,      // insertion
				matrix[i-1][j-1]+cost, // substitution
			)
		}
	}

	return matrix[len(a)][len(b)]
}

func min(values ...int) int {
	m := values[0]
	for _, v := range values[1:] {
		if v < m {
			m = v
		}
	}
	return m
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// IsInternalDomain checks if an email is from an internal domain.
func IsInternalDomain(email string, internalDomains []string) bool {
	domain := ExtractDomain(email)
	for _, internal := range internalDomains {
		if domain == internal || strings.HasSuffix(domain, "."+internal) {
			return true
		}
	}
	return false
}

// emailLocalPartPattern matches the local part of an email
var emailLocalPartPattern = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+$`)

// IsValidEmail checks if a string looks like a valid email address.
func IsValidEmail(email string) bool {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}
	local, domain := parts[0], parts[1]
	if local == "" || domain == "" {
		return false
	}
	if !emailLocalPartPattern.MatchString(local) {
		return false
	}
	if !strings.Contains(domain, ".") {
		return false
	}
	return true
}

// DeriveNameFromEmail derives a human-readable name from an email address prefix.
// Patterns:
//   - "john.smith@example.com" → "John Smith" (split on separator)
//   - "jane_doe@example.com" → "Jane Doe" (split on separator)
//   - "mary-ann@example.com" → "Mary Ann" (split on separator)
//   - "jSmith@example.com" → "J Smith" (split on camelCase)
//   - "jsmith@example.com" → "Jsmith" (single word, title cased - no split)
//   - "uzeeshan@example.com" → "Uzeeshan" (single word, title cased - no split)
func DeriveNameFromEmail(email string) string {
	if email == "" {
		return ""
	}

	// Extract local part (before @)
	parts := strings.Split(email, "@")
	if len(parts) != 2 || parts[0] == "" {
		return ""
	}

	local := parts[0]

	// Check for camelCase pattern before replacing separators
	// e.g., "jSmith" → "j smith" (camelCase detected)
	// BUT preserve single-word usernames like "jsmith", "uzeeshan", "knaidu" (all lowercase)
	//
	// Heuristic: Only split if camelCase (lowercase followed by uppercase)
	// - "jSmith" (camelCase) → split → "J Smith"
	// - "jsmith" (all lowercase) → DON'T split → "Jsmith"
	// - "uzeeshan" (all lowercase) → DON'T split → "Uzeeshan"
	// - "knaidu" (all lowercase) → DON'T split → "Knaidu"
	//
	// This fix prevents false splitting of corporate email prefixes like "uzeeshan" → "U Zeeshan"
	if len(local) >= 3 && !strings.ContainsAny(local, "._-") {
		if isLetter(rune(local[0])) && isLetter(rune(local[1])) {
			firstChar := rune(local[0])
			secondChar := rune(local[1])

			// Only split on camelCase pattern (jSmith)
			if unicode.IsLower(firstChar) && unicode.IsUpper(secondChar) {
				local = string(local[0]) + " " + local[1:]
			}
		}
	}

	// Replace common separators with spaces
	// Handle . _ - as word separators
	name := local
	name = strings.ReplaceAll(name, ".", " ")
	name = strings.ReplaceAll(name, "_", " ")
	name = strings.ReplaceAll(name, "-", " ")

	// Remove numbers (optional - keeping them for now as they may be meaningful)
	// Could add: name = regexp.MustCompile(`\d+`).ReplaceAllString(name, "")

	// Split into words
	words := strings.Fields(name)
	if len(words) == 0 {
		return ""
	}

	// Handle single-letter first word (likely an initial)
	// e.g., "j smith" → "J Smith"
	if len(words) > 1 && len(words[0]) == 1 {
		// First word is a single letter, capitalize it
		words[0] = strings.ToUpper(words[0])
		// Title case the rest
		for i := 1; i < len(words); i++ {
			words[i] = titleCase(words[i])
		}
		return strings.Join(words, " ")
	}

	// Normal case: title case all words
	name = strings.Join(words, " ")
	return titleCase(name)
}

// isLetter checks if a rune is a letter.
func isLetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

// EntityComparisonData contains the data needed to compare two entities for similarity.
type EntityComparisonData struct {
	Name      string
	Email     string
	Domain    string
	SourceIDs []string
}

// EntitySimilarity calculates weighted similarity between two entities (0.0 to 1.0).
// It combines name similarity with bonuses for matching domain and shared sources:
// - Name similarity (primary signal, scaled to 0.73 max contribution)
// - Email domain match (adds 0.22 bonus)
// - Shared sources (adds 0.05 bonus)
// The result is capped at 1.0.
func EntitySimilarity(a, b *EntityComparisonData) float64 {
	// Handle nil inputs
	if a == nil || b == nil {
		return 0.0
	}

	// Name similarity is the primary signal
	nameSim := NameSimilarity(a.Name, b.Name)

	// If names don't match at all, the entities are not similar
	if nameSim == 0.0 {
		return 0.0
	}

	// Start with name similarity as primary component (scaled 0.73)
	score := nameSim * 0.73

	// Add domain bonus (0.22)
	if a.Domain != "" && b.Domain != "" && a.Domain == b.Domain {
		score += 0.22
	}

	// Add shared source bonus (0.05)
	if hasSharedSources(a.SourceIDs, b.SourceIDs) {
		score += 0.05
	}

	// Cap at 1.0
	if score > 1.0 {
		score = 1.0
	}

	return score
}

// hasSharedSources returns true if the two slices share at least one common element.
func hasSharedSources(a, b []string) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}

	// Use a map for efficient lookup
	seen := make(map[string]bool)
	for _, id := range a {
		seen[id] = true
	}

	for _, id := range b {
		if seen[id] {
			return true
		}
	}

	return false
}
