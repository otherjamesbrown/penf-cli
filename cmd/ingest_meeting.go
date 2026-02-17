package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"

	ingestv1 "github.com/otherjamesbrown/penf-cli/api/proto/ingest/v1"
	"github.com/otherjamesbrown/penf-cli/client"
	"github.com/otherjamesbrown/penf-cli/config"
	"github.com/otherjamesbrown/penf-cli/pkg/contentid"
	"github.com/otherjamesbrown/penf-cli/pkg/ingest/meeting"
)

// Meeting ingest specific flags
var (
	meetingSource   string
	meetingPlatform string
	meetingDryRun   bool
	meetingSeries   string
	meetingTitle    string
	meetingDate     string
)

// DefaultTenantID for single-tenant mode
const DefaultTenantID = "00000001-0000-0000-0000-000000000001"

// newIngestMeetingCommand creates the 'ingest meeting' subcommand.
func newIngestMeetingCommand(deps *IngestCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "meeting <path>",
		Short: "Ingest meeting transcripts into Penfold",
		Long: `Ingest meeting transcripts, chat logs, and metadata into Penfold.

Supports:
  - WebVTT (.vtt) transcripts from Webex/Zoom
  - Plain text transcripts (Transcript_*.txt)
  - Chat logs (Chat messages_*.txt)
  - Meeting directories with multiple related files

Files are automatically grouped by meeting. Transcripts are parsed to extract
participants and generate embeddings for semantic search.

Examples:
  # Ingest a single VTT transcript
  penf ingest meeting ./meeting.vtt --source "project-x"

  # Ingest a meeting directory (transcript + chat)
  penf ingest meeting ./MeetingFolder/ --source "weekly-sync"

  # Ingest all meetings from a directory
  penf ingest meeting ~/meetings/ --source "archive-2025"

  # Preview without importing (dry run)
  penf ingest meeting ./meetings/ --source "test" --dry-run`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runIngestMeeting(cmd.Context(), deps, args[0])
		},
	}

	// Meeting-specific flags
	cmd.Flags().StringVarP(&meetingSource, "source", "s", "", "Source tag identifier (required)")
	cmd.Flags().StringVar(&meetingPlatform, "platform", "webex", "Meeting platform: webex, teams, zoom, google_meet")
	cmd.Flags().BoolVar(&meetingDryRun, "dry-run", false, "Preview import without persisting")
	cmd.Flags().StringVar(&meetingSeries, "series", "", "Meeting series name (auto-created if not exists)")
	cmd.Flags().StringVar(&meetingTitle, "title", "", "Override detected meeting title")
	cmd.Flags().StringVar(&meetingDate, "date", "", "Override detected meeting date (YYYY-MM-DD)")

	cmd.MarkFlagRequired("source")

	// Add resolve subcommand
	cmd.AddCommand(newResolveMeetingParticipantsCommand(deps))
	// Add mentions subcommand
	cmd.AddCommand(newExtractMeetingMentionsCommand(deps))
	// Add series subcommand
	// TODO: Temporarily disabled - meeting_series.go has build issues
	// cmd.AddCommand(newMeetingSeriesCommand(DefaultMeetingSeriesDeps()))
	// Add set-series and unset-series subcommands
	cmd.AddCommand(newMeetingSetSeriesCommand(DefaultMeetingSeriesDeps()))
	cmd.AddCommand(newMeetingUnsetSeriesCommand(DefaultMeetingSeriesDeps()))

	return cmd
}

// newResolveMeetingParticipantsCommand creates the 'ingest meeting resolve' subcommand.
func newResolveMeetingParticipantsCommand(deps *IngestCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "resolve",
		Short: "Resolve meeting participants to people",
		Long: `Resolve meeting participants from transcripts to known people in the database.

This command matches participant names against the people table using:
  - Exact canonical name matches
  - Alias matches
  - Name normalization (strips pronouns like she/her, he/him)

Examples:
  # Resolve all unresolved meeting participants
  penf ingest meeting resolve

  # Resolve participants for a specific source
  penf ingest meeting resolve --source "test-data"`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runResolveMeetingParticipants(cmd.Context(), deps)
		},
	}
}

