// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	productv1 "github.com/otherjamesbrown/penf-cli/api/proto/product/v1"
	"github.com/otherjamesbrown/penf-cli/config"
)

// Product query command flags.
var (
	productQueryOutput string
)

// addProductQueryCommands adds the query subcommand to the product command.
func addProductQueryCommands(parent *cobra.Command, deps *ProductCommandDeps) {
	// penf product query "<natural language query>"
	queryCmd := &cobra.Command{
		Use:   "query <question>",
		Short: "Ask natural language questions about products",
		Long: `Ask natural language questions about products, teams, and roles.

Supports various query patterns:
  - "who is the DRI for <product>"
  - "who is the DRI for networking on <product>"
  - "which teams work on <product>"
  - "show timeline for <product>"
  - "what are the sub-products of <product>"
  - General keyword search for products

Examples:
  # Find the DRI for a product
  penf product query "who is the DRI for MTC"

  # Find DRI for a specific scope
  penf product query "who is the DRI for networking on MTC"

  # Find teams working on a product
  penf product query "which teams work on LKE Enterprise"

  # Show product timeline
  penf product query "show timeline for API Gateway"

  # Show product hierarchy
  penf product query "what are the sub-products of Cloud Platform"

  # Search for products
  penf product query "kubernetes"

JSON Output (for AI processing):
  penf product query "who owns LKE" -o json

  Returns (varies by query type):
  {
    "type": "role",
    "message": "Jane Doe (jane@example.com) is the DRI for LKE",
    "persons": [{"person_id": 1, "name": "Jane Doe", "role": "DRI", ...}]
  }`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProductQuery(cmd.Context(), deps, strings.Join(args, " "))
		},
	}

	queryCmd.Flags().StringVarP(&productQueryOutput, "output", "o", "", "Output format: text, json, yaml")

	parent.AddCommand(queryCmd)
}

// runProductQuery executes a natural language product query.
func runProductQuery(ctx context.Context, deps *ProductCommandDeps, queryStr string) error {
	// Connect to Gateway
	conn, err := connectProductToGateway(deps.Config)
	if err != nil {
		return err
	}
	defer conn.Close()

	// Determine output format
	outputFormat := deps.Config.OutputFormat
	if productQueryOutput != "" {
		outputFormat = config.OutputFormat(productQueryOutput)
		if !outputFormat.IsValid() {
			return fmt.Errorf("invalid output format: %s (must be text, json, yaml)", productQueryOutput)
		}
	}

	// Create gRPC client
	client := productv1.NewProductServiceClient(conn)

	// Execute query via gRPC
	resp, err := client.QueryProducts(ctx, &productv1.QueryProductsRequest{
		TenantId: deps.Config.TenantID,
		Query:    queryStr,
	})
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	return outputQueryResultFromProto(outputFormat, resp, queryStr)
}

// ProductQueryResponse represents the JSON/YAML output for a query.
type ProductQueryResponse struct {
	Query     string           `json:"query" yaml:"query"`
	Type      string           `json:"type" yaml:"type"`
	Message   string           `json:"message" yaml:"message"`
	Products  []ProductSummary `json:"products,omitempty" yaml:"products,omitempty"`
	Persons   []PersonInfo     `json:"persons,omitempty" yaml:"persons,omitempty"`
	Teams     []TeamInfo       `json:"teams,omitempty" yaml:"teams,omitempty"`
	Events    []EventInfo      `json:"events,omitempty" yaml:"events,omitempty"`
	Hierarchy *HierarchyInfo   `json:"hierarchy,omitempty" yaml:"hierarchy,omitempty"`
	QueriedAt time.Time        `json:"queried_at" yaml:"queried_at"`
}

// ProductSummary provides product info for query results.
type ProductSummary struct {
	ID          int64    `json:"id" yaml:"id"`
	Name        string   `json:"name" yaml:"name"`
	Description string   `json:"description,omitempty" yaml:"description,omitempty"`
	ProductType string   `json:"product_type" yaml:"product_type"`
	Status      string   `json:"status" yaml:"status"`
	Keywords    []string `json:"keywords,omitempty" yaml:"keywords,omitempty"`
}

// PersonInfo provides person info for role queries.
type PersonInfo struct {
	PersonID    int64  `json:"person_id" yaml:"person_id"`
	Name        string `json:"name" yaml:"name"`
	Email       string `json:"email" yaml:"email"`
	Role        string `json:"role" yaml:"role"`
	Scope       string `json:"scope,omitempty" yaml:"scope,omitempty"`
	TeamName    string `json:"team_name" yaml:"team_name"`
	TeamContext string `json:"team_context,omitempty" yaml:"team_context,omitempty"`
}

