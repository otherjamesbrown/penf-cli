// Package cmd provides CLI commands for the penf tool.
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"gopkg.in/yaml.v3"

	productv1 "github.com/otherjamesbrown/penf-cli/api/proto/product/v1"
	"github.com/otherjamesbrown/penf-cli/client"
	"github.com/otherjamesbrown/penf-cli/config"
	"github.com/otherjamesbrown/penf-cli/pkg/logging"
	"github.com/otherjamesbrown/penf-cli/pkg/products"
)

// Product command flags.
var (
	productTenant      string
	productOutput      string
	productParent      string
	productType        string
	productStatus      string
	productDescription string
	productKeywords    []string
)

// ProductCommandDeps holds the dependencies for product commands.
// Core CRUD commands use gRPC via the gateway.
// Advanced commands (team, timeline, query) use direct DB access until
// their gRPC endpoints are implemented.
type ProductCommandDeps struct {
	Config     *config.CLIConfig
	LoadConfig func() (*config.CLIConfig, error)

	// Direct DB access for advanced commands (team, timeline, query).
	// These will be migrated to gRPC in a future phase.
	Pool       *pgxpool.Pool
	Repository *products.Repository
	InitPool   func(*config.CLIConfig) (*pgxpool.Pool, error)
}

// DefaultProductDeps returns the default dependencies for production use.
func DefaultProductDeps() *ProductCommandDeps {
	return &ProductCommandDeps{
		LoadConfig: config.LoadConfig,
		InitPool:   initProductPool,
	}
}

// initProductPool creates a database connection pool from environment variables.
func initProductPool(cfg *config.CLIConfig) (*pgxpool.Pool, error) {
	// Build connection string from environment variables.
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		host := getEnvOrDefault("DB_HOST", "localhost")
		port := getEnvOrDefault("DB_PORT", "5432")
		user := getEnvOrDefault("DB_USER", "penfold")
		pass := os.Getenv("DB_PASSWORD")
		dbname := getEnvOrDefault("DB_NAME", "penfold")
		sslmode := getEnvOrDefault("DB_SSLMODE", "prefer")

		// Start with base connection string
		if pass != "" {
			connStr = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
				host, port, user, pass, dbname, sslmode)
		} else {
			connStr = fmt.Sprintf("host=%s port=%s user=%s dbname=%s sslmode=%s",
				host, port, user, dbname, sslmode)
		}

		// Add SSL cert paths if provided (for cert-based auth)
		if sslcert := os.Getenv("DB_SSLCERT"); sslcert != "" {
			connStr += fmt.Sprintf(" sslcert=%s", sslcert)
		}
		if sslkey := os.Getenv("DB_SSLKEY"); sslkey != "" {
			connStr += fmt.Sprintf(" sslkey=%s", sslkey)
		}
		if sslrootcert := os.Getenv("DB_SSLROOTCERT"); sslrootcert != "" {
			connStr += fmt.Sprintf(" sslrootcert=%s", sslrootcert)
		}
	}

	pool, err := pgxpool.New(context.Background(), connStr)
	if err != nil {
		return nil, fmt.Errorf("connecting to database: %w", err)
	}

	// Test connection.
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		return nil, fmt.Errorf("testing connection: %w", err)
	}

	return pool, nil
}

// initProductDeps initializes the direct DB dependencies for advanced commands.
// This is used by team, timeline, and query commands that haven't been migrated to gRPC yet.
func initProductDeps(ctx context.Context, deps *ProductCommandDeps) error {
	// Load config if not already loaded.
	if deps.Config == nil {
		cfg, err := deps.LoadConfig()
		if err != nil {
			return fmt.Errorf("loading configuration: %w", err)
		}
		deps.Config = cfg
	}

	// Initialize pool if not already initialized.
	if deps.Pool == nil {
		if deps.InitPool == nil {
			deps.InitPool = initProductPool
		}
		pool, err := deps.InitPool(deps.Config)
		if err != nil {
			return err
		}
		deps.Pool = pool
	}

	// Initialize repository if not already initialized.
	if deps.Repository == nil {
		logger := logging.NewLogger(&logging.Config{
			Level:       logging.LevelInfo,
			ServiceName: "penf",
			Output:      os.Stderr,
		})
		deps.Repository = products.NewRepository(deps.Pool, logger)
	}

	return nil
}