// runIngestMeeting executes the meeting ingestion command.
func runIngestMeeting(ctx context.Context, deps *IngestCommandDeps, path string) error {
	// Load configuration
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Validate path exists
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return fmt.Errorf("path not found: %s", path)
	}
	if err != nil {
		return fmt.Errorf("accessing path: %w", err)
	}

	// Validate source is provided
	if meetingSource == "" {
		return fmt.Errorf("--source flag is required")
	}

	// Determine tenant ID
	tenantID := ingestTenantID
	if tenantID == "" {
		tenantID = cfg.TenantID
	}
	if tenantID == "" {
		tenantID = "default"
	}

	// Display startup message
	fmt.Printf("Meeting Ingest: %s\n", path)
	fmt.Printf("  Source:      %s\n", meetingSource)
	fmt.Printf("  Platform:    %s\n", meetingPlatform)
	fmt.Printf("  Tenant:      %s\n", tenantID)
	if meetingDryRun {
		fmt.Printf("  Mode:        DRY RUN (no changes will be made)\n")
	}
	if info.IsDir() {
		fmt.Printf("  Path type:   directory\n")
	} else {
		fmt.Printf("  Path type:   single file\n")
	}
	fmt.Println()

	// Scan for meetings
	fmt.Printf("Scanning for meetings...\n")
	meetings, err := meeting.ScanMeetingFiles(path)
	if err != nil {
		return fmt.Errorf("scanning for meetings: %w", err)
	}

	if len(meetings) == 0 {
		fmt.Println("No meetings found.")
		return nil
	}

	fmt.Printf("Found %d meeting(s)\n\n", len(meetings))

	if meetingDryRun {
		// Just show what would be imported
		for i, m := range meetings {
			fmt.Printf("%d. %s (%s)\n", i+1, m.Title, m.Date.Format("2006-01-02"))
			if m.Files.TranscriptPath != "" {
				fmt.Printf("   Transcript: %s\n", m.Files.TranscriptPath)
			}
			if m.Files.ChatPath != "" {
				fmt.Printf("   Chat: %s\n", m.Files.ChatPath)
			}
			if m.Files.VideoPath != "" {
				fmt.Printf("   Video: %s\n", m.Files.VideoPath)
			}
		}
		fmt.Println("\nDry run complete. No changes made.")
		return nil
	}

	// Connect to gateway via gRPC
	conn, err := connectMeetingToGateway(cfg)
	if err != nil {
		return fmt.Errorf("connecting to gateway: %w", err)
	}
	defer conn.Close()

	ingestClient := ingestv1.NewIngestServiceClient(conn)

	// Process each meeting
	startTime := time.Now()
	var importedCount, skippedCount, failedCount int
	var contentIDs []string

	for i, m := range meetings {
		fmt.Printf("[%d/%d] Processing: %s\n", i+1, len(meetings), m.Title)

		// Generate content_id before sending to gateway
		cid := contentid.New(contentid.TypeMeeting)

		resp, err := processMeetingViaGRPC(ctx, ingestClient, m, tenantID, meetingSource, meetingPlatform, cid)
		if err != nil {
			fmt.Printf("  ERROR: %v\n", err)
			failedCount++
		} else if resp.WasDuplicate {
			fmt.Printf("  SKIPPED (duplicate)\n")
			skippedCount++
		} else {
			fmt.Printf("  Content ID: %s\n", resp.ContentId)
			fmt.Printf("  Source ID:  %s\n", resp.SourceId)
			fmt.Printf("  Status:     pending\n")
			contentIDs = append(contentIDs, resp.ContentId)
			importedCount++
		}
	}

	// Display results
	duration := time.Since(startTime)
	fmt.Println()
	fmt.Println("Ingest Complete")
	fmt.Println(strings.Repeat("=", 50))
	fmt.Printf("  Total:       %d\n", len(meetings))
	fmt.Printf("  Imported:    \033[32m%d\033[0m\n", importedCount)
	fmt.Printf("  Skipped:     \033[33m%d\033[0m\n", skippedCount)
	fmt.Printf("  Failed:      \033[31m%d\033[0m\n", failedCount)
	fmt.Printf("  Duration:    %s\n", formatDuration(duration))

	// Display content IDs if any
	if len(contentIDs) > 0 {
		fmt.Println("\nContent IDs:")
		displayCount := len(contentIDs)
		if displayCount > 10 {
			displayCount = 10
		}
		for i := 0; i < displayCount; i++ {
			fmt.Printf("  - %s\n", contentIDs[i])
		}
		if len(contentIDs) > 10 {
			fmt.Printf("  ... and %d more\n", len(contentIDs)-10)
		}
	}

	if failedCount > 0 {
		return fmt.Errorf("%d meetings failed to import", failedCount)
	}

	return nil
}