// TeamInfo provides team info for team queries.
type TeamInfo struct {
	TeamID   int64  `json:"team_id" yaml:"team_id"`
	TeamName string `json:"team_name" yaml:"team_name"`
	Context  string `json:"context,omitempty" yaml:"context,omitempty"`
}

// EventInfo provides event info for timeline queries.
type EventInfo struct {
	ID          int64     `json:"id" yaml:"id"`
	EventType   string    `json:"event_type" yaml:"event_type"`
	Visibility  string    `json:"visibility" yaml:"visibility"`
	Title       string    `json:"title" yaml:"title"`
	Description string    `json:"description,omitempty" yaml:"description,omitempty"`
	OccurredAt  time.Time `json:"occurred_at" yaml:"occurred_at"`
}

// HierarchyInfo provides hierarchy info.
type HierarchyInfo struct {
	ID          int64            `json:"id" yaml:"id"`
	Name        string           `json:"name" yaml:"name"`
	ProductType string           `json:"product_type" yaml:"product_type"`
	Children    []ProductSummary `json:"children,omitempty" yaml:"children,omitempty"`
}

// outputQueryResultFromProto formats and outputs the query result from proto response.
func outputQueryResultFromProto(format config.OutputFormat, result *productv1.QueryProductsResponse, queryStr string) error {
	// Build response
	response := ProductQueryResponse{
		Query:     queryStr,
		Type:      queryTypeFromProto(result.Type),
		Message:   result.Message,
		QueriedAt: time.Now(),
	}

	// Add type-specific data
	if len(result.Products) > 0 {
		for _, p := range result.Products {
			response.Products = append(response.Products, ProductSummary{
				ID:          p.Id,
				Name:        p.Name,
				Description: p.Description,
				ProductType: productTypeFromProtoStr(p.ProductType),
				Status:      productStatusFromProtoStr(p.Status),
				Keywords:    p.Keywords,
			})
		}
	}

	if len(result.Persons) > 0 {
		for _, p := range result.Persons {
			response.Persons = append(response.Persons, PersonInfo{
				PersonID:    p.PersonId,
				Name:        p.PersonName,
				Email:       p.PersonEmail,
				Role:        p.Role,
				Scope:       p.Scope,
				TeamName:    p.TeamName,
				TeamContext: p.TeamContext,
			})
		}
	}

	if len(result.Teams) > 0 {
		for _, t := range result.Teams {
			response.Teams = append(response.Teams, TeamInfo{
				TeamID:   t.TeamId,
				TeamName: t.TeamName,
				Context:  t.Context,
			})
		}
	}

	if len(result.Events) > 0 {
		for _, e := range result.Events {
			response.Events = append(response.Events, EventInfo{
				ID:          e.Id,
				EventType:   eventTypeFromProtoStr(e.EventType),
				Visibility:  eventVisibilityFromProtoStr(e.Visibility),
				Title:       e.Title,
				Description: e.Description,
				OccurredAt:  e.OccurredAt.AsTime(),
			})
		}
	}

	if len(result.Hierarchy) > 0 {
		h := result.Hierarchy[0]
		response.Hierarchy = &HierarchyInfo{
			ID:          h.Product.Id,
			Name:        h.Product.Name,
			ProductType: productTypeFromProtoStr(h.Product.ProductType),
		}
	}

	// Output based on format
	switch format {
	case config.OutputFormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(response)
	case config.OutputFormatYAML:
		enc := yaml.NewEncoder(os.Stdout)
		return enc.Encode(response)
	default:
		return outputQueryResultTextFromProto(response, result)
	}
}

