// Package gomod membaca module path dari go.mod project yang sedang
// dikerjakan, supaya generator tahu import path yang benar untuk
// package yang dibuatnya.
package gomod

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DetectModulePath mencari go.mod mulai dari direktori kerja saat ini,
// naik ke direktori induk kalau tidak ketemu di situ — sama seperti git
// atau go sendiri mencari root project, supaya perintah tetap jalan kalau
// dipanggil dari subfolder.
//
// Begitu ketemu, working directory PROSES INI dipindah (os.Chdir) ke
// direktori go.mod itu — supaya seluruh path relatif yang dipakai
// pemanggil sesudahnya (mis. "entity/", "modules/<nama>/") konsisten
// dihitung dari root project, bukan dari subfolder tempat command
// dijalankan. Ini penting: tanpa Chdir, file yang digenerate akan
// tersesat ke subfolder saat command dipanggil dari dalamnya.
//
// Berhenti dengan error yang jelas begitu ketemu go.work tanpa go.mod di
// direktori yang sama — itu tandanya project multi-module (lihat
// workspace.md § Proyek Multi-Modul): tidak ada satu go.mod yang menaungi
// seluruh repo, jadi menebak module path dari sana akan salah. Untuk kasus
// itu pemanggil harus pakai --module-path (module baru, berdiri sendiri)
// atau --module (scope ke modul yang sudah punya go.mod sendiri — lihat
// ModulePathAt).
func DetectModulePath() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("baca direktori kerja: %w", err)
	}

	for {
		if path, ok := ModulePathAt(dir); ok {
			if err := os.Chdir(dir); err != nil {
				return "", fmt.Errorf("pindah ke root project %s: %w", dir, err)
			}
			return path, nil
		}

		if _, err := os.Stat(filepath.Join(dir, "go.work")); err == nil {
			return "", fmt.Errorf(
				"ketemu go.work di %s tapi tidak ada go.mod di situ — project ini kemungkinan multi-module. "+
					"Untuk modul baru yang berdiri sendiri: pakai --module-path=<path-go-mod-baru>. "+
					"Untuk men-scope ke modul yang sudah punya go.mod sendiri: pakai --module=<nama>",
				dir,
			)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("go.mod tidak ditemukan di direktori ini maupun direktori induknya")
}

// ModulePathAt membaca go.mod TEPAT di dir (tanpa naik ke direktori
// induk) dan mengembalikan module path-nya. ok bernilai false kalau
// go.mod tidak ada di situ atau tidak bisa diparse — bukan error, karena
// pemanggil (resolveScope) memang memakainya untuk mengecek "apakah
// modul ini punya go.mod sendiri" sebagai bagian dari alur normal.
func ModulePathAt(dir string) (path string, ok bool) {
	f, err := os.Open(filepath.Join(dir, "go.mod"))
	if err != nil {
		return "", false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module")), true
		}
	}

	return "", false
}

// FindModuleUpward mencari go.mod mulai dari startDir, naik ke direktori
// induk kalau tidak ketemu di situ — sama seperti DetectModulePath, tapi
// TIDAK melakukan os.Chdir dan beroperasi di direktori yang dikasih
// (bukan cwd proses) — dipakai untuk mencari go.mod milik direktori LAIN
// dari module project utama yang sedang dikerjakan (mis. composition
// root "app/" saat module baru sedang dibuat di "modules/<pkg>/").
// Mengembalikan direktori root yang ketemu beserta module path-nya.
func FindModuleUpward(startDir string) (root, path string, err error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", "", fmt.Errorf("baca path %s: %w", startDir, err)
	}

	for {
		if p, ok := ModulePathAt(dir); ok {
			return dir, p, nil
		}

		if _, err := os.Stat(filepath.Join(dir, "go.work")); err == nil {
			return "", "", fmt.Errorf("ketemu go.work di %s tapi tidak ada go.mod di situ maupun di induk %s", dir, startDir)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", "", fmt.Errorf("go.mod tidak ditemukan di %s maupun direktori induknya", startDir)
}

// WorkspaceGoVersion mencari go.work mulai dari dir naik ke direktori
// induk, dan mengembalikan versi Go yang dideklarasikannya (baris "go
// X.Y.Z"). Dipakai supaya go.mod baru yang dibuat "go mod init" untuk
// module standalone (--module-path) ikut versi go.work, bukan versi
// default toolchain lokal — kalau tidak disamakan, "go build" di
// workspace itu akan menolak dengan "requires go >= X tapi go.work
// lists Y". ok bernilai false kalau tidak ada go.work ditemukan.
func WorkspaceGoVersion(dir string) (version string, ok bool) {
	d, err := filepath.Abs(dir)
	if err != nil {
		return "", false
	}

	for {
		content, err := os.ReadFile(filepath.Join(d, "go.work"))
		if err == nil {
			scanner := bufio.NewScanner(strings.NewReader(string(content)))
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if strings.HasPrefix(line, "go ") {
					return strings.TrimSpace(strings.TrimPrefix(line, "go")), true
				}
			}
			return "", false
		}

		parent := filepath.Dir(d)
		if parent == d {
			return "", false
		}
		d = parent
	}
}

// HasWorkspace mencari go.work mulai dari dir naik ke direktori induk —
// dipakai sebelum mencoba "go work use" otomatis (lihat NewModule mode
// standalone), supaya tidak menjalankan command itu di project yang
// memang tidak memakai workspace sama sekali.
func HasWorkspace(dir string) bool {
	d, err := filepath.Abs(dir)
	if err != nil {
		return false
	}

	for {
		if _, err := os.Stat(filepath.Join(d, "go.work")); err == nil {
			return true
		}

		parent := filepath.Dir(d)
		if parent == d {
			return false
		}
		d = parent
	}
}