// connectMeetingToGateway establishes a gRPC connection to the gateway.
func connectMeetingToGateway(cfg *config.CLIConfig) (*grpc.ClientConn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	opts := []grpc.DialOption{
		grpc.WithBlock(),
	}

	if cfg.Insecure {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else if cfg.TLS.Enabled {
		tlsConfig, err := client.LoadClientTLSConfig(&cfg.TLS)
		if err != nil {
			return nil, fmt.Errorf("loading TLS config: %w", err)
		}
		if tlsConfig != nil {
			opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
		} else {
			opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		}
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.DialContext(ctx, cfg.ServerAddress, opts...)
	if err != nil {
		return nil, fmt.Errorf("connecting to gateway at %s: %w", cfg.ServerAddress, err)
	}

	return conn, nil
}

// platformToProto converts a platform string to the proto Platform enum.
func platformToProto(platform string) ingestv1.Platform {
	switch strings.ToLower(platform) {
	case "webex":
		// Webex is similar to Teams in functionality
		return ingestv1.Platform_PLATFORM_TEAMS
	case "teams":
		return ingestv1.Platform_PLATFORM_TEAMS
	case "zoom":
		return ingestv1.Platform_PLATFORM_ZOOM
	case "google_meet", "googlemeet", "meet":
		return ingestv1.Platform_PLATFORM_GOOGLE_MEET
	case "local":
		return ingestv1.Platform_PLATFORM_LOCAL
	default:
		return ingestv1.Platform_PLATFORM_UNSPECIFIED
	}
}

// processMeetingViaGRPC processes a single meeting and sends it to the gateway via gRPC.
func processMeetingViaGRPC(ctx context.Context, client ingestv1.IngestServiceClient, m *meeting.Meeting, tenantID, sourceTag, platform string, contentID string) (*ingestv1.IngestMeetingResponse, error) {
	// Resolve tenant ID
	resolvedTenantID := tenantID
	if resolvedTenantID == "" || resolvedTenantID == "default" {
		resolvedTenantID = DefaultTenantID
	}

	// Parse transcript if available (CLI keeps file parsing)
	if m.Files.TranscriptPath != "" {
		f, err := os.Open(m.Files.TranscriptPath)
		if err != nil {
			return nil, fmt.Errorf("opening transcript: %w", err)
		}
		defer f.Close()

		var transcriptResult *meeting.TranscriptResult
		// Detect format and parse
		if strings.HasSuffix(strings.ToLower(m.Files.TranscriptPath), ".vtt") {
			transcriptResult, err = meeting.ParseVTT(f)
		} else {
			transcriptResult, err = meeting.ParseTXTTranscript(f)
		}
		if err != nil {
			return nil, fmt.Errorf("parsing transcript: %w", err)
		}
		m.Transcript = transcriptResult
		m.Participants = transcriptResult.Speakers
		if m.DurationSeconds == 0 {
			m.DurationSeconds = transcriptResult.DurationSeconds
		}
	}

	// Parse chat if available (CLI keeps file parsing)
	if m.Files.ChatPath != "" {
		f, err := os.Open(m.Files.ChatPath)
		if err != nil {
			return nil, fmt.Errorf("opening chat: %w", err)
		}
		defer f.Close()

		chatResult, err := meeting.ParseChatLog(f)
		if err != nil {
			return nil, fmt.Errorf("parsing chat: %w", err)
		}
		m.Chat = chatResult

		// Merge chat participants with transcript participants
		for _, speaker := range chatResult.Speakers {
			found := false
			for _, p := range m.Participants {
				if p == speaker {
					found = true
					break
				}
			}
			if !found {
				m.Participants = append(m.Participants, speaker)
			}
		}
	}

	// Convert parsed meeting to proto request
	req := meetingToProtoRequest(m, resolvedTenantID, sourceTag, platform, contentID, meetingSeries, meetingTitle, meetingDate)

	// Call gRPC service
	resp, err := client.IngestMeeting(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("calling IngestMeeting: %w", err)
	}

	return resp, nil
}

// generateMeetingID creates a deterministic external meeting ID from meeting metadata.
// Format: {platform}-{date}-{title-slug}
// Example: teams-20251118-weekly-standup
func generateMeetingID(m *meeting.Meeting, platform string) string {
	var dateStr string
	if !m.Date.IsZero() {
		dateStr = m.Date.Format("20060102")
	} else {
		// Fallback: use hash of title if date is missing
		dateStr = fmt.Sprintf("%x", strings.ToLower(m.Title))
		if len(dateStr) > 8 {
			dateStr = dateStr[:8]
		}
	}

	// Slugify title: lowercase, replace non-alphanumeric with hyphens
	titleSlug := strings.ToLower(m.Title)
	titleSlug = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return '-'
	}, titleSlug)
	// Collapse multiple hyphens and trim
	titleSlug = strings.Trim(titleSlug, "-")
	for strings.Contains(titleSlug, "--") {
		titleSlug = strings.ReplaceAll(titleSlug, "--", "-")
	}
	// Truncate if too long
	if len(titleSlug) > 50 {
		titleSlug = titleSlug[:50]
	}

	return fmt.Sprintf("%s-%s-%s", platform, dateStr, titleSlug)
}

