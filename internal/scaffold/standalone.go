package scaffold

import (
	"fmt"
	"path/filepath"

	"github.com/nersus15/webcore-go-cli/internal/gomod"
	"github.com/nersus15/webcore-go-cli/internal/names"
)

// StandaloneData adalah variabel template untuk generator satu-file
// (config/entity/repository/service/handler) yang bisa ditaruh di root
// project (tanpa modul) atau di-scope ke dalam modul lewat --module.
type StandaloneData struct {
	Entity        string
	Pkg           string
	Table         string
	EntityImport  string
	RepoImport    string
	ServiceImport string
}

// resolveScope menentukan direktori tujuan dan import path untuk satu
// "kind" (entity/repository/service/handler/config), tergantung ada
// --module atau tidak:
//
//   - Tanpa --module: rata di root project (mis. entity/order.go) —
//     pola yang didokumentasikan di Quick Start § Alternatif Tanpa Modul.
//     Butuh go.mod di root project (dicari lewat gomod.DetectModulePath,
//     yang juga naik ke direktori induk kalau perlu).
//
//   - Dengan --module=orders DAN modules/orders/ punya go.mod sendiri
//     (pola project multi-module/go.work, lihat workspace.md): pakai
//     module path ASLI dari go.mod itu — tidak pernah ditebak dari nama
//     direktori, karena keduanya boleh berbeda.
//
//   - Dengan --module=orders tapi modules/orders/ TIDAK punya go.mod
//     sendiri: fallback ke pola lama, dinestkan di bawah go.mod root
//     project (modules/orders/<kind> diimpor sebagai
//     "<module root>/modules/orders/<kind>").
func resolveScope(moduleScope, kind string) (dir, importPath string, err error) {
	if moduleScope == "" {
		modulePath, err := gomod.DetectModulePath()
		if err != nil {
			return "", "", err
		}
		return kind, modulePath + "/" + kind, nil
	}

	scopePkg := names.Package(moduleScope)
	moduleDir := filepath.Join("modules", scopePkg)
	dir = filepath.Join(moduleDir, kind)

	if ownPath, ok := gomod.ModulePathAt(moduleDir); ok {
		return dir, ownPath + "/" + kind, nil
	}

	modulePath, err := gomod.DetectModulePath()
	if err != nil {
		return "", "", fmt.Errorf("modul %q tidak punya go.mod sendiri di %s, dan: %w", moduleScope, moduleDir, err)
	}
	return dir, modulePath + "/modules/" + scopePkg + "/" + kind, nil
}

func buildStandaloneData(name, moduleScope string) (StandaloneData, error) {
	_, entityImport, err := resolveScope(moduleScope, "entity")
	if err != nil {
		return StandaloneData{}, err
	}
	_, repoImport, err := resolveScope(moduleScope, "repository")
	if err != nil {
		return StandaloneData{}, err
	}
	_, svcImport, err := resolveScope(moduleScope, "service")
	if err != nil {
		return StandaloneData{}, err
	}

	return StandaloneData{
		Entity:        names.Pascal(name),
		Pkg:           names.Package(name),
		Table:         names.Table(name),
		EntityImport:  entityImport,
		RepoImport:    repoImport,
		ServiceImport: svcImport,
	}, nil
}

func writeStandalone(name, moduleScope, kind, tmpl string, force bool) error {
	dir, _, err := resolveScope(moduleScope, kind)
	if err != nil {
		return err
	}

	data, err := buildStandaloneData(name, moduleScope)
	if err != nil {
		return err
	}

	content, err := render(tmpl, data)
	if err != nil {
		return fmt.Errorf("render: %w", err)
	}

	return WriteFile(filepath.Join(dir, data.Pkg+".go"), content, force)
}

// NewEntity membuat file entity baru (struct + Input struct).
func NewEntity(name, moduleScope string, force bool) error {
	return writeStandalone(name, moduleScope, "entity", entityTmpl, force)
}

// NewConfig membuat ConfigObject baru (SetDefaults/SetEnvBindings) —
// bisa dipakai lewat config.LoadDefaultConfigModule tanpa perlu core.Module.
func NewConfig(name, moduleScope string, force bool) error {
	return writeStandalone(name, moduleScope, "config", configTmpl, force)
}

// NewRepository membuat repository baru di atas port.IDatabase.
func NewRepository(name, moduleScope string, force bool) error {
	return writeStandalone(name, moduleScope, "repository", repositoryTmpl, force)
}

// NewService membuat service baru yang membungkus repository.
func NewService(name, moduleScope string, force bool) error {
	return writeStandalone(name, moduleScope, "service", serviceTmpl, force)
}

// NewHandler membuat Fiber handler baru yang membungkus service.
func NewHandler(name, moduleScope string, force bool) error {
	return writeStandalone(name, moduleScope, "handler", handlerTmpl, force)
}

// NewHelper membuat package helper mandiri di helper/<nama>.go — tidak
// terikat konvensi apa pun, cuma skeleton fungsi murni untuk diisi sendiri.
func NewHelper(name string, force bool) error {
	pkg := names.Package(name)
	fn := names.Pascal(name)

	data := map[string]string{"Pkg": pkg, "Fn": fn}
	content, err := render(helperTmpl, data)
	if err != nil {
		return err
	}

	return WriteFile(filepath.Join("helper", pkg+".go"), content, force)
}