// outputQueryResultTextFromProto formats query results for terminal display from proto response.
func outputQueryResultTextFromProto(response ProductQueryResponse, result *productv1.QueryProductsResponse) error {
	// Print the answer
	fmt.Printf("\n%s\n\n", response.Message)

	// Print additional details based on query type
	switch result.Type {
	case productv1.QueryType_QUERY_TYPE_ROLE:
		if len(result.Persons) > 0 {
			fmt.Println("Details:")
			for _, p := range result.Persons {
				fmt.Printf("  • %s (%s)\n", p.PersonName, p.PersonEmail)
				fmt.Printf("    Role: %s", p.Role)
				if p.Scope != "" {
					fmt.Printf(" (%s)", p.Scope)
				}
				fmt.Println()
				fmt.Printf("    Team: %s", p.TeamName)
				if p.TeamContext != "" {
					fmt.Printf(" [%s]", p.TeamContext)
				}
				fmt.Println()
			}
		}

	case productv1.QueryType_QUERY_TYPE_TEAM:
		if len(result.Teams) > 0 {
			fmt.Println("Team Details:")
			for _, t := range result.Teams {
				fmt.Printf("  • %s", t.TeamName)
				if t.Context != "" {
					fmt.Printf(" (%s)", t.Context)
				}
				fmt.Println()
			}
		}

	case productv1.QueryType_QUERY_TYPE_TIMELINE:
		if len(result.Events) > 0 {
			fmt.Println("Recent Events:")
			fmt.Println("  DATE        TYPE         TITLE")
			for _, e := range result.Events {
				fmt.Printf("  %s  %-11s  %s\n",
					e.OccurredAt.AsTime().Format("2006-01-02"),
					eventTypeFromProtoStr(e.EventType),
					truncateString(e.Title, 50))
			}
		}

	case productv1.QueryType_QUERY_TYPE_HIERARCHY:
		if len(result.Hierarchy) > 0 {
			h := result.Hierarchy[0]
			fmt.Printf("Product: %s (%s)\n", h.Product.Name, productTypeFromProtoStr(h.Product.ProductType))
		}

	case productv1.QueryType_QUERY_TYPE_SEARCH:
		if len(result.Products) > 0 {
			fmt.Println("Products:")
			for _, p := range result.Products {
				fmt.Printf("  • %s", p.Name)
				pt := productTypeFromProtoStr(p.ProductType)
				if pt != "product" {
					fmt.Printf(" [%s]", pt)
				}
				fmt.Println()
				if p.Description != "" {
					fmt.Printf("    %s\n", truncateString(p.Description, 60))
				}
			}
		}
	}

	return nil
}

// Query type and enum conversion helpers

func queryTypeFromProto(qt productv1.QueryType) string {
	switch qt {
	case productv1.QueryType_QUERY_TYPE_SEARCH:
		return "search"
	case productv1.QueryType_QUERY_TYPE_ROLE:
		return "role"
	case productv1.QueryType_QUERY_TYPE_TEAM:
		return "team"
	case productv1.QueryType_QUERY_TYPE_TIMELINE:
		return "timeline"
	case productv1.QueryType_QUERY_TYPE_HIERARCHY:
		return "hierarchy"
	default:
		return "unknown"
	}
}

func productTypeFromProtoStr(pt productv1.ProductType) string {
	switch pt {
	case productv1.ProductType_PRODUCT_TYPE_PRODUCT:
		return "product"
	case productv1.ProductType_PRODUCT_TYPE_SUB_PRODUCT:
		return "sub_product"
	case productv1.ProductType_PRODUCT_TYPE_FEATURE:
		return "feature"
	default:
		return "product"
	}
}

func productStatusFromProtoStr(ps productv1.ProductStatus) string {
	switch ps {
	case productv1.ProductStatus_PRODUCT_STATUS_ACTIVE:
		return "active"
	case productv1.ProductStatus_PRODUCT_STATUS_BETA:
		return "beta"
	case productv1.ProductStatus_PRODUCT_STATUS_SUNSET:
		return "sunset"
	case productv1.ProductStatus_PRODUCT_STATUS_DEPRECATED:
		return "deprecated"
	default:
		return "active"
	}
}

func eventTypeFromProtoStr(et productv1.EventType) string {
	switch et {
	case productv1.EventType_EVENT_TYPE_DECISION:
		return "decision"
	case productv1.EventType_EVENT_TYPE_MILESTONE:
		return "milestone"
	case productv1.EventType_EVENT_TYPE_RISK:
		return "risk"
	case productv1.EventType_EVENT_TYPE_RELEASE:
		return "release"
	case productv1.EventType_EVENT_TYPE_COMPETITOR:
		return "competitor"
	case productv1.EventType_EVENT_TYPE_ORG_CHANGE:
		return "org_change"
	case productv1.EventType_EVENT_TYPE_MARKET:
		return "market"
	case productv1.EventType_EVENT_TYPE_NOTE:
		return "note"
	default:
		return "note"
	}
}

func eventVisibilityFromProtoStr(ev productv1.EventVisibility) string {
	switch ev {
	case productv1.EventVisibility_EVENT_VISIBILITY_INTERNAL:
		return "internal"
	case productv1.EventVisibility_EVENT_VISIBILITY_EXTERNAL:
		return "external"
	default:
		return "internal"
	}
}