// meetingToProtoRequest converts a parsed meeting to a proto IngestMeetingRequest.
func meetingToProtoRequest(m *meeting.Meeting, tenantID, sourceTag, platform string, contentID string, seriesName, titleOverride, dateOverride string) *ingestv1.IngestMeetingRequest {
	// Use title override if provided
	title := m.Title
	if titleOverride != "" {
		title = titleOverride
	}

	req := &ingestv1.IngestMeetingRequest{
		TenantId:          tenantID,
		ExternalMeetingId: generateMeetingID(m, platform),
		Title:             title,
		Platform:          platformToProto(platform),
		Labels:            []string{sourceTag},
		ContentId:         contentID,
		SeriesName:        seriesName,
		TitleOverride:     titleOverride,
		DateOverride:      dateOverride,
	}

	// Set meeting times
	if !m.Date.IsZero() {
		req.ActualStart = timestamppb.New(m.Date)
		// Estimate end time based on duration
		if m.DurationSeconds > 0 {
			endTime := m.Date.Add(time.Duration(m.DurationSeconds) * time.Second)
			req.ActualEnd = timestamppb.New(endTime)
		}
	}

	// Convert participants
	for _, name := range m.Participants {
		req.Participants = append(req.Participants, &ingestv1.MeetingParticipant{
			Name:     name,
			Attended: true,
		})
	}

	// Convert transcript segments
	if m.Transcript != nil {
		for _, seg := range m.Transcript.Segments {
			req.Transcript = append(req.Transcript, &ingestv1.TranscriptSegment{
				Speaker:      seg.Speaker,
				Text:         seg.Text,
				StartSeconds: float64(seg.StartMs) / 1000.0,
				EndSeconds:   float64(seg.EndMs) / 1000.0,
			})
		}
	}

	// Convert chat messages
	if m.Chat != nil {
		for _, msg := range m.Chat.Messages {
			chatMsg := &ingestv1.ChatMessage{
				Sender: msg.Speaker,
				Text:   msg.Message,
			}
			if !msg.Timestamp.IsZero() {
				chatMsg.Timestamp = timestamppb.New(msg.Timestamp)
			}
			req.ChatMessages = append(req.ChatMessages, chatMsg)
		}
	}

	return req
}

