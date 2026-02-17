// Package learning provides correction tracking and learning capabilities.
package learning

import (
	"sort"
	"strings"
	"time"
)

// MentionCluster represents a cluster of similar mentions.
type MentionCluster struct {
	ID             string          `json:"id"`
	Representative string          `json:"representative"` // Most common text
	Members        []ClusterMember `json:"members"`
	Size           int             `json:"size"`
	SimilarityType string          `json:"similarity_type"` // phonetic, substring, acronym, typo
	CreatedAt      time.Time       `json:"created_at"`
}

// ClusterMember represents a mention in a cluster.
type ClusterMember struct {
	MentionText     string  `json:"mention_text"`
	Count           int     `json:"count"` // Times seen
	SimilarityScore float32 `json:"similarity_score"`
	ContentSample   string  `json:"content_sample,omitempty"` // Sample context
}

// ClusterOptions configures clustering behavior.
type ClusterOptions struct {
	MinClusterSize      int     `json:"min_cluster_size"`
	SimilarityThreshold float32 `json:"similarity_threshold"`
}

// DefaultClusterOptions returns default clustering options.
func DefaultClusterOptions() ClusterOptions {
	return ClusterOptions{
		MinClusterSize:      2,
		SimilarityThreshold: 0.7,
	}
}

// ClusterMentions clusters a list of mention texts by similarity.
func ClusterMentions(mentionTexts map[string]int, opts ClusterOptions) []MentionCluster {
	var clusters []MentionCluster
	processed := make(map[string]bool)

	for text, count := range mentionTexts {
		if processed[text] {
			continue
		}

		cluster := MentionCluster{
			ID:             generateClusterID(),
			Representative: text,
			Size:           count,
			CreatedAt:      time.Now(),
		}

		cluster.Members = append(cluster.Members, ClusterMember{
			MentionText:     text,
			Count:           count,
			SimilarityScore: 1.0,
		})
		processed[text] = true

		// Find similar texts
		for otherText, otherCount := range mentionTexts {
			if processed[otherText] {
				continue
			}

			simType, simScore := calculateSimilarity(text, otherText)
			if simScore >= opts.SimilarityThreshold {
				cluster.Members = append(cluster.Members, ClusterMember{
					MentionText:     otherText,
					Count:           otherCount,
					SimilarityScore: simScore,
				})
				cluster.Size += otherCount
				if cluster.SimilarityType == "" {
					cluster.SimilarityType = simType
				}
				processed[otherText] = true
			}
		}

		// Only keep clusters that meet minimum size
		if len(cluster.Members) >= opts.MinClusterSize {
			// Find representative (most common)
			sort.Slice(cluster.Members, func(i, j int) bool {
				return cluster.Members[i].Count > cluster.Members[j].Count
			})
			cluster.Representative = cluster.Members[0].MentionText
			clusters = append(clusters, cluster)
		}
	}

	// Sort clusters by size
	sort.Slice(clusters, func(i, j int) bool {
		return clusters[i].Size > clusters[j].Size
	})

	return clusters
}

// calculateSimilarity calculates similarity between two strings.
func calculateSimilarity(a, b string) (simType string, score float32) {
	a = strings.ToLower(strings.TrimSpace(a))
	b = strings.ToLower(strings.TrimSpace(b))

	if a == b {
		return "exact", 1.0
	}

	// Check substring
	if strings.Contains(a, b) || strings.Contains(b, a) {
		shorter := a
		if len(b) < len(a) {
			shorter = b
		}
		longer := a
		if len(b) > len(a) {
			longer = b
		}
		return "substring", float32(len(shorter)) / float32(len(longer))
	}

	// Check phonetic similarity (simplified)
	if phoneticMatch(a, b) {
		return "phonetic", 0.85
	}

	// Check edit distance
	dist := levenshteinDistance(a, b)
	maxLen := len(a)
	if len(b) > maxLen {
		maxLen = len(b)
	}
	if maxLen > 0 {
		editSim := 1.0 - (float32(dist) / float32(maxLen))
		if editSim > 0.7 {
			return "typo", editSim
		}
	}

	// Check acronym match
	if isAcronymMatch(a, b) {
		return "acronym", 0.9
	}

	return "", 0
}

// phoneticMatch checks for phonetic similarity (simplified).
func phoneticMatch(a, b string) bool {
	// Simple phonetic rules - can be enhanced with Soundex/Metaphone
	replacements := map[string]string{
		"ph": "f",
		"ck": "k",
		"ee": "i",
		"ou": "u",
	}

	normalizePhonetic := func(s string) string {
		result := strings.ToLower(s)
		for from, to := range replacements {
			result = strings.ReplaceAll(result, from, to)
		}
		return result
	}

	return normalizePhonetic(a) == normalizePhonetic(b)
}

// levenshteinDistance calculates the edit distance between two strings.
func levenshteinDistance(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	// Create matrix
	matrix := make([][]int, len(a)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(b)+1)
		matrix[i][0] = i
	}
	for j := range matrix[0] {
		matrix[0][j] = j
	}

	// Fill matrix
	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}

			matrix[i][j] = minInt(
				matrix[i-1][j]+1,      // deletion
				matrix[i][j-1]+1,      // insertion
				matrix[i-1][j-1]+cost, // substitution
			)
		}
	}

	return matrix[len(a)][len(b)]
}

// minInt returns the minimum of three integers.
func minInt(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

// isAcronymMatch checks if one string is an acronym of the other.
func isAcronymMatch(a, b string) bool {
	// Check if shorter is acronym of longer
	shorter, longer := a, b
	if len(b) < len(a) {
		shorter, longer = b, a
	}

	// Build acronym from longer
	words := strings.Fields(longer)
	if len(words) < 2 {
		return false
	}

	var acronym strings.Builder
	for _, word := range words {
		if len(word) > 0 {
			acronym.WriteByte(word[0])
		}
	}

	return strings.EqualFold(acronym.String(), shorter)
}

// generateClusterID generates a unique cluster ID.
func generateClusterID() string {
	return "cluster_" + time.Now().Format("20060102150405")
}

// ClusterSummary provides a summary of clustering analysis.
type ClusterSummary struct {
	TotalMentions  int `json:"total_mentions"`
	TotalClusters  int `json:"total_clusters"`
	LargestCluster int `json:"largest_cluster"`
}

// GetClusterSummary returns a summary of clusters.
func GetClusterSummary(clusters []MentionCluster) *ClusterSummary {
	summary := &ClusterSummary{
		TotalClusters: len(clusters),
	}

	for _, c := range clusters {
		summary.TotalMentions += c.Size
		if c.Size > summary.LargestCluster {
			summary.LargestCluster = c.Size
		}
	}

	return summary
}
