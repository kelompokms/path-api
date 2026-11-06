package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: go run cmd/pather/main.go new <name>")
		os.Exit(1)
	}

	command := os.Args[1]
	name := strings.ToLower(os.Args[2])

	if command != "new" {
		fmt.Printf("Unknown command: %s. Use 'new'.\n", command)
		os.Exit(1)
	}

	if name == "" {
		fmt.Println("Error: module/resource name cannot be empty.")
		os.Exit(1)
	}

	// Capitalized name for types/comments if needed, e.g. Class
	capName := strings.Title(name)
	dir := filepath.Join("internal", name)

	// 1. Create directory
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Printf("Error creating directory %s: %v\n", dir, err)
		os.Exit(1)
	}
	fmt.Printf("Created directory: %s\n", dir)

	// 2. Generate handler.go
	handlerContent := fmt.Sprintf(`package %s

import "path-api/internal/db"

type Handler struct {
	db *db.Queries
}

func New(db *db.Queries) *Handler {
	return &Handler{db}
}
`, name)

	handlerPath := filepath.Join(dir, "handler.go")
	if err := os.WriteFile(handlerPath, []byte(handlerContent), 0644); err != nil {
		fmt.Printf("Error writing %s: %v\n", handlerPath, err)
		os.Exit(1)
	}
	fmt.Printf("Created file: %s\n", handlerPath)

	// 3. Generate logic.go
	logicContent := fmt.Sprintf(`package %s

import "github.com/labstack/echo/v5"

func (h *Handler) List(c *echo.Context) error   {}
func (h *Handler) Get(c *echo.Context) error    {}
func (h *Handler) Create(c *echo.Context) error {}
`, name)

	logicPath := filepath.Join(dir, "logic.go")
	if err := os.WriteFile(logicPath, []byte(logicContent), 0644); err != nil {
		fmt.Printf("Error writing %s: %v\n", logicPath, err)
		os.Exit(1)
	}
	fmt.Printf("Created file: %s\n", logicPath)

	// 4. Generate routes.go
	routesContent := fmt.Sprintf(`package %s

import "github.com/labstack/echo/v5"

func (h *Handler) Routes(e *echo.Group) {
	group := e.Group("/%s")

	group.GET("", h.List)
	group.POST("", h.Create)
	group.GET("/:id", h.Get)
}
`, name, name)

	routesPath := filepath.Join(dir, "routes.go")
	if err := os.WriteFile(routesPath, []byte(routesContent), 0644); err != nil {
		fmt.Printf("Error writing %s: %v\n", routesPath, err)
		os.Exit(1)
	}
	fmt.Printf("Created file: %s\n", routesPath)

	// 5. Inject into main.go
	if err := injectMain(name, capName); err != nil {
		fmt.Printf("Warning: failed to inject into main.go: %v\n", err)
	} else {
		fmt.Println("Successfully injected routes and handler into main.go")
	}

	fmt.Printf("Successfully scaffolded '%s' module!\n", name)
}

func injectMain(name, capName string) error {
	mainPath := "main.go"
	contentBytes, err := os.ReadFile(mainPath)
	if err != nil {
		return err
	}
	content := string(contentBytes)

	importLine := fmt.Sprintf("\t\"path-api/internal/%s\"", name)
	if strings.Contains(content, importLine) {
		return nil // already imported
	}

	// Inject import before last import or closing parenthesis of imports
	importTarget := "\t\"path-api/internal/utils\""
	if !strings.Contains(content, importTarget) {
		return fmt.Errorf("import target not found in main.go")
	}
	newImport := importLine + "\n" + importTarget
	content = strings.Replace(content, importTarget, newImport, 1)

	// Inject handler instantiation and routes registration before port := utils.GetPort()
	regTarget := "\tport := utils.GetPort()"
	if !strings.Contains(content, regTarget) {
		return fmt.Errorf("registration target not found in main.go")
	}

	handlerInstantiation := fmt.Sprintf("\t%sHandler := %s.New(q)\n\t%sHandler.Routes(protected)\n\n\tport := utils.GetPort()", name, name, name)
	content = strings.Replace(content, regTarget, handlerInstantiation, 1)

	return os.WriteFile(mainPath, []byte(content), 0644)
}
