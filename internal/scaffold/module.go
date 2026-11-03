package scaffold

import (
	"fmt"
	"path/filepath"

	"github.com/webcore-go/webcore-cli/internal/names"
)

// ModuleData adalah variabel yang tersedia di semua template modul.
type ModuleData struct {
	ModulePath string // module path project, mis. github.com/acme/my-api
	Pkg        string // nama package/direktori, mis. "orders"
	Entity     string // PascalCase, mis. "Order"
	Table      string // nama tabel, mis. "orders"
}

// NewModule membuat modul lengkap (module.go + config/entity/repository/
// service/handler) di modules/<pkg>/, mengikuti pola yang didokumentasikan
// di webcore-docs § Pengembangan Modul.
func NewModule(modulePath, name string, force bool) error {
	pkg := names.Package(name)
	entity := names.Pascal(name)

	data := ModuleData{
		ModulePath: modulePath,
		Pkg:        pkg,
		Entity:     entity,
		Table:      names.Table(name),
	}

	base := filepath.Join("modules", pkg)
	files := map[string]string{
		"module.go":                moduleGoTmpl,
		"config/config.go":         moduleConfigTmpl,
		"entity/" + pkg + ".go":    moduleEntityTmpl,
		"repository/repository.go": moduleRepositoryTmpl,
		"service/service.go":       moduleServiceTmpl,
		"handler/handler.go":       moduleHandlerTmpl,
	}

	for rel, tmpl := range files {
		content, err := render(tmpl, data)
		if err != nil {
			return fmt.Errorf("render %s: %w", rel, err)
		}
		if err := WriteFile(filepath.Join(base, rel), content, force); err != nil {
			return err
		}
	}

	fmt.Printf(`
Modul %q siap di %s/.

Jangan lupa daftarkan di deps/packages.go:

    import "%s/modules/%s"

    var APP_PACKAGES = []core.Module{
        %s.NewModule(),
    }

Dan kalau modul ini butuh database, daftarkan loader-nya di deps/libraries.go
(mis. "database:postgres": &postgres.PostgresLoader{}).
`, pkg, base, modulePath, pkg, pkg)

	return nil
}

const moduleGoTmpl = `package {{.Pkg}}

import (
	"github.com/webcore-go/webcore/app/core"
	appConfig "github.com/webcore-go/webcore/infra/config"
	"github.com/webcore-go/webcore/port"

	"{{.ModulePath}}/modules/{{.Pkg}}/config"
	"{{.ModulePath}}/modules/{{.Pkg}}/handler"
	"{{.ModulePath}}/modules/{{.Pkg}}/repository"
	"{{.ModulePath}}/modules/{{.Pkg}}/service"
)

const (
	ModuleName    = "{{.Pkg}}"
	ModuleVersion = "1.0.0"
)

type Module struct {
	config     *config.ModuleConfig
	repository *repository.Repository
	service    *service.Service
	handler    *handler.Handler
	routes     []*core.ModuleRoute
}

func NewModule() *Module { return &Module{} }

func (m *Module) Name() string                   { return ModuleName }
func (m *Module) Version() string                 { return ModuleVersion }
func (m *Module) Dependencies() []string          { return []string{} }
func (m *Module) Config() appConfig.ConfigObject  { return m.config }
func (m *Module) Routes() []*core.ModuleRoute     { return m.routes }
func (m *Module) Destroy() error                  { return nil }

func (m *Module) Services() map[string]any {
	return map[string]any{"service": m.service}
}

func (m *Module) Repositories() map[string]any {
	return map[string]any{"repository": m.repository}
}

func (m *Module) Init(ctx *core.AppContext) error {
	// 1. Muat konfigurasi modul (namespace module.{{.Pkg}} di config.yaml).
	m.config = &config.ModuleConfig{}
	if err := appConfig.LoadDefaultConfigModule(m.Name(), m.config); err != nil {
		return err
	}

	// 2. Ambil shared library — database harus sudah didaftarkan di
	//    deps/libraries.go (mis. "database:postgres").
	if lib, ok := ctx.GetDefaultSingletonInstance("database"); ok {
		db := lib.(port.IDatabase)
		m.repository = repository.NewRepository(ctx, m.config, db)
		m.service = service.NewService(ctx, m.repository)
		m.handler = handler.NewHandler(ctx, m.service)
	}

	// 3. Registrasi route di bawah ctx.Root (sudah ber-auth & ber-prefix).
	m.registerRoutes(ctx)

	return nil
}

func (m *Module) registerRoutes(ctx *core.AppContext) {
	root := ctx.Root.Group("/" + m.Name())

	m.routes = core.AppendRouteToArray(m.routes, &core.ModuleRoute{
		Method: "GET", Path: "/", Handler: m.handler.List{{.Entity}}s, Root: root,
	})
	m.routes = core.AppendRouteToArray(m.routes, &core.ModuleRoute{
		Method: "POST", Path: "/", Handler: m.handler.Create{{.Entity}}, Root: root,
	})
	m.routes = core.AppendRouteToArray(m.routes, &core.ModuleRoute{
		Method: "GET", Path: "/:id", Handler: m.handler.Get{{.Entity}}, Root: root,
	})
}
`