// NewProductCommand creates the root product command with all subcommands.
func NewProductCommand(deps *ProductCommandDeps) *cobra.Command {
	if deps == nil {
		deps = DefaultProductDeps()
	}

	cmd := &cobra.Command{
		Use:   "product",
		Short: "Manage products and organizational entities",
		Long: `Manage products and organizational entities in Penfold.

Products represent business products, sub-products, and features organized in
a hierarchy. Each product can have team associations, scoped roles, and timeline
events for tracking decisions, milestones, and changes.

Product Types:
  - product:     Top-level business product (e.g., "LKE", "Managed Databases")
  - sub_product: Sub-product or component (e.g., "LKE Enterprise")
  - feature:     Specific feature (e.g., "Node Pools", "Auto-scaling")

Status Values:
  - active:      Currently active and supported
  - beta:        In beta testing
  - sunset:      Being phased out
  - deprecated:  No longer supported

Examples:
  # List all products
  penf product list

  # Add a new product
  penf product add "My Product" --type product --description "Description"

  # Show product details
  penf product show "My Product"

  # Show product hierarchy
  penf product hierarchy "My Product"

Documentation:
  Product model:          docs/concepts/products.md (hierarchy, teams, events)
  Entity types:           docs/shared/entities.md (products as entities)
  Use case:               docs/shared/use-cases.md (UC-4: Product Knowledge Base)`,
		Aliases: []string{"prod", "products"},
	}

	// Add persistent flags.
	cmd.PersistentFlags().StringVarP(&productTenant, "tenant", "t", "", "Tenant ID (overrides config)")
	cmd.PersistentFlags().StringVarP(&productOutput, "output", "o", "", "Output format: text, json, yaml")

	// Add subcommands.
	cmd.AddCommand(newProductListCommand(deps))
	cmd.AddCommand(newProductAddCommand(deps))
	cmd.AddCommand(newProductShowCommand(deps))
	cmd.AddCommand(newProductHierarchyCommand(deps))
	cmd.AddCommand(newProductAliasCommand(deps))

	// Add team subcommands (defined in product_team.go).
	addProductTeamCommands(cmd, deps)

	// Add timeline and event subcommands (defined in product_timeline.go).
	addProductTimelineCommands(cmd, deps)

	// Add query subcommand (defined in product_query.go).
	addProductQueryCommands(cmd, deps)

	return cmd
}

// newProductListCommand creates the 'product list' subcommand.
func newProductListCommand(deps *ProductCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all products",
		Long: `List all products in the system.

By default, lists top-level products. Use --parent to list children of a
specific product, or --all to list all products regardless of hierarchy.

Examples:
  # List top-level products
  penf product list

  # List all products
  penf product list --all

  # List children of a specific product
  penf product list --parent "LKE"

  # Filter by type
  penf product list --type sub_product

  # Filter by status
  penf product list --status active`,
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			listAll, _ := cmd.Flags().GetBool("all")
			return runProductList(cmd.Context(), deps, listAll)
		},
	}

	cmd.Flags().StringVar(&productParent, "parent", "", "Filter by parent product name")
	cmd.Flags().StringVar(&productType, "type", "", "Filter by product type (product, sub_product, feature)")
	cmd.Flags().StringVar(&productStatus, "status", "", "Filter by status (active, beta, sunset, deprecated)")
	cmd.Flags().Bool("all", false, "List all products (not just top-level)")

	return cmd
}

// newProductAddCommand creates the 'product add' subcommand.
func newProductAddCommand(deps *ProductCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a new product",
		Long: `Add a new product to the system.

Creates a new product with the specified name and options. By default,
products are created as top-level 'product' type with 'active' status.

Examples:
  # Add a simple product
  penf product add "My Product"

  # Add with full details
  penf product add "My Product" \
    --type product \
    --status active \
    --description "My product description" \
    --keywords "api,backend,core"

  # Add a sub-product
  penf product add "Sub Product" --parent "My Product" --type sub_product

  # Add a feature
  penf product add "Cool Feature" --parent "Sub Product" --type feature`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProductAdd(cmd.Context(), deps, args[0])
		},
	}

	cmd.Flags().StringVar(&productParent, "parent", "", "Parent product name (for sub-products/features)")
	cmd.Flags().StringVar(&productType, "type", "product", "Product type: product, sub_product, feature")
	cmd.Flags().StringVar(&productStatus, "status", "active", "Status: active, beta, sunset, deprecated")
	cmd.Flags().StringVar(&productDescription, "description", "", "Product description")
	cmd.Flags().StringSliceVar(&productKeywords, "keywords", nil, "Keywords (comma-separated)")

	return cmd
}