// runResolveMeetingParticipants resolves meeting participants to people.
// NOTE: This still uses direct database access as there is no gRPC service for this operation yet.
func runResolveMeetingParticipants(ctx context.Context, deps *IngestCommandDeps) error {
	// Load configuration
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Determine tenant ID
	tenantID := ingestTenantID
	if tenantID == "" {
		tenantID = cfg.TenantID
	}
	if tenantID == "" || tenantID == "default" {
		tenantID = DefaultTenantID
	}

	fmt.Printf("Resolving Meeting Participants\n")
	fmt.Printf("  Tenant: %s\n\n", tenantID)

	// Initialize database connection
	pool, err := connectMeetingToDatabase(ctx, cfg)
	if err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}
	defer pool.Close()

	// Load all people for resolution
	fmt.Println("Loading people from database...")
	people, err := loadPeople(ctx, pool, tenantID)
	if err != nil {
		return fmt.Errorf("loading people: %w", err)
	}
	fmt.Printf("  Found %d people\n\n", len(people))

	if len(people) == 0 {
		fmt.Println("No people found. Add people to the database first.")
		return nil
	}

	// Create resolver
	resolver := meeting.NewParticipantResolver(people)

	// Load meetings with participants
	fmt.Println("Loading meetings...")
	var sourceFilter string
	if meetingSource != "" {
		sourceFilter = " AND source_tag = $2"
	}

	query := `SELECT id, title, participants FROM meetings WHERE tenant_id = $1` + sourceFilter + ` ORDER BY id`
	var rows interface{ Close() }
	var meetingRows interface {
		Next() bool
		Scan(dest ...interface{}) error
		Err() error
	}

	if meetingSource != "" {
		r, err := pool.Query(ctx, query, tenantID, meetingSource)
		if err != nil {
			return fmt.Errorf("querying meetings: %w", err)
		}
		rows = r
		meetingRows = r
	} else {
		r, err := pool.Query(ctx, query, tenantID)
		if err != nil {
			return fmt.Errorf("querying meetings: %w", err)
		}
		rows = r
		meetingRows = r
	}
	defer rows.Close()

	var totalMeetings, totalParticipants, totalMatched int

	for meetingRows.Next() {
		var meetingID int64
		var title string
		var participantsJSON []byte

		if err := meetingRows.Scan(&meetingID, &title, &participantsJSON); err != nil {
			return fmt.Errorf("scanning meeting: %w", err)
		}

		// Parse participants JSON array
		var participants []string
		if len(participantsJSON) > 0 {
			// Simple JSON array parsing
			participantsStr := string(participantsJSON)
			participantsStr = strings.TrimPrefix(participantsStr, "[")
			participantsStr = strings.TrimSuffix(participantsStr, "]")
			if participantsStr != "" {
				for _, p := range strings.Split(participantsStr, ",") {
					p = strings.TrimSpace(p)
					p = strings.Trim(p, "\"")
					if p != "" {
						participants = append(participants, p)
					}
				}
			}
		}

		if len(participants) == 0 {
			continue
		}

		totalMeetings++

		// Resolve participants
		results := resolver.ResolveAll(participants)
		stats := results.Stats()

		totalParticipants += stats.Total
		totalMatched += stats.Matched

		// Insert into meeting_participants table
		for _, result := range results {
			var personID *int64
			var matchType *string
			var confidence *float64

			if result.Match != nil {
				personID = &result.Match.PersonID
				mt := string(result.Match.MatchType)
				matchType = &mt
				confidence = &result.Match.Confidence
			}

			_, err := pool.Exec(ctx, `
				INSERT INTO meeting_participants (
					tenant_id, meeting_id, person_id, display_name, match_type, confidence
				) VALUES ($1, $2, $3, $4, $5, $6)
				ON CONFLICT (meeting_id, display_name) DO UPDATE SET
					person_id = EXCLUDED.person_id,
					match_type = EXCLUDED.match_type,
					confidence = EXCLUDED.confidence,
					updated_at = NOW()
			`, tenantID, meetingID, personID, result.DisplayName, matchType, confidence)

			if err != nil {
				return fmt.Errorf("inserting participant %s for meeting %d: %w", result.DisplayName, meetingID, err)
			}
		}

		fmt.Printf("  [%d] %s: %d/%d matched\n", meetingID, truncateMeetingTitle(title, 40), stats.Matched, stats.Total)
	}

	if err := meetingRows.Err(); err != nil {
		return fmt.Errorf("iterating meetings: %w", err)
	}

	// Print summary
	fmt.Println()
	fmt.Println("Resolution Complete")
	fmt.Println(strings.Repeat("=", 50))
	fmt.Printf("  Meetings:     %d\n", totalMeetings)
	fmt.Printf("  Participants: %d\n", totalParticipants)
	fmt.Printf("  Matched:      \033[32m%d\033[0m\n", totalMatched)
	fmt.Printf("  Unmatched:    \033[33m%d\033[0m\n", totalParticipants-totalMatched)
	if totalParticipants > 0 {
		fmt.Printf("  Match Rate:   %.1f%%\n", float64(totalMatched)/float64(totalParticipants)*100)
	}

	return nil
}

