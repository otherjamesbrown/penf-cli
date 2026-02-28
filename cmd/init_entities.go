// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	entityv1 "github.com/otherjamesbrown/penf-cli/api/proto/entity/v1"
	glossaryv1 "github.com/otherjamesbrown/penf-cli/api/proto/glossary/v1"
	"github.com/otherjamesbrown/penf-cli/client"
	"github.com/otherjamesbrown/penf-cli/config"
)

var (
	initEntitiesFromJSON string
)

// EntitiesSeedData represents the JSON structure for bulk entity import.
type EntitiesSeedData struct {
	People   []PersonSeed   `json:"people"`
	Products []ProductSeed  `json:"products"`
	Projects []ProjectSeed  `json:"projects"`
	Glossary []GlossarySeed `json:"glossary"`
}

// PersonSeed represents a person to seed.
type PersonSeed struct {
	Name       string   `json:"name"`
	Email      string   `json:"email"`
	Company    string   `json:"company,omitempty"`
	Title      string   `json:"title,omitempty"`
	Department string   `json:"department,omitempty"`
	Aliases    []string `json:"aliases,omitempty"`
	IsInternal bool     `json:"is_internal,omitempty"`
}

// ProductSeed represents a product to seed.
type ProductSeed struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Type        string   `json:"type,omitempty"` // product, sub_product, feature
	Parent      string   `json:"parent,omitempty"`
	Aliases     []string `json:"aliases,omitempty"`
	Status      string   `json:"status,omitempty"` // active, beta, sunset, deprecated
}

// ProjectSeed represents a project to seed.
type ProjectSeed struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Status      string   `json:"status,omitempty"` // planning, active, on_hold, completed
	Keywords    []string `json:"keywords,omitempty"`
}

// GlossarySeed represents a glossary term to seed.
type GlossarySeed struct {
	Term       string   `json:"term"`
	Expansion  string   `json:"expansion"`
	Definition string   `json:"definition,omitempty"`
	Context    []string `json:"context,omitempty"`
	Aliases    []string `json:"aliases,omitempty"`
}

// NewInitEntitiesCommand creates the 'init entities' subcommand.
func NewInitEntitiesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "entities",
		Short: "Seed known entities before importing content",
		Long: `Seed known entities (people, products, projects, glossary) before importing content.

This helps Penfold match mentions to the correct entities during processing,
reducing the number of items that need human review after import.

Interactive mode (default):
  Walks you through adding entities one by one.

JSON import mode:
  Bulk import from a JSON file with --from-json.

Example JSON format:
{
  "people": [
    {"name": "John Smith", "email": "john@company.com", "company": "Acme"}
  ],
  "products": [
    {"name": "DBaaS", "description": "Database as a Service"}
  ],
  "projects": [
    {"name": "MTC", "description": "Major TikTok Contract", "keywords": ["TikTok"]}
  ],
  "glossary": [
    {"term": "TER", "expansion": "Technical Execution Review", "context": ["meetings"]}
  ]
}

Entity seeding workflow details are in Context Palace knowledge shards.`,
		RunE: runInitEntities,
	}

	cmd.Flags().StringVar(&initEntitiesFromJSON, "from-json", "", "Import entities from JSON file")

	return cmd
}

func runInitEntities(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w (run 'penf init' first)", err)
	}

	if initEntitiesFromJSON != "" {
		return runInitEntitiesFromJSON(cmd.Context(), cfg, initEntitiesFromJSON)
	}

	return runInitEntitiesInteractive(cmd.Context(), cfg)
}