// newProductShowCommand creates the 'product show' subcommand.
func newProductShowCommand(deps *ProductCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "show <name>",
		Short: "Show product details",
		Long: `Show detailed information about a specific product.

Displays the product's properties, aliases, and associated teams.
The name can be either the product name or an alias.

Examples:
  penf product show "My Product"
  penf product show "MP"  # Using an alias`,
		Aliases: []string{"info"},
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProductShow(cmd.Context(), deps, args[0])
		},
	}
}

// newProductHierarchyCommand creates the 'product hierarchy' subcommand.
func newProductHierarchyCommand(deps *ProductCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "hierarchy <name>",
		Short: "Show product hierarchy tree",
		Long: `Show the hierarchy tree starting from a product.

Displays the product and all its descendants in a tree format,
showing the parent-child relationships.

Examples:
  penf product hierarchy "LKE"

Output format:
  LKE (product)
  ├── LKE Enterprise (sub_product)
  │   ├── Node Pools (feature)
  │   └── Auto-scaling (feature)
  └── LKE Standard (sub_product)`,
		Aliases: []string{"tree"},
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProductHierarchy(cmd.Context(), deps, args[0])
		},
	}
}

// newProductAliasCommand creates the 'product alias' subcommand group.
func newProductAliasCommand(deps *ProductCommandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "alias",
		Short: "Manage product aliases",
		Long: `Manage aliases for products.

Aliases allow products to be referenced by alternative names.
For example, "LKE" could have an alias "Kubernetes Engine".`,
	}

	cmd.AddCommand(newProductAliasAddCommand(deps))
	cmd.AddCommand(newProductAliasRemoveCommand(deps))
	cmd.AddCommand(newProductAliasListCommand(deps))

	return cmd
}

// newProductAliasAddCommand creates the 'product alias add' subcommand.
func newProductAliasAddCommand(deps *ProductCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "add <product> <alias>",
		Short: "Add an alias to a product",
		Long: `Add an alternative name (alias) to a product.

Example:
  penf product alias add "LKE" "Kubernetes Engine"`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProductAliasAdd(cmd.Context(), deps, args[0], args[1])
		},
	}
}

// newProductAliasRemoveCommand creates the 'product alias remove' subcommand.
func newProductAliasRemoveCommand(deps *ProductCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "remove <product> <alias>",
		Short: "Remove an alias from a product",
		Long: `Remove an alias from a product.

Example:
  penf product alias remove "LKE" "Kubernetes Engine"`,
		Aliases: []string{"rm"},
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProductAliasRemove(cmd.Context(), deps, args[0], args[1])
		},
	}
}

// newProductAliasListCommand creates the 'product alias list' subcommand.
func newProductAliasListCommand(deps *ProductCommandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "list <product>",
		Short: "List aliases for a product",
		Long: `List all aliases for a product.

Example:
  penf product alias list "LKE"`,
		Aliases: []string{"ls"},
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProductAliasList(cmd.Context(), deps, args[0])
		},
	}
}

// ==================== gRPC Connection ====================

// connectProductToGateway creates a gRPC connection to the gateway service.
func connectProductToGateway(cfg *config.CLIConfig) (*grpc.ClientConn, error) {
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

// getTenantIDForProduct returns the tenant ID from flag, env, or config.
func getTenantIDForProduct(deps *ProductCommandDeps) string {
	if productTenant != "" {
		return productTenant
	}
	if envTenant := os.Getenv("PENF_TENANT_ID"); envTenant != "" {
		return envTenant
	}
	if deps.Config != nil && deps.Config.TenantID != "" {
		return deps.Config.TenantID
	}
	// Default tenant.
	return "00000001-0000-0000-0000-000000000001"
}

// ==================== Command Execution Functions ====================

// runProductList executes the product list command.
func runProductList(ctx context.Context, deps *ProductCommandDeps, listAll bool) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectProductToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := productv1.NewProductServiceClient(conn)
	tenantID := getTenantIDForProduct(deps)

	filter := &productv1.ProductFilter{
		TenantId:   tenantID,
		IncludeAll: listAll,
	}

	if productParent != "" {
		filter.Parent = productParent
	}

	if productType != "" {
		filter.ProductType = productTypeToProto(productType)
	}

	if productStatus != "" {
		filter.Status = productStatusToProto(productStatus)
	}

	resp, err := client.ListProducts(ctx, &productv1.ListProductsRequest{Filter: filter})
	if err != nil {
		return fmt.Errorf("listing products: %w", err)
	}

	return outputProducts(cfg, resp.Products)
}

