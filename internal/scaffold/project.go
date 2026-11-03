package scaffold

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/nersus15/webcore-go-cli/internal/names"
)

// NewProjectOptions mengatur pembuatan project baru.
type NewProjectOptions struct {
	// Dir adalah nama folder yang dibuat (relatif ke cwd saat ini).
	Dir string
	// ModulePath adalah module path Go dasar — default-nya Dir sendiri
	// (module path lokal tanpa domain, sah untuk Go, ganti manual nanti
	// kalau mau publish). Namanya sengaja "ModulePath", BUKAN "Module" —
	// supaya tidak ambigu dengan ModuleName (nama modul bisnis) di bawah.
	// Dipakai sebagai module path go.mod langsung (mode tanpa modul) atau
	// sebagai module path deps/ (composition root, mode dengan modul).
	ModulePath string
	// ModuleName, kalau diisi, membangun WORKSPACE go.work penuh dengan
	// SATU modul bisnis default yang berdiri sendiri — lihat
	// newProjectWithModule untuk detail struktur. Kosong = project "tanpa
	// modul" (lihat Quick Start § Alternatif Tanpa Modul webcore-docs):
	// hanya main.go + go.mod + config.yaml, tidak ada deps/ atau modules/
	// sama sekali — tambahkan layer (entity/repository/service/handler)
	// nanti lewat "make entity/repository/..." tanpa --module.
	ModuleName string
	// SkipGoGet melewati `go get github.com/webcore-go/webcore` (mis. saat
	// tidak ada koneksi internet).
	SkipGoGet bool
}

// NewProject membuat project webcore baru. Dua struktur, tergantung
// ModuleName — lihat newProjectWithoutModule/newProjectWithModule.
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

	// Pindah ke dalam project — semua langkah sesudah ini (termasuk
	// NewModule, yang mendeteksi go.mod/go.work lewat cwd) memakai path
	// relatif terhadap root project ini.
	origDir, err := os.Getwd()
	if err != nil {
		return err
	}
	if err := os.Chdir(opt.Dir); err != nil {
		return fmt.Errorf("masuk ke %s: %w", opt.Dir, err)
	}
	defer os.Chdir(origDir)

	if opt.ModuleName != "" {
		return newProjectWithModule(opt)
	}
	return newProjectWithoutModule(opt)
}