// connectMeetingToDatabase establishes a database connection for meeting-related operations.
// NOTE: This is used by resolve and mentions subcommands that still use direct DB access.
func connectMeetingToDatabase(ctx context.Context, cfg *config.CLIConfig) (*pgxpool.Pool, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable is required for this operation")
	}

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		return nil, fmt.Errorf("creating database pool: %w", err)
	}

	// Test the connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	return pool, nil
}

// loadPeople loads all people from the database for entity resolution.
func loadPeople(ctx context.Context, pool *pgxpool.Pool, tenantID string) ([]meeting.Person, error) {
	rows, err := pool.Query(ctx, `
		SELECT id, canonical_name, aliases
		FROM people
		WHERE tenant_id = $1 AND (is_deleted = false OR is_deleted IS NULL)
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var people []meeting.Person
	for rows.Next() {
		var id int64
		var canonicalName string
		var aliases []string

		if err := rows.Scan(&id, &canonicalName, &aliases); err != nil {
			return nil, err
		}

		people = append(people, meeting.Person{
			ID:            id,
			CanonicalName: canonicalName,
			Aliases:       aliases,
		})
	}

	return people, rows.Err()
}

// truncateMeetingTitle truncates a meeting title to maxLen characters.
func truncateMeetingTitle(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// newExtractMeetingMentionsCommand creates the 'ingest meeting mentions' subcommand.
func newExtractMeetingMentionsCommand(deps *IngestCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "mentions",
		Short: "Extract mentions of people from meeting transcripts",
		Long: `Extract mentions of known people from meeting transcript content.

This identifies people who were discussed/mentioned in meetings (distinct from
attendees who spoke). Useful for queries like "meetings where Rishi was mentioned".

Attendees are excluded from mention extraction to avoid false positives.

Examples:
  # Extract mentions from all meetings
  penf ingest meeting mentions`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExtractMeetingMentions(cmd.Context(), deps)
		},
	}
}

// runExtractMeetingMentions extracts mentions of people from meeting transcripts.
// NOTE: This still uses direct database access as there is no gRPC service for this operation yet.
func runExtractMeetingMentions(ctx context.Context, deps *IngestCommandDeps) error {
	// Load configuration
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	// Determine tenant ID
	tenantID := ingestTenantID
	if tenantID == "" {
		tenantID = cfg.TenantID
	}
	if tenantID == "" || tenantID == "default" {
		tenantID = DefaultTenantID
	}

	fmt.Printf("Extracting Meeting Mentions\n")
	fmt.Printf("  Tenant: %s\n\n", tenantID)

	// Initialize database connection
	pool, err := connectMeetingToDatabase(ctx, cfg)
	if err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}
	defer pool.Close()

	// Load all people for extraction
	fmt.Println("Loading people from database...")
	people, err := loadPeople(ctx, pool, tenantID)
	if err != nil {
		return fmt.Errorf("loading people: %w", err)
	}
	fmt.Printf("  Found %d people\n\n", len(people))

	if len(people) == 0 {
		fmt.Println("No people found. Add people to the database first.")
		return nil
	}

	// Create mention extractor
	extractor := meeting.NewMentionExtractor(people)

	// Load meetings with transcript sources
	fmt.Println("Processing meeting transcripts...")
	rows, err := pool.Query(ctx, `
		SELECT m.id, m.title, s.id as source_id, s.raw_content
		FROM meetings m
		JOIN sources s ON s.meeting_id = m.id
		WHERE m.tenant_id = $1
		  AND s.source_system = 'meeting_transcript'
		  AND s.raw_content IS NOT NULL
		ORDER BY m.id
	`, tenantID)
	if err != nil {
		return fmt.Errorf("querying meetings: %w", err)
	}
	defer rows.Close()

	var totalMeetings, totalMentions int

	for rows.Next() {
		var meetingID, sourceID int64
		var title, rawContent string

		if err := rows.Scan(&meetingID, &title, &sourceID, &rawContent); err != nil {
			return fmt.Errorf("scanning meeting: %w", err)
		}

		// Get attendee IDs to exclude from mentions
		attendeeIDs, err := getAttendeeIDs(ctx, pool, meetingID)
		if err != nil {
			return fmt.Errorf("getting attendees for meeting %d: %w", meetingID, err)
		}

		// Extract mentions (excluding attendees)
		mentions := extractor.ExtractExcluding(rawContent, attendeeIDs)

		if len(mentions) == 0 {
			continue
		}

		totalMeetings++
		totalMentions += len(mentions)

		// Insert mentions into database
		for _, mention := range mentions {
			_, err := pool.Exec(ctx, `
				INSERT INTO meeting_mentions (
					tenant_id, meeting_id, source_id, person_id,
					matched_text, match_type, context, mention_count
				) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
				ON CONFLICT (meeting_id, person_id) DO UPDATE SET
					matched_text = EXCLUDED.matched_text,
					match_type = EXCLUDED.match_type,
					context = EXCLUDED.context,
					mention_count = meeting_mentions.mention_count + EXCLUDED.mention_count,
					updated_at = NOW()
			`, tenantID, meetingID, sourceID, mention.PersonID,
				mention.MatchedText, string(mention.MatchType), mention.Context, mention.Count)

			if err != nil {
				return fmt.Errorf("inserting mention for meeting %d: %w", meetingID, err)
			}
		}

		// Show progress
		mentionNames := make([]string, len(mentions))
		for i, m := range mentions {
			mentionNames[i] = m.CanonicalName
		}
		fmt.Printf("  [%d] %s: %d mentions (%s)\n",
			meetingID, truncateMeetingTitle(title, 35), len(mentions), strings.Join(mentionNames, ", "))
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterating meetings: %w", err)
	}

	// Print summary
	fmt.Println()
	fmt.Println("Mention Extraction Complete")
	fmt.Println(strings.Repeat("=", 50))
	fmt.Printf("  Meetings with mentions: %d\n", totalMeetings)
	fmt.Printf("  Total mentions:         %d\n", totalMentions)

	return nil
}

// getAttendeeIDs returns the person IDs of attendees for a meeting.
func getAttendeeIDs(ctx context.Context, pool *pgxpool.Pool, meetingID int64) (map[int64]bool, error) {
	rows, err := pool.Query(ctx, `
		SELECT person_id FROM meeting_participants
		WHERE meeting_id = $1 AND person_id IS NOT NULL
	`, meetingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := make(map[int64]bool)
	for rows.Next() {
		var personID int64
		if err := rows.Scan(&personID); err != nil {
			return nil, err
		}
		ids[personID] = true
	}

	return ids, rows.Err()
}