// runProductAdd executes the product add command.
func runProductAdd(ctx context.Context, deps *ProductCommandDeps, name string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectProductToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := productv1.NewProductServiceClient(conn)
	tenantID := getTenantIDForProduct(deps)

	input := &productv1.ProductInput{
		Name:        name,
		Description: productDescription,
		Parent:      productParent,
		ProductType: productTypeToProto(productType),
		Status:      productStatusToProto(productStatus),
		Keywords:    productKeywords,
	}

	resp, err := client.CreateProduct(ctx, &productv1.CreateProductRequest{
		TenantId: tenantID,
		Input:    input,
	})
	if err != nil {
		return fmt.Errorf("creating product: %w", err)
	}

	fmt.Printf("\033[32mCreated product:\033[0m %s (ID: %d)\n", name, resp.Product.Id)
	return nil
}

// runProductShow executes the product info command.
func runProductShow(ctx context.Context, deps *ProductCommandDeps, name string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectProductToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := productv1.NewProductServiceClient(conn)
	tenantID := getTenantIDForProduct(deps)

	resp, err := client.GetProduct(ctx, &productv1.GetProductRequest{
		TenantId:   tenantID,
		Identifier: name,
	})
	if err != nil {
		return fmt.Errorf("product not found: %s", name)
	}

	return outputProductDetail(cfg, resp.Product)
}

// runProductHierarchy executes the product hierarchy command.
func runProductHierarchy(ctx context.Context, deps *ProductCommandDeps, name string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectProductToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := productv1.NewProductServiceClient(conn)
	tenantID := getTenantIDForProduct(deps)

	resp, err := client.GetHierarchy(ctx, &productv1.GetHierarchyRequest{
		TenantId:   tenantID,
		Identifier: name,
	})
	if err != nil {
		return fmt.Errorf("getting hierarchy: %w", err)
	}

	return outputProductHierarchy(cfg, resp.Hierarchy)
}

// runProductAliasAdd adds an alias to a product.
func runProductAliasAdd(ctx context.Context, deps *ProductCommandDeps, productName, alias string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectProductToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := productv1.NewProductServiceClient(conn)
	tenantID := getTenantIDForProduct(deps)

	resp, err := client.AddAlias(ctx, &productv1.AddAliasRequest{
		TenantId:   tenantID,
		Identifier: productName,
		Alias:      alias,
	})
	if err != nil {
		return fmt.Errorf("adding alias: %w", err)
	}

	fmt.Printf("\033[32mAdded alias:\033[0m '%s' -> %s\n", resp.Alias, resp.ProductName)
	return nil
}

// runProductAliasRemove removes an alias from a product.
func runProductAliasRemove(ctx context.Context, deps *ProductCommandDeps, productName, alias string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectProductToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := productv1.NewProductServiceClient(conn)
	tenantID := getTenantIDForProduct(deps)

	resp, err := client.RemoveAlias(ctx, &productv1.RemoveAliasRequest{
		TenantId:   tenantID,
		Identifier: productName,
		Alias:      alias,
	})
	if err != nil {
		return fmt.Errorf("removing alias: %w", err)
	}

	fmt.Printf("\033[32mRemoved alias:\033[0m '%s' from %s\n", resp.Alias, resp.ProductName)
	return nil
}

// runProductAliasList lists aliases for a product.
func runProductAliasList(ctx context.Context, deps *ProductCommandDeps, productName string) error {
	cfg, err := deps.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}
	deps.Config = cfg

	conn, err := connectProductToGateway(cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := productv1.NewProductServiceClient(conn)
	tenantID := getTenantIDForProduct(deps)

	resp, err := client.ListAliases(ctx, &productv1.ListAliasesRequest{
		TenantId:   tenantID,
		Identifier: productName,
	})
	if err != nil {
		return fmt.Errorf("getting aliases: %w", err)
	}

	return outputProductAliases(cfg, resp.ProductName, resp.Aliases)
}