const moduleConfigTmpl = `// Package config berisi konfigurasi khusus modul {{.Pkg}}, dibaca dari
// namespace "module.{{.Pkg}}.*" di config.yaml / env MODULE_{{.Pkg | upper}}_*.
package config

type ModuleConfig struct {
	PageSize int ` + "`mapstructure:\"page_size\"`" + `
}

func (c *ModuleConfig) SetDefaults() map[string]any {
	return map[string]any{
		"page_size": 20,
	}
}

func (c *ModuleConfig) SetEnvBindings() map[string]string {
	return map[string]string{
		"page_size": "MODULE_{{.Pkg | upper}}_PAGE_SIZE",
	}
}
`

const moduleEntityTmpl = `package entity

import "time"

type {{.Entity}} struct {
	ID        string    ` + "`json:\"id\" db:\"id\"`" + `
	Name      string    ` + "`json:\"name\" db:\"name\"`" + `
	CreatedAt time.Time ` + "`json:\"created_at\" db:\"created_at\"`" + `
	DeletedAt *time.Time ` + "`json:\"deleted_at,omitempty\" db:\"deleted_at\"`" + `
}

type {{.Entity}}Input struct {
	Name string ` + "`json:\"name\"`" + `
}
`

const moduleRepositoryTmpl = `package repository

import (
	"context"

	"github.com/webcore-go/webcore/app/core"
	"github.com/webcore-go/webcore/port"

	"{{.ModulePath}}/modules/{{.Pkg}}/config"
	"{{.ModulePath}}/modules/{{.Pkg}}/entity"
)

type {{.Entity}}Repository interface {
	GetAll(ctx context.Context) ([]entity.{{.Entity}}, error)
	GetByID(ctx context.Context, id string) (*entity.{{.Entity}}, error)
	Create(ctx context.Context, input entity.{{.Entity}}Input) (*entity.{{.Entity}}, error)
}

type Repository struct {
	Connection port.IDatabase
	Context    *core.AppContext
	Config     *config.ModuleConfig
}

func NewRepository(ctx *core.AppContext, cfg *config.ModuleConfig, db port.IDatabase) *Repository {
	return &Repository{Connection: db, Context: ctx, Config: cfg}
}

func (r *Repository) GetAll(ctx context.Context) ([]entity.{{.Entity}}, error) {
	var items []entity.{{.Entity}}

	filter := []port.DbExpression{
		{Expr: "deleted_at", Args: []any{nil}}, // soft-delete filter
	}
	sort := map[string]int{"created_at": -1}

	err := r.Connection.Find(ctx, &items, "{{.Table}}", []string{}, filter, sort, 0, 0)
	return items, err
}

func (r *Repository) GetByID(ctx context.Context, id string) (*entity.{{.Entity}}, error) {
	var item entity.{{.Entity}}

	filter := []port.DbExpression{
		{Expr: "id", Args: []any{id}},
		{Expr: "deleted_at", Args: []any{nil}},
	}

	if err := r.Connection.FindOne(ctx, &item, "{{.Table}}", []string{}, filter, nil); err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repository) Create(ctx context.Context, input entity.{{.Entity}}Input) (*entity.{{.Entity}}, error) {
	item := &entity.{{.Entity}}{Name: input.Name}

	if _, err := r.Connection.InsertOne(ctx, "{{.Table}}", item); err != nil {
		return nil, err
	}
	return item, nil
}
`