// newProjectWithoutModule membuat struktur "tanpa modul" — SATU go.mod di
// root, main.go (core.NewApp dengan packages=nil) + config.yaml saja,
// tidak ada deps/ atau modules/ sama sekali (lihat Quick Start §
// Alternatif Tanpa Modul webcore-docs).
func newProjectWithoutModule(opt NewProjectOptions) error {
	fmt.Printf("Membuat project %q (module %s, tanpa modul)...\n", opt.Dir, opt.ModulePath)

	if err := runIn(".", "go", "mod", "init", opt.ModulePath); err != nil {
		return fmt.Errorf("go mod init: %w", err)
	}

	data := map[string]string{
		"ModulePath":  opt.ModulePath,
		"ProjectName": opt.Dir,
	}

	files := map[string]string{
		"config.yaml":         projectConfigYAMLTmpl,
		"main.go":             projectMainGoNoModuleTmpl,
		".gitignore":          projectGitignoreTmpl,
		"Makefile":            projectMakefileTmpl,
		"migrations/.gitkeep": "",
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

	if !opt.SkipGoGet {
		fmt.Println("Mengambil github.com/webcore-go/webcore...")
		if err := runIn(".", "go", "get", "github.com/webcore-go/webcore@latest"); err != nil {
			fmt.Printf("  peringatan: go get gagal (%v) — jalankan manual nanti: cd %s && go get github.com/webcore-go/webcore@latest\n", err, opt.Dir)
		} else if err := runIn(".", "go", "mod", "tidy"); err != nil {
			fmt.Printf("  peringatan: go mod tidy gagal: %v\n", err)
		}
	}

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

Atau mulai project lain dengan modul penuh (workspace go.work):
  webcore-go-cli new <project-lain> --module=<nama>
`, opt.Dir, opt.Dir)

	return nil
}

// newProjectWithModule membangun WORKSPACE go.work penuh dengan SATU
// modul bisnis default yang berdiri sendiri — struktur yang sama seperti
// project yang sudah "naik kelas" ke multi-modul (lihat webcore-docs §
// Proyek Multi-Modul & Multi-Service), dibuat langsung dari awal:
//
//	<project>/
//	├── go.work                     # use (./deps ./modules/<pkg>)
//	├── config.yaml / access.yaml   # SEJAJAR go.work, bukan bagian go.mod manapun
//	├── .env-<pkg> / air-<pkg>.toml # dev env override + hot-reload
//	├── deps/                       # COMPOSITION ROOT, go.mod sendiri
//	│   ├── go.mod                  #   module <ModulePath>
//	│   ├── main.go                 #   dispatch "switch service" (siap ditambah modul lain)
//	│   └── <pkg>/
//	│       ├── libraries.go
//	│       └── packages.go
//	└── modules/
//	    └── <pkg>/                  # MODUL BISNIS, go.mod sendiri
//	        ├── go.mod               #   module <ModulePath>/mod-<pkg>
//	        └── ... (module.go, config/entity/repository/service/handler)
//
// Dipilih SENGAJA bukan struktur single-go.mod yang dinestkan di bawah
// root — supaya "make module <nama2> --module-path=... --deps-dir=deps"
// dan "remove module <nama> --deps-dir=deps" bisa langsung dipakai tanpa
// perlu migrasi struktur begitu project ini butuh modul kedua.
func newProjectWithModule(opt NewProjectOptions) error {
	pkg := names.Package(opt.ModuleName)
	standaloneModulePath := opt.ModulePath + "/mod-" + pkg

	fmt.Printf("Membuat project %q (workspace go.work, modul default %q)...\n", opt.Dir, pkg)

	if err := runIn(".", "go", "work", "init"); err != nil {
		return fmt.Errorf("go work init: %w", err)
	}

	rootData := map[string]string{
		"ModulePath":  opt.ModulePath,
		"ProjectName": opt.Dir,
		"ModuleName":  pkg,
	}
	rootFiles := map[string]string{
		"config.yaml":         projectConfigYAMLTmpl,
		"access.yaml":         projectAccessYAMLTmpl,
		".gitignore":          projectGitignoreTmpl,
		"Makefile":            projectWorkspaceMakefileTmpl,
		"migrations/.gitkeep": "",
	}
	for rel, tmpl := range rootFiles {
		content, err := render(tmpl, rootData)
		if err != nil {
			return fmt.Errorf("render %s: %w", rel, err)
		}
		if err := WriteFile(rel, content, false); err != nil {
			return err
		}
	}

	// deps/ — composition root, Go module berdiri sendiri (BUKAN dinestkan
	// di bawah go.mod project, karena di mode ini tidak ada go.mod di root
	// sama sekali — cuma go.work).
	if err := os.MkdirAll("deps", 0o755); err != nil {
		return fmt.Errorf("buat deps/: %w", err)
	}
	if err := runIn("deps", "go", "mod", "init", opt.ModulePath); err != nil {
		return fmt.Errorf("go mod init di deps/: %w", err)
	}

	mainGoContent, err := render(projectDepsMainGoTmpl, map[string]string{
		"Pkg":        pkg,
		"ModulePath": opt.ModulePath,
	})
	if err != nil {
		return fmt.Errorf("render deps/main.go: %w", err)
	}
	if err := WriteFile("deps/main.go", mainGoContent, false); err != nil {
		return err
	}

	// libraries.go/packages.go per-service persis seperti yang dibuat
	// WireDepsService (make module --deps-dir=deps) — dengan sengaja
	// dipakai template yang sama (depsLibrariesTmpl/depsPackagesTmpl di
	// depswiring.go), supaya hasilnya identik baik modul ini dibuat lewat
	// "new --module" (di sini) maupun ditambahkan belakangan lewat
	// "make module --deps-dir".
	depsWiringData := map[string]string{
		"Pkg":                pkg,
		"ModulePathOverride": standaloneModulePath,
	}
	depsWiringFiles := map[string]string{
		"libraries.go": depsLibrariesTmpl,
		"packages.go":  depsPackagesTmpl,
	}
	for rel, tmpl := range depsWiringFiles {
		content, err := render(tmpl, depsWiringData)
		if err != nil {
			return fmt.Errorf("render deps/%s/%s: %w", pkg, rel, err)
		}
		if err := WriteFile(filepath.Join("deps", pkg, rel), content, false); err != nil {
			return err
		}
	}

	if err := NewModule(pkg, standaloneModulePath, false); err != nil {
		return fmt.Errorf("buat modul default %s: %w", pkg, err)
	}

	if err := runIn(".", "go", "work", "use", "./deps", "./modules/"+pkg); err != nil {
		return fmt.Errorf("go work use: %w", err)
	}

	if !opt.SkipGoGet {
		fmt.Println("Mengambil github.com/webcore-go/webcore di deps/ dan modules/" + pkg + "/...")
		for _, dir := range []string{"deps", filepath.Join("modules", pkg)} {
			if err := runIn(dir, "go", "get", "github.com/webcore-go/webcore@latest"); err != nil {
				fmt.Printf("  peringatan: go get gagal di %s (%v) — jalankan manual nanti: cd %s/%s && go get github.com/webcore-go/webcore@latest\n", dir, err, opt.Dir, dir)
			}
		}
		// go mod tidy KHUSUS modules/<pkg> — bukan deps/. deps/go.mod
		// meng-import modules/<pkg> yang HANYA ada sebagai anggota go.work
		// (module path-nya tidak pernah di-publish), dan "go mod tidy"
		// (beda dari "go build") mencoba resolve require barunya lewat
		// VCS/proxy alih-alih lewat workspace, jadi selalu gagal kalau
		// tidak ada akses jaringan ke path itu — walau build-nya sendiri
		// sudah benar lewat go.work tanpa butuh require sama sekali
		// (dikonfirmasi cocok dengan app/go.mod di project nyata yang
		// memakai pola ini: tidak ada require utk modul workspace lokal).
		// "go work sync" di bawah sudah cukup menyelaraskan go.work.sum.
		if err := runIn(filepath.Join("modules", pkg), "go", "mod", "tidy"); err != nil {
			fmt.Printf("  peringatan: go mod tidy gagal di modules/%s: %v\n", pkg, err)
		}
		if err := runIn(".", "go", "work", "sync"); err != nil {
			fmt.Printf("  peringatan: go work sync gagal: %v\n", err)
		}
	}

	if err := GenerateDevEnv(pkg, "deps", false); err != nil {
		fmt.Printf("  peringatan: tidak bisa generate .env-%s/air-%s.toml (%v)\n", pkg, pkg, err)
	}

	fmt.Printf(`
Project %q siap sebagai workspace go.work dengan modul default %q.

Langkah selanjutnya:
  cd %s
  webcore-go-cli run %s          # atau: go run ./deps/main.go %s

Tambah modul lagi (jadi service baru di deps/main.go):
  webcore-go-cli make module <nama2> --module-path=%s/mod-<nama2> --deps-dir=deps
`, opt.Dir, pkg, opt.Dir, pkg, pkg, opt.ModulePath)

	return nil
}

func runIn(dir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

const projectDepsMainGoTmpl = `package main

import (
	"context"
	"fmt"
	"os"

	{{.Pkg}} "{{.ModulePath}}/{{.Pkg}}"
	"github.com/webcore-go/webcore/app/core"
	"github.com/webcore-go/webcore/app/out"
	"github.com/webcore-go/webcore/infra/config"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: <binary> <service>")
		os.Exit(1)
	}
	service := os.Args[1]

	validServices := map[string]bool{
		"{{.Pkg}}": true,
	}
	if !validServices[service] {
		fmt.Printf("service tidak dikenal: %s\n", service)
		os.Exit(1)
	}

	cfg := &config.Config{}
	if err := config.LoadDefaultConfig(cfg); err != nil {
		panic(err)
	}
	// penting: tanpa ini response error selalu bermode "development"
	// (stack trace & detail bocor ke klien)
	out.SetEnvironment(cfg.App.Environment)

	var libraries map[string]core.LibraryLoader
	var packages []core.Module

	switch service {
	case "{{.Pkg}}":
		libraries = {{.Pkg}}.APP_LIBRARIES
		packages = {{.Pkg}}.APP_PACKAGES
	default:
		fmt.Printf("service tidak dikenal: %s\n", service)
		os.Exit(1)
	}

	app := core.NewApp(context.Background(), cfg, libraries, packages)
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

// Target test/vet masuk ke tiap direktori module secara eksplisit (cd
// deps && ..., cd modules/<pkg> && ...) alih-alih "go test ./..." polos —
// go.work TIDAK mengizinkan wildcard "./..." di-root dari direktori
// workspace sendiri (yang tidak punya go.mod), atau dari direktori
// ANCESTOR sebuah module (mis. "./modules/..." ditolak, harus tepat
// "./modules/<pkg>/..."). "cd x && ..." di tiap baris Makefile aman —
// make menjalankan tiap baris resep di shell baru, jadi tidak perlu
// subshell "(...)" segala.
const projectWorkspaceMakefileTmpl = `.PHONY: run build test vet fmt sync

sync:
	go work sync

run: sync
	go run ./deps/main.go {{.ModuleName}}

build: sync
	go build -o ./bin/{{.ProjectName}} ./deps/main.go

test:
	cd deps && go test ./...
	cd modules/{{.ModuleName}} && go test ./...

vet:
	cd deps && go vet ./...
	cd modules/{{.ModuleName}} && go vet ./...

fmt:
	gofmt -l .
`