// ==================== Output Functions ====================

// getProductOutputFormat returns the output format from flag or config.
func getProductOutputFormat(cfg *config.CLIConfig) config.OutputFormat {
	if productOutput != "" {
		return config.OutputFormat(productOutput)
	}
	if cfg != nil {
		return cfg.OutputFormat
	}
	return config.OutputFormatText
}

// outputProducts outputs a list of products.
func outputProducts(cfg *config.CLIConfig, productsList []*productv1.Product) error {
	format := getProductOutputFormat(cfg)

	switch format {
	case config.OutputFormatJSON:
		return outputProductJSON(productsList)
	case config.OutputFormatYAML:
		return outputProductYAML(productsList)
	default:
		return outputProductsTable(productsList)
	}
}

// outputProductsTable outputs products in table format.
func outputProductsTable(productsList []*productv1.Product) error {
	if len(productsList) == 0 {
		fmt.Println("No products found.")
		return nil
	}

	fmt.Printf("Products (%d):\n\n", len(productsList))
	fmt.Println("  ID      NAME                           TYPE          STATUS")
	fmt.Println("  --      ----                           ----          ------")

	for _, p := range productsList {
		typeStr := productTypeFromProtoToString(p.ProductType)
		statusStr := productStatusFromProtoToString(p.Status)
		typeColor := getProductTypeColor(typeStr)
		statusColor := getProductStatusColor(statusStr)
		fmt.Printf("  %-6d  %-30s %s%-12s\033[0m  %s%-10s\033[0m\n",
			p.Id,
			truncateString(p.Name, 30),
			typeColor,
			typeStr,
			statusColor,
			statusStr)
	}

	fmt.Println()
	return nil
}

// outputProductDetail outputs detailed product information.
func outputProductDetail(cfg *config.CLIConfig, product *productv1.Product) error {
	format := getProductOutputFormat(cfg)

	switch format {
	case config.OutputFormatJSON:
		return outputProductJSON(product)
	case config.OutputFormatYAML:
		return outputProductYAML(product)
	default:
		return outputProductDetailText(product)
	}
}

// outputProductDetailText outputs product info in human-readable format.
func outputProductDetailText(product *productv1.Product) error {
	typeStr := productTypeFromProtoToString(product.ProductType)
	statusStr := productStatusFromProtoToString(product.Status)
	typeColor := getProductTypeColor(typeStr)
	statusColor := getProductStatusColor(statusStr)

	fmt.Println("Product Details:")
	fmt.Println()
	fmt.Printf("  \033[1mID:\033[0m          %d\n", product.Id)
	fmt.Printf("  \033[1mName:\033[0m        %s\n", product.Name)
	fmt.Printf("  \033[1mType:\033[0m        %s%s\033[0m\n", typeColor, typeStr)
	fmt.Printf("  \033[1mStatus:\033[0m      %s%s\033[0m\n", statusColor, statusStr)
	fmt.Println()

	if product.Description != "" {
		fmt.Printf("  \033[1mDescription:\033[0m\n    %s\n\n", product.Description)
	}

	if product.ParentName != "" {
		fmt.Printf("  \033[1mParent:\033[0m      %s\n", product.ParentName)
	}

	if len(product.Keywords) > 0 {
		fmt.Printf("  \033[1mKeywords:\033[0m    %s\n", strings.Join(product.Keywords, ", "))
	}

	if len(product.Aliases) > 0 {
		fmt.Printf("  \033[1mAliases:\033[0m     %s\n", strings.Join(product.Aliases, ", "))
	}

	fmt.Println()
	if product.CreatedAt != nil {
		fmt.Printf("  \033[1mCreated:\033[0m     %s\n", product.CreatedAt.AsTime().Format("2006-01-02 15:04:05"))
	}
	if product.UpdatedAt != nil {
		fmt.Printf("  \033[1mUpdated:\033[0m     %s\n", product.UpdatedAt.AsTime().Format("2006-01-02 15:04:05"))
	}

	return nil
}

// outputProductHierarchy outputs the product hierarchy tree.
func outputProductHierarchy(cfg *config.CLIConfig, hierarchy []*productv1.ProductWithHierarchy) error {
	format := getProductOutputFormat(cfg)

	switch format {
	case config.OutputFormatJSON:
		return outputProductJSON(hierarchy)
	case config.OutputFormatYAML:
		return outputProductYAML(hierarchy)
	default:
		return outputProductHierarchyTree(hierarchy)
	}
}

