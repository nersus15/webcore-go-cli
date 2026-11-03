package scaffold

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// NewProjectOptions mengatur pembuatan project baru.
type NewProjectOptions struct {
	// Dir adalah nama folder yang dibuat (relatif ke cwd saat ini).
	Dir string
	// ModulePath adalah module path Go (baris "module" di go.mod) —
	// default-nya Dir sendiri (module path lokal tanpa domain, sah untuk
	// Go, ganti manual nanti kalau mau publish). Namanya sengaja
	// "ModulePath", BUKAN "Module" — supaya tidak ambigu dengan ModuleName
	// (nama modul bisnis) di bawah.
	ModulePath string
	// ModuleName, kalau diisi, membuat SATU modul bisnis default
	// (modules/<ModuleName>/ + deps/<ModuleName>/{libraries,packages}.go)
	// sekaligus mem-wiring main.go ke situ. Kosong = project "tanpa modul"
	// (lihat Quick Start § Alternatif Tanpa Modul webcore-docs): hanya
	// main.go + go.mod + config.yaml, tidak ada deps/ atau modules/ sama
	// sekali — tambahkan layer (entity/repository/service/handler) nanti
	// lewat "make entity/repository/..." tanpa --module.
	ModuleName string
	// SkipGoGet melewati `go get github.com/webcore-go/webcore` (mis. saat
	// tidak ada koneksi internet).
	SkipGoGet bool
}

// NewProject membuat project webcore baru. Dua struktur, tergantung
// ModuleName:
//
//   - Kosong: struktur "tanpa modul" — main.go (core.NewApp dengan
//     packages=nil) + go.mod + config.yaml saja, tidak ada deps/ atau
//     modules/.
//   - Diisi: struktur klasik dengan SATU modul bisnis default, dinestkan
//     di bawah go.mod project ini (modules/<ModuleName>/), dan wiring-nya
//     di deps/<ModuleName>/{libraries,packages}.go — BUKAN flat di
//     deps/libraries.go, supaya kalau nanti nambah modul lagi lewat
//     "make module <nama2> --deps-dir=deps" pola foldernya sudah konsisten.
func NewProject(opt NewProjectOptions) error {
	if opt.ModulePath == "" {
		opt.ModulePath = opt.Dir
	}

	if _, err := os.Stat(opt.Dir); err == nil {
		return fmt.Errorf("folder %q sudah ada", opt.Dir)
	}
	if err := os.MkdirAll(opt.Dir, 0o755); err != nil {
		return fmt.Errorf("buat folder %s: %w", opt.Dir, err)
	}

	withModule := opt.ModuleName != ""

	fmt.Printf("Membuat project %q (module %s)...\n", opt.Dir, opt.ModulePath)

	if err := runIn(opt.Dir, "go", "mod", "init", opt.ModulePath); err != nil {
		return fmt.Errorf("go mod init: %w", err)
	}

	// Pindah ke dalam project — langkah-langkah sesudah ini (termasuk
	// NewModule, yang mendeteksi go.mod lewat cwd) memakai path relatif
	// terhadap root project ini.
	origDir, err := os.Getwd()
	if err != nil {
		return err
	}
	if err := os.Chdir(opt.Dir); err != nil {
		return fmt.Errorf("masuk ke %s: %w", opt.Dir, err)
	}
	defer os.Chdir(origDir)

	data := map[string]string{
		"ModulePath":  opt.ModulePath,
		"ProjectName": opt.Dir,
		"ModuleName":  opt.ModuleName,
	}

	files := map[string]string{
		"config.yaml":         projectConfigYAMLTmpl,
		".gitignore":          projectGitignoreTmpl,
		"Makefile":            projectMakefileTmpl,
		"migrations/.gitkeep": "",
	}
	if withModule {
		files["main.go"] = projectMainGoWithModuleTmpl
		files["access.yaml"] = projectAccessYAMLTmpl
	} else {
		files["main.go"] = projectMainGoNoModuleTmpl
	}

	for rel, tmpl := range files {
		content, err := render(tmpl, data)
		if err != nil {
			return fmt.Errorf("render %s: %w", rel, err)
		}
		if err := WriteFile(rel, content, false); err != nil {
			return err
		}
	}

	if withModule {
		if err := NewModule(opt.ModuleName, "", false); err != nil {
			return fmt.Errorf("buat modul default %s: %w", opt.ModuleName, err)
		}
		if err := generateNestedDeps(opt.ModuleName, opt.ModulePath, false); err != nil {
			return fmt.Errorf("buat deps/%s: %w", opt.ModuleName, err)
		}
	}

	if !opt.SkipGoGet {
		fmt.Println("Mengambil github.com/webcore-go/webcore...")
		if err := runIn(".", "go", "get", "github.com/webcore-go/webcore@latest"); err != nil {
			fmt.Printf("  peringatan: go get gagal (%v) — jalankan manual nanti: cd %s && go get github.com/webcore-go/webcore@latest\n", err, opt.Dir)
		} else if err := runIn(".", "go", "mod", "tidy"); err != nil {
			fmt.Printf("  peringatan: go mod tidy gagal: %v\n", err)
		}
	}

	if withModule {
		fmt.Printf(`
Project %q siap dengan modul default %q.

Langkah selanjutnya:
  cd %s
  go run main.go

Tambah modul lagi:
  webcore-go-cli make module <nama> --deps-dir=deps
`, opt.Dir, opt.ModuleName, opt.Dir)
	} else {
		fmt.Printf(`
Project %q siap (tanpa modul).

Langkah selanjutnya:
  cd %s
  go run main.go

Tambah layer satu-file langsung di root (lihat Quick Start § Alternatif
Tanpa Modul webcore-docs):
  webcore-go-cli make entity <Nama>
  webcore-go-cli make repository <Nama>
  webcore-go-cli make service <Nama>
  webcore-go-cli make handler <Nama>

Atau modul penuh pertama:
  webcore-go-cli make module <nama>
`, opt.Dir, opt.Dir)
	}

	return nil
}

