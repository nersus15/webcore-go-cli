# webcore-go-cli

CLI scaffolding untuk framework [WebCore](https://github.com/webcore-go/webcore) — peran yang dimainkannya setara `php artisan` di Laravel: membuat project baru dan menggenerate boilerplate (modul, config, entity, repository, service, handler, helper, library, migration) supaya tidak ditulis tangan berulang-ulang.

Semua template digenerate mengikuti pola yang didokumentasikan di [webcore-docs](https://github.com/webcore-go/webcore) (§ Pengembangan Modul, § Konfigurasi, § Membuat Library Kustom, § Migrasi Database), dan hasilnya diverifikasi benar-benar compile terhadap `github.com/webcore-go/webcore` sungguhan sebelum dirilis.

## Instalasi

```bash
go install github.com/nersus15/webcore-go-cli@latest
```

Menghasilkan binary bernama `webcore-go-cli`.

## Shell Completion

CLI ini dibangun dengan [Cobra](https://github.com/spf13/cobra), jadi completion tab seperti git/kubectl sudah otomatis tersedia — termasuk completion **dinamis**: `--module=<TAB>` menyarankan nama modul yang sungguh ada di `modules/` project saat ini, bukan daftar statis.

```bash
# bash (taruh di ~/.bashrc)
command -v webcore-go-cli >/dev/null 2>&1 && source <(webcore-go-cli completion bash)

# zsh (taruh di ~/.zshrc)
command -v webcore-go-cli >/dev/null 2>&1 && source <(webcore-go-cli completion zsh)
```

Shell lain: `webcore-go-cli completion --help`.

## Perintah

Jalankan `webcore-go-cli --help` untuk daftar lengkap. Ringkasan:

| Perintah | Fungsi |
| --- | --- |
| `new <nama-project>` | Project baru lengkap (go.mod, main.go, config.yaml, deps/) |
| `make module <Nama>` | Modul lengkap: module.go + config/entity/repository/service/handler |
| `make config <Nama>` | `ConfigObject` satu-file (`SetDefaults`/`SetEnvBindings`) |
| `make entity <Nama>` | Struct entity + Input |
| `make repository <Nama>` | Repository di atas `port.IDatabase` |
| `make service <Nama>` | Service layer |
| `make handler <Nama>` | Fiber handler (list/get/create) |
| `make helper <Nama>` | Package helper mandiri |
| `make library <Nama>` | Skeleton library kustom (`port.Library`/`Connector`/`LibraryLoader`) |
| `make migration <nama>` | File migrasi goose (`NNNNN_nama.sql`, auto-increment) |

Semua subcommand `make` (kecuali `new`) membaca module path dari `go.mod` project — dicari mulai dari direktori kerja saat ini, naik ke direktori induk kalau perlu (jadi tetap jalan dipanggil dari subfolder).

> Subcommand `make` sengaja pakai spasi (`make module`), bukan titik dua (`make:module`) — titik dua ada di `COMP_WORDBREAKS` default bash, dan akan membuat completion tab tidak ter-insert dengan benar.

### Dua mode penempatan

`make config`/`make entity`/`make repository`/`make service`/`make handler` bisa diarahkan ke dua tempat:

- **Tanpa `--module`** → rata di root project (`entity/`, `repository/`, dst.) — pola ["tanpa modul"](https://github.com/webcore-go/webcore) untuk app kecil/prototipe.
- **Dengan `--module=<nama>`** → masuk ke `modules/<nama>/<kind>/` milik modul yang sudah ada — mis. menambah entity kedua ke modul yang sudah dibuat `make module`.

Flag boleh diletakkan sebelum atau sesudah argumen nama:

```bash
webcore-go-cli make module Orders
webcore-go-cli make entity OrderItem --module=orders
webcore-go-cli make entity OrderItem --module=orders --force   # timpa kalau sudah ada
```

### Project multi-module (go.work)

Kalau project Anda memakai [go.work](https://go.dev/ref/mod#workspaces) dengan tiap modul punya `go.mod` sendiri (lihat [workspace.md](https://github.com/webcore-go/webcore) § Proyek Multi-Modul) — tidak ada satu `go.mod` yang menaungi seluruh repo, jadi perlu sedikit berbeda:

- **`make module <Nama> --module-path=<path>`** — buat modul baru yang berdiri sendiri (jalankan `go mod init <path>` otomatis di `modules/<nama>/`). `<path>` bisa berbeda total dari nama direktorinya — nama direktori cuma label folder, bukan sumber import path.

  ```bash
  webcore-go-cli make module satusehat --module-path=github.com/acme/mod-satusehat
  ```

- **`make config/entity/repository/service/handler <Nama> --module=<nama>`** — kalau `modules/<nama>/` sudah punya `go.mod` sendiri, CLI otomatis membaca module path ASLI dari situ (tidak pernah menebak dari nama direktori). Tidak perlu flag tambahan apa pun — perilakunya otomatis berbeda dari mode nested klasik begitu `go.mod` terdeteksi.

Kalau dijalankan dari root project (ada `go.work`, tidak ada `go.mod` di situ) tanpa `--module-path` atau `--module` yang sesuai, CLI akan menolak dengan pesan error yang menjelaskan opsi mana yang harus dipakai — bukan asal tebak.

## Contoh Alur Penuh

```bash
webcore-go-cli new my-api
cd my-api

webcore-go-cli make module orders          # modul lengkap pertama
# daftarkan manual di deps/packages.go (CLI mencetak snippet-nya)

webcore-go-cli make entity OrderItem --module=orders   # entity kedua di modul yang sama
webcore-go-cli make migration create_orders_table --module=orders
webcore-go-cli make library mailer                      # library kustom

go run main.go
```

## Catatan

- Generator **tidak pernah menimpa file yang sudah ada** kecuali `--force` — aman dijalankan berulang.
- Semua output `.go` dilewatkan `go/format` (library resmi di balik `gofmt`) sebelum ditulis — kalau ada yang gagal format, itu artinya ada bug di template, bukan masalah file Anda.
- Konten yang digenerate sengaja generik (struktur CRUD + comment `// TODO` di beberapa tempat) — sesuaikan dengan kebutuhan bisnis sebenarnya, terutama skema tabel dan validasi.
