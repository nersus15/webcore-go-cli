package scaffold

import (
	"fmt"
	"path/filepath"

	"github.com/nersus15/webcore-go-cli/internal/names"
)

// LibraryData adalah variabel template untuk make:library.
type LibraryData struct {
	Pkg    string // nama package, mis. "mailer"
	Entity string // PascalCase, mis. "Mailer" — dipakai utk IMailer/MailerConfig/MailerLoader
}

// NewLibrary membuat skeleton library kustom (interface+implementasi,
// config, loader) di libraries/<pkg>/, mengikuti pola port.Library/
// port.Connector yang didokumentasikan di webcore-docs §
// Membuat Library Kustom. Library dibuat sebagai package biasa di dalam
// module project ini; jadikan module Go terpisah sendiri (go mod init)
// kalau nanti mau dipublish/dipakai project lain — lihat dokumentasi
// Proyek Multi-Modul (go.work).
func NewLibrary(name string, force bool) error {
	pkg := names.Package(name)
	data := LibraryData{Pkg: pkg, Entity: names.Pascal(name)}

	base := filepath.Join("libraries", pkg)
	files := map[string]string{
		pkg + ".go": libraryImplTmpl,
		"config.go": libraryConfigTmpl,
		"loader.go": libraryLoaderTmpl,
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
Library %q siap di %s/.

Daftarkan di deps/libraries.go:

    import %s "<module-path-anda>/%s"

    var APP_LIBRARIES = map[string]core.LibraryLoader{
        "%s": &%s.%sLoader{},
    }

Lalu load dari modul pemakai lewat ctx.StartSingletonInstance("%s", cfg) —
lihat Membuat Library Kustom § Registrasi & Load.
`, pkg, base, pkg, base, pkg, pkg, data.Entity, pkg)

	return nil
}

const libraryImplTmpl = `package {{.Pkg}}

import "context"

// I{{.Entity}} adalah kontrak publik yang dilihat modul pemakai — definisikan
// hanya kalau tidak ada port.* bawaan yang cocok (lihat port/conn.go untuk
// IDatabase/ICacheMemory/IPubSub/IKafka).
type I{{.Entity}} interface {
	DoSomething(ctx context.Context) error
}

type {{.Entity}} struct {
	config {{.Entity}}Config
}

func New{{.Entity}}(cfg {{.Entity}}Config) *{{.Entity}} {
	return &{{.Entity}}{config: cfg}
}

// --- port.Library ---

// Install: validasi & persiapan, TANPA side-effect eksternal (koneksi dsb).
func (l *{{.Entity}}) Install(args ...any) error {
	return nil
}

func (l *{{.Entity}}) Uninstall() error { return nil }

// --- port.Connector (hapus kalau library ini tidak memegang koneksi) ---

func (l *{{.Entity}}) Connect() error {
	// TODO: buka koneksi eksternal di sini
	return nil
}

func (l *{{.Entity}}) Disconnect() error {
	// TODO: tutup koneksi — HARUS idempoten, aman dipanggil meski Connect
	// belum pernah terjadi (dipanggil otomatis saat app.Stop()).
	return nil
}

// --- I{{.Entity}} ---

func (l *{{.Entity}}) DoSomething(ctx context.Context) error {
	// TODO: implementasi
	return nil
}
`

const libraryConfigTmpl = `package {{.Pkg}}

// {{.Entity}}Config adalah section config kustom "{{.Pkg}}:" di config.yaml.
// Tempelkan ke config inti lewat config.LoadDefaultConfigWithAdditional
// (lihat Membuat Library Kustom § Konfigurasi).
type {{.Entity}}Config struct {
	Host string ` + "`mapstructure:\"host\"`" + `
}

func (c *{{.Entity}}Config) SetDefaults() map[string]any {
	return map[string]any{
		"{{.Pkg}}.host": "localhost",
	}
}

func (c *{{.Entity}}Config) SetEnvBindings() map[string]string {
	return map[string]string{
		"{{.Pkg}}.host": "{{.Pkg | upper}}_HOST",
	}
}
`

const libraryLoaderTmpl = `package {{.Pkg}}

import (
	"fmt"

	"github.com/webcore-go/webcore/port"
)

type {{.Entity}}Loader struct {
	name string
}

// SetName WAJIB pass-through — LibraryManager mengisinya dengan key dari
// APP_LIBRARIES; nama itulah identitas DI-nya.
func (l *{{.Entity}}Loader) SetName(name string) { l.name = name }
func (l *{{.Entity}}Loader) Name() string        { return l.name }

// Kontrak argumen (dokumentasikan untuk pemakai!): args[0] = {{.Entity}}Config
func (l *{{.Entity}}Loader) Init(args ...any) (port.Library, error) {
	cfg, ok := args[0].({{.Entity}}Config)
	if !ok {
		return nil, fmt.Errorf("{{.Entity}}Loader.Init: args[0] harus {{.Entity}}Config")
	}

	lib := New{{.Entity}}(cfg)
	if err := lib.Install(args...); err != nil {
		return nil, err
	}
	if err := lib.Connect(); err != nil {
		return nil, err
	}
	return lib, nil
}
`