// outputProductHierarchyTree outputs hierarchy as a tree.
func outputProductHierarchyTree(hierarchy []*productv1.ProductWithHierarchy) error {
	if len(hierarchy) == 0 {
		fmt.Println("No products in hierarchy.")
		return nil
	}

	fmt.Println("Product Hierarchy:")
	fmt.Println()

	for i, h := range hierarchy {
		prefix := ""
		for d := 0; d < int(h.Depth); d++ {
			prefix += "    "
		}

		// Determine tree character.
		treeChar := "├── "
		if h.Depth == 0 {
			treeChar = ""
		} else if i == len(hierarchy)-1 || (i+1 < len(hierarchy) && hierarchy[i+1].Depth <= h.Depth) {
			treeChar = "└── "
		}

		typeStr := productTypeFromProtoToString(h.Product.ProductType)
		statusStr := productStatusFromProtoToString(h.Product.Status)
		typeColor := getProductTypeColor(typeStr)
		statusColor := getProductStatusColor(statusStr)

		fmt.Printf("%s%s%s %s(%s)%s %s[%s]%s\n",
			prefix,
			treeChar,
			h.Product.Name,
			typeColor,
			typeStr,
			"\033[0m",
			statusColor,
			statusStr,
			"\033[0m")
	}

	fmt.Println()
	return nil
}

// outputProductAliases outputs product aliases.
func outputProductAliases(cfg *config.CLIConfig, productName string, aliases []*productv1.ProductAlias) error {
	format := getProductOutputFormat(cfg)

	switch format {
	case config.OutputFormatJSON:
		return outputProductJSON(aliases)
	case config.OutputFormatYAML:
		return outputProductYAML(aliases)
	default:
		return outputProductAliasesText(productName, aliases)
	}
}

// outputProductAliasesText outputs aliases in text format.
func outputProductAliasesText(productName string, aliases []*productv1.ProductAlias) error {
	if len(aliases) == 0 {
		fmt.Printf("No aliases for product '%s'\n", productName)
		return nil
	}

	fmt.Printf("Aliases for '%s':\n\n", productName)
	for _, a := range aliases {
		fmt.Printf("  - %s\n", a.Alias)
	}
	fmt.Println()
	return nil
}

// Helper output functions.

func outputProductJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func outputProductYAML(v interface{}) error {
	enc := yaml.NewEncoder(os.Stdout)
	return enc.Encode(v)
}

// ==================== Proto Conversion Helpers ====================

func productTypeToProto(t string) productv1.ProductType {
	switch t {
	case "product":
		return productv1.ProductType_PRODUCT_TYPE_PRODUCT
	case "sub_product":
		return productv1.ProductType_PRODUCT_TYPE_SUB_PRODUCT
	case "feature":
		return productv1.ProductType_PRODUCT_TYPE_FEATURE
	default:
		return productv1.ProductType_PRODUCT_TYPE_PRODUCT
	}
}

func productTypeFromProtoToString(pt productv1.ProductType) string {
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

func productStatusToProto(s string) productv1.ProductStatus {
	switch s {
	case "active":
		return productv1.ProductStatus_PRODUCT_STATUS_ACTIVE
	case "beta":
		return productv1.ProductStatus_PRODUCT_STATUS_BETA
	case "sunset":
		return productv1.ProductStatus_PRODUCT_STATUS_SUNSET
	case "deprecated":
		return productv1.ProductStatus_PRODUCT_STATUS_DEPRECATED
	default:
		return productv1.ProductStatus_PRODUCT_STATUS_ACTIVE
	}
}

func productStatusFromProtoToString(ps productv1.ProductStatus) string {
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

// Color helpers for product output.

func getProductTypeColor(pt string) string {
	switch pt {
	case "product":
		return "\033[35m" // Magenta.
	case "sub_product":
		return "\033[36m" // Cyan.
	case "feature":
		return "\033[34m" // Blue.
	default:
		return ""
	}
}

func getProductStatusColor(ps string) string {
	switch ps {
	case "active":
		return "\033[32m" // Green.
	case "beta":
		return "\033[33m" // Yellow.
	case "sunset":
		return "\033[31m" // Red.
	case "deprecated":
		return "\033[90m" // Gray.
	default:
		return ""
	}
}
