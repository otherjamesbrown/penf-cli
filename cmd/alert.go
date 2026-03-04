package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	alertv1 "github.com/otherjamesbrown/penf-cli/api/proto/alert/v1"
	"github.com/otherjamesbrown/penf-cli/config"
)

var (
	alertOutput   string
	alertProject  string
	alertUnacked  bool
	alertSeverity string
	alertLimit    int
)

// AlertCommandDeps holds dependencies for alert commands.
type AlertCommandDeps struct {
	Config     *config.CLIConfig
	LoadConfig func() (*config.CLIConfig, error)
}

// DefaultAlertDeps returns the default dependencies.
func DefaultAlertDeps() *AlertCommandDeps {
	return &AlertCommandDeps{
		LoadConfig: config.LoadConfig,
	}
}

// NewAlertCommand creates the root alert command with all subcommands.
func NewAlertCommand(deps *AlertCommandDeps) *cobra.Command {
	if deps == nil {
		deps = DefaultAlertDeps()
	}

	cmd := &cobra.Command{
		Use:     "alert",
		Short:   "Manage alerts — instruction-triggered notifications",
		Aliases: []string{"alerts"},
	}

	cmd.PersistentFlags().StringVarP(&alertOutput, "output", "o", "", "Output format: text, json, yaml")

	cmd.AddCommand(newAlertListCommand(deps))
	cmd.AddCommand(newAlertAckCommand(deps))

	return cmd
}

// ==================== List ====================

func newAlertListCommand(deps *AlertCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List alerts for a project",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAlertList(cmd.Context(), deps)
		},
	}

	cmd.Flags().StringVar(&alertProject, "project", "", "Project name")
	cmd.Flags().BoolVar(&alertUnacked, "unacked", false, "Show only unacknowledged alerts")
	cmd.Flags().StringVar(&alertSeverity, "severity", "", "Filter by severity: low, medium, high, critical")
	cmd.Flags().IntVar(&alertLimit, "limit", 20, "Maximum results")

	return cmd
}

func runAlertList(ctx context.Context, deps *AlertCommandDeps) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectToGateway(cfg)
	if err != nil {
		return fmt.Errorf("connecting to gateway: %w", err)
	}
	defer conn.Close()

	client := alertv1.NewAlertServiceClient(conn)

	tenantID, err := resolveTenantID(deps.Config)
	if err != nil {
		return err
	}

	resp, err := client.ListAlerts(ctx, &alertv1.ListAlertsRequest{
		TenantId:    tenantID,
		ProjectName: alertProject,
		UnackedOnly: alertUnacked,
		Severity:    alertSeverity,
		Limit:       int32(alertLimit),
	})
	if err != nil {
		return fmt.Errorf("listing alerts: %w", err)
	}

	format := resolveOutputFormat(alertOutput, cfg)

	switch format {
	case config.OutputFormatJSON:
		items := make([]map[string]interface{}, len(resp.Alerts))
		for i, a := range resp.Alerts {
			items[i] = map[string]interface{}{
				"id":               a.Id,
				"project_id":       a.ProjectId,
				"project_name":     a.ProjectName,
				"instruction_id":   a.InstructionId,
				"instruction_name": a.InstructionName,
				"severity":         a.Severity,
				"title":            a.Title,
				"body":             a.Body,
				"acknowledged":     a.Acknowledged,
				"created_at":       a.CreatedAt,
			}
		}
		data, _ := json.MarshalIndent(items, "", "  ")
		fmt.Println(string(data))
	case config.OutputFormatYAML:
		items := make([]map[string]interface{}, len(resp.Alerts))
		for i, a := range resp.Alerts {
			items[i] = map[string]interface{}{
				"id":           a.Id,
				"severity":     a.Severity,
				"title":        a.Title,
				"acknowledged": a.Acknowledged,
				"created_at":   a.CreatedAt,
			}
		}
		data, _ := yaml.Marshal(items)
		fmt.Print(string(data))
	default:
		if len(resp.Alerts) == 0 {
			fmt.Println("No alerts found.")
			return nil
		}
		fmt.Printf("%-16s %-8s %-5s %-40s %s\n", "ID", "SEVERITY", "ACK", "TITLE", "CREATED")
		for _, a := range resp.Alerts {
			ack := " "
			if a.Acknowledged {
				ack = "Y"
			}
			fmt.Printf("%-16s %-8s %-5s %-40s %s\n",
				truncateString(a.Id, 16),
				a.Severity,
				ack,
				truncateString(a.Title, 40),
				a.CreatedAt,
			)
		}
	}
	return nil
}

// ==================== Ack ====================

func newAlertAckCommand(deps *AlertCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "ack <alert-id>",
		Short: "Acknowledge an alert",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAlertAck(cmd.Context(), deps, args[0])
		},
	}
}

func runAlertAck(ctx context.Context, deps *AlertCommandDeps, alertID string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectToGateway(cfg)
	if err != nil {
		return fmt.Errorf("connecting to gateway: %w", err)
	}
	defer conn.Close()

	client := alertv1.NewAlertServiceClient(conn)

	tenantID, err := resolveTenantID(deps.Config)
	if err != nil {
		return err
	}

	_, err = client.AcknowledgeAlert(ctx, &alertv1.AcknowledgeAlertRequest{
		TenantId: tenantID,
		AlertId:  alertID,
	})
	if err != nil {
		return fmt.Errorf("acknowledging alert: %w", err)
	}

	fmt.Printf("Acknowledged alert: %s\n", alertID)
	return nil
}