const entityTmpl = `package entity

import "time"

type {{.Entity}} struct {
	ID        string    ` + "`json:\"id\" db:\"id\"`" + `
	Name      string    ` + "`json:\"name\" db:\"name\"`" + `
	CreatedAt time.Time ` + "`json:\"created_at\" db:\"created_at\"`" + `
}

type {{.Entity}}Input struct {
	Name string ` + "`json:\"name\"`" + `
}
`

const configTmpl = `// Package config — dipakai lewat appconfig.LoadDefaultConfigModule, tidak
// menuntut core.Module. Namespace default: module.{{.Pkg}}.* di config.yaml.
package config

type {{.Entity}}Config struct {
	PageSize int ` + "`mapstructure:\"page_size\"`" + `
}

func (c *{{.Entity}}Config) SetDefaults() map[string]any {
	return map[string]any{
		"page_size": 20,
	}
}

func (c *{{.Entity}}Config) SetEnvBindings() map[string]string {
	return map[string]string{
		"page_size": "{{.Pkg | upper}}_PAGE_SIZE",
	}
}
`

const repositoryTmpl = `package repository

import (
	"context"

	"github.com/webcore-go/webcore/port"

	"{{.EntityImport}}"
)

type {{.Entity}}Repository struct {
	Connection port.IDatabase
}

func New{{.Entity}}Repository(db port.IDatabase) *{{.Entity}}Repository {
	return &{{.Entity}}Repository{Connection: db}
}

func (r *{{.Entity}}Repository) GetAll(ctx context.Context) ([]entity.{{.Entity}}, error) {
	var items []entity.{{.Entity}}

	filter := []port.DbExpression{
		{Expr: "deleted_at", Args: []any{nil}}, // soft-delete filter
	}
	sort := map[string]int{"created_at": -1}

	err := r.Connection.Find(ctx, &items, "{{.Table}}", []string{}, filter, sort, 0, 0)
	return items, err
}

func (r *{{.Entity}}Repository) GetByID(ctx context.Context, id string) (*entity.{{.Entity}}, error) {
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

func (r *{{.Entity}}Repository) Create(ctx context.Context, input entity.{{.Entity}}Input) (*entity.{{.Entity}}, error) {
	item := &entity.{{.Entity}}{Name: input.Name}

	if _, err := r.Connection.InsertOne(ctx, "{{.Table}}", item); err != nil {
		return nil, err
	}
	return item, nil
}
`

const serviceTmpl = `package service

import (
	"context"
	"fmt"

	"{{.EntityImport}}"
	"{{.RepoImport}}"
)

type {{.Entity}}Service struct {
	repo *repository.{{.Entity}}Repository
}

func New{{.Entity}}Service(repo *repository.{{.Entity}}Repository) *{{.Entity}}Service {
	return &{{.Entity}}Service{repo: repo}
}

func (s *{{.Entity}}Service) Get{{.Entity}}s(ctx context.Context) ([]entity.{{.Entity}}, error) {
	return s.repo.GetAll(ctx)
}

func (s *{{.Entity}}Service) Get{{.Entity}}(ctx context.Context, id string) (*entity.{{.Entity}}, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *{{.Entity}}Service) Create{{.Entity}}(ctx context.Context, input entity.{{.Entity}}Input) (*entity.{{.Entity}}, error) {
	if input.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	return s.repo.Create(ctx, input)
}
`

const handlerTmpl = `package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/webcore-go/webcore/app/helper"
	"github.com/webcore-go/webcore/app/out"

	"{{.EntityImport}}"
	"{{.ServiceImport}}"
)

type {{.Entity}}Handler struct {
	service *service.{{.Entity}}Service
}

func New{{.Entity}}Handler(svc *service.{{.Entity}}Service) *{{.Entity}}Handler {
	return &{{.Entity}}Handler{service: svc}
}

func (h *{{.Entity}}Handler) List{{.Entity}}s(c *fiber.Ctx) error {
	items, err := h.service.Get{{.Entity}}s(c.Context())
	if err != nil {
		return err
	}

	page := c.QueryInt("page", 1)
	pageSize := c.QueryInt("page_size", 20)
	paged, pagination := helper.Paginate(items, page, pageSize)
	return c.JSON(helper.NewPaginatedResponse(paged, pagination))
}

func (h *{{.Entity}}Handler) Get{{.Entity}}(c *fiber.Ctx) error {
	item, err := h.service.Get{{.Entity}}(c.Context(), c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusNotFound).
			JSON(out.Error(fiber.StatusNotFound, 1, "NOT_FOUND", err.Error()))
	}
	return c.JSON(out.SuccessData(item))
}

func (h *{{.Entity}}Handler) Create{{.Entity}}(c *fiber.Ctx) error {
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

const helperTmpl = `package helper

// {{.Fn}} — isi sendiri logikanya. Helper di webcore murni fungsi biasa,
// tidak ada interface atau registrasi apa pun yang dibutuhkan.
func {{.Fn}}(input string) string {
	return input
}
`