func runInitEntitiesFromJSON(ctx context.Context, cfg *config.CLIConfig, jsonPath string) error {
	// Read JSON file
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return fmt.Errorf("reading JSON file: %w", err)
	}

	var seed EntitiesSeedData
	if err := json.Unmarshal(data, &seed); err != nil {
		return fmt.Errorf("parsing JSON: %w", err)
	}

	fmt.Println("Entity Import from JSON")
	fmt.Println("=======================")
	fmt.Println()
	fmt.Printf("Found in %s:\n", jsonPath)
	fmt.Printf("  People:   %d\n", len(seed.People))
	fmt.Printf("  Products: %d\n", len(seed.Products))
	fmt.Printf("  Projects: %d\n", len(seed.Projects))
	fmt.Printf("  Glossary: %d\n", len(seed.Glossary))
	fmt.Println()

	// Connect to gateway
	conn, err := connectToInitGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	var stats struct {
		people   int
		products int
		projects int
		glossary int
		errors   []string
	}

	// Seed glossary (we have gRPC support for this)
	if len(seed.Glossary) > 0 {
		fmt.Println("Seeding glossary terms...")
		glossaryClient := glossaryv1.NewGlossaryServiceClient(conn)
		tenantID := getTenantID()

		for _, g := range seed.Glossary {
			_, err := glossaryClient.AddTerm(ctx, &glossaryv1.AddTermRequest{
				TenantId:       tenantID,
				Term:           g.Term,
				Expansion:      g.Expansion,
				Definition:     g.Definition,
				Context:        g.Context,
				Aliases:        g.Aliases,
				ExpandInSearch: true,
			})
			if err != nil {
				// Check if it's a duplicate error
				if strings.Contains(err.Error(), "already exists") {
					fmt.Printf("  \033[33m⚠\033[0m %s (already exists)\n", g.Term)
				} else {
					stats.errors = append(stats.errors, fmt.Sprintf("glossary %s: %v", g.Term, err))
				}
			} else {
				fmt.Printf("  \033[32m✓\033[0m %s = %s\n", g.Term, g.Expansion)
				stats.glossary++
			}
		}
		fmt.Println()
	}

	// Create entity client for people, products, projects
	entityClient := entityv1.NewEntityServiceClient(conn)

	// Seed people
	if len(seed.People) > 0 {
		fmt.Println("Seeding people...")
		peopleInput := make([]*entityv1.PersonInput, len(seed.People))
		for i, p := range seed.People {
			peopleInput[i] = &entityv1.PersonInput{
				Name:       p.Name,
				Email:      p.Email,
				Company:    p.Company,
				Title:      p.Title,
				Department: p.Department,
				Aliases:    p.Aliases,
				IsInternal: p.IsInternal,
			}
		}

		resp, err := entityClient.BulkCreatePeople(ctx, &entityv1.BulkCreatePeopleRequest{
			TenantId:       cfg.TenantID,
			People:         peopleInput,
			SkipDuplicates: true,
		})
		if err != nil {
			stats.errors = append(stats.errors, fmt.Sprintf("people: %v", err))
		} else {
			stats.people = int(resp.TotalCreated)
			for _, p := range resp.Created {
				fmt.Printf("  \033[32m✓\033[0m %s <%s>\n", p.Name, p.Email)
			}
			for _, s := range resp.Skipped {
				fmt.Printf("  \033[33m⚠\033[0m %s (already exists)\n", s.Email)
			}
			for _, e := range resp.Errors {
				stats.errors = append(stats.errors, fmt.Sprintf("person %s: %s", e.Identifier, e.Error))
			}
		}
		fmt.Println()
	}

	// Seed products
	if len(seed.Products) > 0 {
		fmt.Println("Seeding products...")
		productsInput := make([]*entityv1.ProductInput, len(seed.Products))
		for i, p := range seed.Products {
			productsInput[i] = &entityv1.ProductInput{
				Name:        p.Name,
				Description: p.Description,
				ProductType: p.Type,
				ParentName:  p.Parent,
				Aliases:     p.Aliases,
				Status:      p.Status,
			}
		}

		resp, err := entityClient.BulkCreateProducts(ctx, &entityv1.BulkCreateProductsRequest{
			TenantId:       cfg.TenantID,
			Products:       productsInput,
			SkipDuplicates: true,
		})
		if err != nil {
			stats.errors = append(stats.errors, fmt.Sprintf("products: %v", err))
		} else {
			stats.products = int(resp.TotalCreated)
			for _, p := range resp.Created {
				fmt.Printf("  \033[32m✓\033[0m %s\n", p.Name)
			}
			for _, s := range resp.Skipped {
				fmt.Printf("  \033[33m⚠\033[0m %s (already exists)\n", s.Name)
			}
			for _, e := range resp.Errors {
				stats.errors = append(stats.errors, fmt.Sprintf("product %s: %s", e.Identifier, e.Error))
			}
		}
		fmt.Println()
	}

	// Seed projects
	if len(seed.Projects) > 0 {
		fmt.Println("Seeding projects...")
		projectsInput := make([]*entityv1.ProjectInput, len(seed.Projects))
		for i, p := range seed.Projects {
			projectsInput[i] = &entityv1.ProjectInput{
				Name:        p.Name,
				Description: p.Description,
				Keywords:    p.Keywords,
			}
		}

		resp, err := entityClient.BulkCreateProjects(ctx, &entityv1.BulkCreateProjectsRequest{
			TenantId:       cfg.TenantID,
			Projects:       projectsInput,
			SkipDuplicates: true,
		})
		if err != nil {
			stats.errors = append(stats.errors, fmt.Sprintf("projects: %v", err))
		} else {
			stats.projects = int(resp.TotalCreated)
			for _, p := range resp.Created {
				fmt.Printf("  \033[32m✓\033[0m %s\n", p.Name)
			}
			for _, s := range resp.Skipped {
				fmt.Printf("  \033[33m⚠\033[0m %s (already exists)\n", s.Name)
			}
			for _, e := range resp.Errors {
				stats.errors = append(stats.errors, fmt.Sprintf("project %s: %s", e.Identifier, e.Error))
			}
		}
		fmt.Println()
	}

	// Summary
	fmt.Println("Import Summary")
	fmt.Println("--------------")
	fmt.Printf("  Glossary: %d added\n", stats.glossary)
	fmt.Printf("  People:   %d added\n", stats.people)
	fmt.Printf("  Products: %d added\n", stats.products)
	fmt.Printf("  Projects: %d added\n", stats.projects)

	if len(stats.errors) > 0 {
		fmt.Println()
		fmt.Printf("  \033[31mErrors: %d\033[0m\n", len(stats.errors))
		for _, e := range stats.errors {
			fmt.Printf("    - %s\n", e)
		}
	}

	fmt.Println()
	fmt.Println("Next: Run 'penf ingest email <files>' to import content")

	return nil
}