func runIn(dir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// generateNestedDeps membuat deps/<name>/{libraries,packages}.go untuk
// modul default yang DINESTKAN di bawah go.mod project ini (bukan
// standalone --module-path) — packages.go meng-import modules/<name>
// dengan alias eksplisit (package itu juga bernama <name>, tidak masalah,
// alias cuma berlaku di file ini).
func generateNestedDeps(name, modulePath string, force bool) error {
	data := map[string]string{"ModuleName": name, "ModulePath": modulePath}

	base := filepath.Join("deps", name)
	files := map[string]string{
		"libraries.go": nestedDepsLibrariesTmpl,
		"packages.go":  nestedDepsPackagesTmpl,
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
	return nil
}

const nestedDepsLibrariesTmpl = `package {{.ModuleName}}

import (
	"github.com/webcore-go/webcore/app/core"
)

// APP_LIBRARIES khusus modul "{{.ModuleName}}".
var APP_LIBRARIES = map[string]core.LibraryLoader{
	// "database:postgres": &postgres.PostgresLoader{},
}
`

const nestedDepsPackagesTmpl = `package {{.ModuleName}}

import (
	{{.ModuleName}} "{{.ModulePath}}/modules/{{.ModuleName}}"
	"github.com/webcore-go/webcore/app/core"
)

var APP_PACKAGES = []core.Module{
	{{.ModuleName}}.NewModule(),
}
`

const projectMainGoWithModuleTmpl = `package main

import (
	"context"

	{{.ModuleName}} "{{.ModulePath}}/deps/{{.ModuleName}}"
	"github.com/webcore-go/webcore/app/core"
	"github.com/webcore-go/webcore/app/out"
	"github.com/webcore-go/webcore/infra/config"
)

func main() {
	cfg := &config.Config{}
	if err := config.LoadDefaultConfig(cfg); err != nil {
		panic(err)
	}
	// penting: tanpa ini response error selalu bermode "development"
	// (stack trace & detail bocor ke klien)
	out.SetEnvironment(cfg.App.Environment)

	app := core.NewApp(context.Background(), cfg, {{.ModuleName}}.APP_LIBRARIES, {{.ModuleName}}.APP_PACKAGES)
	if err := app.Start(); err != nil {
		panic(err)
	}
}
`

const projectMainGoNoModuleTmpl = `package main

import (
	"context"

	"github.com/webcore-go/webcore/app/core"
	"github.com/webcore-go/webcore/app/out"
	"github.com/webcore-go/webcore/infra/config"
)

func main() {
	cfg := &config.Config{}
	if err := config.LoadDefaultConfig(cfg); err != nil {
		panic(err)
	}
	out.SetEnvironment(cfg.App.Environment)

	libraries := map[string]core.LibraryLoader{} // tambahkan library di sini kalau perlu

	app := core.NewApp(context.Background(), cfg, libraries, nil) // tanpa modul
	if err := app.Start(); err != nil {
		panic(err)
	}
}
`

const projectConfigYAMLTmpl = `app:
  name: "{{.ProjectName}}"
  environment: "development"
  logging:
    level: "debug"
  cors:
    allow_credentials: false   # wajib false kalau allow_origins masih "*" (default), kalau tidak Fiber panic saat start

server:
  host: "0.0.0.0"
  port: 7272
  path: "/api"     # semua route modul berada di bawah prefix ini

auth:
  type: "none"     # ganti ke "apikey" | "basic" | "jwt" begitu butuh autentikasi
`

const projectAccessYAMLTmpl = `# Dipakai kalau auth.store: yaml dan auth.type bukan "none".
# Lihat dokumentasi Autentikasi untuk formatnya.
users: []
resources: []
`

const projectGitignoreTmpl = `/bin/
/tmp/
*.env
.env
`

const projectMakefileTmpl = `.PHONY: run build test vet fmt

run:
	go run main.go

build:
	go build -o ./bin/{{.ProjectName}} .

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -l .
`
