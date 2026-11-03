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
	// ModulePath adalah module path Go — default-nya Dir sendiri (module
	// path lokal tanpa domain, sah untuk Go, ganti manual nanti kalau mau
	// publish).
	ModulePath string
	// SkipGoGet melewati `go get github.com/webcore-go/webcore` (mis. saat
	// tidak ada koneksi internet).
	SkipGoGet bool
}

// NewProject membuat project webcore baru dengan satu modul kosong yang
// sudah siap diisi (lihat `make:module`), mengikuti struktur yang
// didokumentasikan di Quick Start webcore-docs.
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

	fmt.Printf("Membuat project %q (module %s)...\n", opt.Dir, opt.ModulePath)

	if err := runIn(opt.Dir, "go", "mod", "init", opt.ModulePath); err != nil {
		return fmt.Errorf("go mod init: %w", err)
	}

	files := map[string]string{
		"main.go":             projectMainGoTmpl,
		"config.yaml":         projectConfigYAMLTmpl,
		"access.yaml":         projectAccessYAMLTmpl,
		"deps/libraries.go":   projectDepsLibrariesTmpl,
		"deps/packages.go":    projectDepsPackagesTmpl,
		".gitignore":          projectGitignoreTmpl,
		"Makefile":            projectMakefileTmpl,
		"modules/.gitkeep":    "",
		"migrations/.gitkeep": "",
	}

	data := map[string]string{"ModulePath": opt.ModulePath, "ProjectName": opt.Dir}
	for rel, tmpl := range files {
		content, err := render(tmpl, data)
		if err != nil {
			return fmt.Errorf("render %s: %w", rel, err)
		}
		if err := WriteFile(filepath.Join(opt.Dir, rel), content, false); err != nil {
			return err
		}
	}

	if !opt.SkipGoGet {
		fmt.Println("Mengambil github.com/webcore-go/webcore...")
		if err := runIn(opt.Dir, "go", "get", "github.com/webcore-go/webcore@latest"); err != nil {
			fmt.Printf("  peringatan: go get gagal (%v) — jalankan manual nanti: cd %s && go get github.com/webcore-go/webcore@latest\n", err, opt.Dir)
		} else if err := runIn(opt.Dir, "go", "mod", "tidy"); err != nil {
			fmt.Printf("  peringatan: go mod tidy gagal: %v\n", err)
		}
	}

	fmt.Printf(`
Project %q siap.

Langkah selanjutnya:
  cd %s
  webcore-cli make:module hello     # buat modul pertama
  go run main.go

`, opt.Dir, opt.Dir)

	return nil
}

func runIn(dir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

const projectMainGoTmpl = `package main

import (
	"context"

	"{{.ModulePath}}/deps"
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

	app := core.NewApp(context.Background(), cfg, deps.APP_LIBRARIES, deps.APP_PACKAGES)
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

const projectDepsLibrariesTmpl = `package deps

import (
	"github.com/webcore-go/webcore/app/core"
)

// Key map: "<jenis>:<driver>" — dicocokkan dengan nilai config, lihat
// dokumentasi Dependency Injection.
var APP_LIBRARIES = map[string]core.LibraryLoader{
	// "database:postgres": &postgres.PostgresLoader{},
	// "cache:redis":       &redis.RedisLoader{},
	// "authentication:apikey": &apikey.ApiKeyLoader{},
	// "authstorage:yaml":      &authyaml.YamlLoader{},
}
`

const projectDepsPackagesTmpl = `package deps

import (
	"github.com/webcore-go/webcore/app/core"
)

var APP_PACKAGES = []core.Module{
	// tambahkan modul di sini, mis: hello.NewModule()
	// (dibuat otomatis oleh "webcore-cli make:module <nama>")
}
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
