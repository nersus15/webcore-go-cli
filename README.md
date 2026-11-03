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
| `rename-module <path-baru>` | Ganti module path (go.mod) + perbaiki semua import ke path lama, termasuk di module lain dalam go.work yang sama |
| `run <module>` | Jalankan `air -c air-<module>.toml` (dev, hot-reload) — error kalau filenya belum ada |

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

`go mod init` yang dijalankan otomatis untuk module baru ikut menyamakan versi Go dengan yang dideklarasikan `go.work` (bukan selalu versi toolchain lokal) — supaya tidak langsung error "requires go >= X tapi go.work lists Y".

### Wiring per-service (`--deps-dir`), untuk pola composition-root multi-service

Kalau project Anda punya composition root yang dispatch per-service lewat argumen CLI (satu `main.go` yang `switch service { case "a": ...; case "b": ...; default: }`, dengan tiap service punya folder `<deps>/<service>/{libraries,packages}.go` sendiri — lihat contoh nyata di [workspace.md](https://github.com/webcore-go/webcore) § Composition Root), tambahkan `--deps-dir=<nama-folder>` ke `make module --module-path=...`:

```bash
webcore-go-cli make module satusehat \
  --module-path=github.com/acme/mod-satusehat \
  --deps-dir=app
```

Ini akan:

1. Generate `app/satusehat/libraries.go` dan `app/satusehat/packages.go` (file baru, selalu aman).
2. **Coba** mengedit `app/main.go` secara otomatis lewat AST Go (bukan regex) — menambahkan import, entry `"satusehat": true` ke map `validServices`-nya, dan `case "satusehat":` ke switch dispatch-nya, tepat sebelum `default:`. Hasilnya diverifikasi lewat `go/format` sebelum ditulis.
3. Generate `.env-satusehat` (checklist semua env var config webcore, SEMUA ter-comment dengan contoh default value-nya — lihat bagian selanjutnya) dan `air-satusehat.toml` (siap pakai lewat `webcore-go-cli run satusehat`).

Langkah 2 **best-effort**: kalau polanya (map `map[string]bool` atau switch `switch x { case "a": ... default: }`) tidak ketemu atau ambigu (lebih dari satu kandidat) di `main.go`, CLI tidak menebak — cukup mencetak snippet yang harus ditambahkan manual. Nama folder `<deps-dir>` bebas (`deps`, `app`, dll.) — bukan konvensi yang di-hardcode. `air-<nama>.toml`/`.env-<nama>` selalu ditaruh di direktori kerja saat ini (root workspace), bukan di dalam `<deps-dir>` — mengikuti konvensi `air.toml` yang biasanya sejajar dengan `go.work`.

Diverifikasi terhadap project `go.work` multi-service nyata (composition root dengan `validServices` map + switch per-service + override port) — hasil edit `main.go` tetap `go build` bersih, komentar dan kode lain yang sudah ada tidak terganggu, dan hasil `air-<nama>.toml`-nya benar-benar bisa dijalankan sampai binary start.

### Checklist env var (`.env-<module>`)

Daftar env var di `.env-<module>` diambil dari tabel statis yang disalin dari `infra/config/binding.go` + `infra/config/defaults.go` webcore (termasuk field yang belum terpublish ke proxy publik) — **bukan** di-parse dari `config.yaml`/`config.yaml.example` project Anda, supaya hasilnya konsisten di project mana pun terlepas dari isi file config yang kebetulan ada. Semua baris sengaja di-comment; nilai di sebelah kanan adalah default bawaan webcore, bukan nilai aktif — uncomment & sesuaikan yang relevan saja, mis.:

```bash
# .env-satusehat
AUTH_TYPE=none
```

### Menjalankan dev (`run`)

```bash
webcore-go-cli run satusehat
```

Sama dengan `air -c air-satusehat.toml` — error jelas kalau file itu belum ada (generate dulu lewat `make module ... --deps-dir=...`). Binary `air` dicari lewat `PATH` dulu, lalu fallback ke `~/air` — banyak setup nge-alias `air` ke binary yang didownload manual ke home directory, dan alias shell semacam itu tidak pernah ter-resolve lewat pencarian `PATH` biasa (`exec.Command` tidak lewat shell, jadi alias tidak berlaku).

### Mengganti module path (`rename-module`)

Butuh ganti module path setelah project sudah berjalan (mis. salah nama waktu `go mod init`, atau pindah org)? `rename-module`:

1. Mengganti baris `module` di `go.mod` target.
2. Mencari & memperbaiki semua import ke path lama — di dalam module itu sendiri, **dan** di module lain dalam workspace `go.work` yang sama yang memakainya sebagai dependency (pemanggil lintas-modul).
3. Menjalankan `go mod tidy` di tiap module yang tersentuh.

```bash
webcore-go-cli rename-module github.com/acme/my-api-v2        # target: go.mod root project
webcore-go-cli rename-module github.com/acme/mod-orders --module=orders  # target: modules/orders/go.mod

webcore-go-cli rename-module github.com/acme/my-api-v2 --dry-run  # lihat dulu file apa saja yang akan kesentuh
```

`--module=<nama>` cuma berlaku untuk modul yang sudah punya `go.mod` sendiri (dibuat lewat `make module --module-path=...`) — bukan modul klasik yang dinestkan di bawah go.mod root.

Tidak menjangkau repo/module di luar workspace `go.work` saat ini — perbaiki manual kalau ada project lain (di luar workspace ini) yang juga meng-import module yang di-rename.

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