func runInitEntitiesInteractive(ctx context.Context, cfg *config.CLIConfig) error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("Entity Seeding Wizard")
	fmt.Println("=====================")
	fmt.Println()
	fmt.Println("Seed known entities before importing content.")
	fmt.Println("This helps Penfold match mentions correctly.")
	fmt.Println()

	// Connect to gateway
	conn, err := connectToInitGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	glossaryClient := glossaryv1.NewGlossaryServiceClient(conn)
	tenantID := getTenantID()

	var stats struct {
		people   int
		products int
		projects int
		glossary int
	}

	// 1. GLOSSARY
	fmt.Println("1. GLOSSARY")
	fmt.Println("   Add domain-specific acronyms and terms.")
	fmt.Println()

	for {
		fmt.Print("   Add term (or 'done' to continue): ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "" || strings.ToLower(input) == "done" {
			break
		}

		// Get expansion
		fmt.Print("   > Expansion: ")
		expansion, _ := reader.ReadString('\n')
		expansion = strings.TrimSpace(expansion)

		if expansion == "" {
			fmt.Println("   Expansion is required. Skipping.")
			continue
		}

		// Get optional context
		fmt.Print("   > Context tags (comma-separated, optional): ")
		contextInput, _ := reader.ReadString('\n')
		contextInput = strings.TrimSpace(contextInput)

		var contextTags []string
		if contextInput != "" {
			for _, c := range strings.Split(contextInput, ",") {
				c = strings.TrimSpace(c)
				if c != "" {
					contextTags = append(contextTags, c)
				}
			}
		}

		// Add to glossary
		_, err := glossaryClient.AddTerm(ctx, &glossaryv1.AddTermRequest{
			TenantId:       tenantID,
			Term:           input,
			Expansion:      expansion,
			Context:        contextTags,
			ExpandInSearch: true,
		})
		if err != nil {
			if strings.Contains(err.Error(), "already exists") {
				fmt.Printf("   \033[33m⚠\033[0m %s already exists in glossary\n", input)
			} else {
				fmt.Printf("   \033[31mError:\033[0m %v\n", err)
			}
		} else {
			fmt.Printf("   \033[32m✓\033[0m Added: %s = %s\n", input, expansion)
			stats.glossary++
		}
		fmt.Println()
	}
	fmt.Println()

	entityClient := entityv1.NewEntityServiceClient(conn)

	// 2. PEOPLE
	fmt.Println("2. PEOPLE")
	fmt.Println("   Add key people (colleagues, stakeholders, contacts).")
	fmt.Println()

	for {
		fmt.Print("   Add person name (or 'done' to continue): ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "" || strings.ToLower(input) == "done" {
			break
		}

		// Get email
		fmt.Print("   > Email: ")
		email, _ := reader.ReadString('\n')
		email = strings.TrimSpace(email)

		if email == "" {
			fmt.Println("   Email is required. Skipping.")
			continue
		}

		// Get optional title
		fmt.Print("   > Title (optional): ")
		title, _ := reader.ReadString('\n')
		title = strings.TrimSpace(title)

		// Get optional department
		fmt.Print("   > Department (optional): ")
		department, _ := reader.ReadString('\n')
		department = strings.TrimSpace(department)

		// Add person
		resp, err := entityClient.BulkCreatePeople(ctx, &entityv1.BulkCreatePeopleRequest{
			TenantId: cfg.TenantID,
			People: []*entityv1.PersonInput{{
				Name:       input,
				Email:      email,
				Title:      title,
				Department: department,
				IsInternal: true,
			}},
			SkipDuplicates: true,
		})
		if err != nil {
			fmt.Printf("   \033[31mError:\033[0m %v\n", err)
		} else if len(resp.Created) > 0 {
			fmt.Printf("   \033[32m✓\033[0m Added: %s <%s>\n", input, email)
			stats.people++
		} else if len(resp.Skipped) > 0 {
			fmt.Printf("   \033[33m⚠\033[0m %s already exists\n", email)
		} else if len(resp.Errors) > 0 {
			fmt.Printf("   \033[31mError:\033[0m %s\n", resp.Errors[0].Error)
		}
		fmt.Println()
	}
	fmt.Println()

	// 3. PRODUCTS
	fmt.Println("3. PRODUCTS")
	fmt.Println("   Add products, features, or services your team works on.")
	fmt.Println()

	for {
		fmt.Print("   Add product name (or 'done' to continue): ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "" || strings.ToLower(input) == "done" {
			break
		}

		// Get optional description
		fmt.Print("   > Description (optional): ")
		description, _ := reader.ReadString('\n')
		description = strings.TrimSpace(description)

		// Get optional type
		fmt.Print("   > Type (product/sub_product/feature, default: product): ")
		prodType, _ := reader.ReadString('\n')
		prodType = strings.TrimSpace(prodType)
		if prodType == "" {
			prodType = "product"
		}

		// Add product
		resp, err := entityClient.BulkCreateProducts(ctx, &entityv1.BulkCreateProductsRequest{
			TenantId: cfg.TenantID,
			Products: []*entityv1.ProductInput{{
				Name:        input,
				Description: description,
				ProductType: prodType,
			}},
			SkipDuplicates: true,
		})
		if err != nil {
			fmt.Printf("   \033[31mError:\033[0m %v\n", err)
		} else if len(resp.Created) > 0 {
			fmt.Printf("   \033[32m✓\033[0m Added: %s\n", input)
			stats.products++
		} else if len(resp.Skipped) > 0 {
			fmt.Printf("   \033[33m⚠\033[0m %s already exists\n", input)
		} else if len(resp.Errors) > 0 {
			fmt.Printf("   \033[31mError:\033[0m %s\n", resp.Errors[0].Error)
		}
		fmt.Println()
	}
	fmt.Println()

	// 4. PROJECTS
	fmt.Println("4. PROJECTS")
	fmt.Println("   Add projects or initiatives your team is tracking.")
	fmt.Println()

	for {
		fmt.Print("   Add project name (or 'done' to continue): ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "" || strings.ToLower(input) == "done" {
			break
		}

		// Get optional description
		fmt.Print("   > Description (optional): ")
		description, _ := reader.ReadString('\n')
		description = strings.TrimSpace(description)

		// Get optional keywords
		fmt.Print("   > Keywords (comma-separated, optional): ")
		keywordsInput, _ := reader.ReadString('\n')
		keywordsInput = strings.TrimSpace(keywordsInput)

		var keywords []string
		if keywordsInput != "" {
			for _, k := range strings.Split(keywordsInput, ",") {
				k = strings.TrimSpace(k)
				if k != "" {
					keywords = append(keywords, k)
				}
			}
		}

		// Add project
		resp, err := entityClient.BulkCreateProjects(ctx, &entityv1.BulkCreateProjectsRequest{
			TenantId: cfg.TenantID,
			Projects: []*entityv1.ProjectInput{{
				Name:        input,
				Description: description,
				Keywords:    keywords,
			}},
			SkipDuplicates: true,
		})
		if err != nil {
			fmt.Printf("   \033[31mError:\033[0m %v\n", err)
		} else if len(resp.Created) > 0 {
			fmt.Printf("   \033[32m✓\033[0m Added: %s\n", input)
			stats.projects++
		} else if len(resp.Skipped) > 0 {
			fmt.Printf("   \033[33m⚠\033[0m %s already exists\n", input)
		} else if len(resp.Errors) > 0 {
			fmt.Printf("   \033[31mError:\033[0m %s\n", resp.Errors[0].Error)
		}
		fmt.Println()
	}
	fmt.Println()

	// Summary
	fmt.Println("Summary")
	fmt.Println("-------")
	fmt.Printf("  Glossary: %d terms added\n", stats.glossary)
	fmt.Printf("  People:   %d added\n", stats.people)
	fmt.Printf("  Products: %d added\n", stats.products)
	fmt.Printf("  Projects: %d added\n", stats.projects)
	fmt.Println()
	fmt.Println("Ready to import content!")
	fmt.Println("  Run: penf ingest email <files>")
	fmt.Println("  Then: penf process onboarding context")

	return nil
}

// connectToInitGateway creates a gRPC connection to the gateway.
func connectToInitGateway(cfg *config.CLIConfig) (*grpc.ClientConn, error) {
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