const moduleServiceTmpl = `package service

import (
	"context"
	"fmt"

	"github.com/webcore-go/webcore/app/core"

	"{{.ModulePath}}/modules/{{.Pkg}}/entity"
	"{{.ModulePath}}/modules/{{.Pkg}}/repository"
)

type {{.Entity}}Service interface {
	Get{{.Entity}}s(ctx context.Context) ([]entity.{{.Entity}}, error)
	Get{{.Entity}}(ctx context.Context, id string) (*entity.{{.Entity}}, error)
	Create{{.Entity}}(ctx context.Context, input entity.{{.Entity}}Input) (*entity.{{.Entity}}, error)
}

type Service struct {
	repo repository.{{.Entity}}Repository
}

func NewService(ctx *core.AppContext, repo repository.{{.Entity}}Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Get{{.Entity}}s(ctx context.Context) ([]entity.{{.Entity}}, error) {
	return s.repo.GetAll(ctx)
}

func (s *Service) Get{{.Entity}}(ctx context.Context, id string) (*entity.{{.Entity}}, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) Create{{.Entity}}(ctx context.Context, input entity.{{.Entity}}Input) (*entity.{{.Entity}}, error) {
	if input.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	return s.repo.Create(ctx, input)
}
`

const moduleHandlerTmpl = `package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/webcore-go/webcore/app/core"
	"github.com/webcore-go/webcore/app/helper"
	"github.com/webcore-go/webcore/app/out"

	"{{.ModulePath}}/modules/{{.Pkg}}/entity"
	"{{.ModulePath}}/modules/{{.Pkg}}/service"
)

type Handler struct {
	service service.{{.Entity}}Service
}

func NewHandler(ctx *core.AppContext, svc service.{{.Entity}}Service) *Handler {
	return &Handler{service: svc}
}

func (h *Handler) List{{.Entity}}s(c *fiber.Ctx) error {
	items, err := h.service.Get{{.Entity}}s(c.Context())
	if err != nil {
		return err // ditangani ErrorHandler global
	}

	page := c.QueryInt("page", 1)
	pageSize := c.QueryInt("page_size", 20)
	paged, pagination := helper.Paginate(items, page, pageSize)
	return c.JSON(helper.NewPaginatedResponse(paged, pagination))
}

func (h *Handler) Get{{.Entity}}(c *fiber.Ctx) error {
	item, err := h.service.Get{{.Entity}}(c.Context(), c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusNotFound).
			JSON(out.Error(fiber.StatusNotFound, 1, "NOT_FOUND", err.Error()))
	}
	return c.JSON(out.SuccessData(item))
}

func (h *Handler) Create{{.Entity}}(c *fiber.Ctx) error {
	var input entity.{{.Entity}}Input
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).
			JSON(out.Error(fiber.StatusBadRequest, 10, "BAD_REQUEST", err.Error()))
	}

	item, err := h.service.Create{{.Entity}}(c.Context(), input)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).
			JSON(out.Error(fiber.StatusBadRequest, 11, "VALIDATION", err.Error()))
	}
	return c.JSON(out.SuccessData(item))
}
`
