package scaffold

import (
	"bufio"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/nersus15/webcore-go-cli/internal/gomod"
	"github.com/nersus15/webcore-go-cli/internal/names"
)

// RenameResult melaporkan apa yang disentuh RenameModule — dipakai juga
// untuk --dry-run (isinya sama, tapi tidak ada yang ditulis ke disk).
type RenameResult struct {
	OldPath    string
	NewPath    string
	GoModFile  string
	ChangedGo  []string // file .go yang import-nya ikut diubah
	TidyErrors []string // error go mod tidy per-direktori (best-effort, bukan fatal)
}

// RenameModule mengganti module path sebuah module Go dan memperbaiki
// SEMUA import ke path lama: baik self-import di dalam module itu sendiri
// (mis. "<old>/entity"), MAUPUN import dari module lain dalam workspace
// go.work yang sama yang memakainya sebagai dependency ("pemanggil lintas
// modul"). Rename-nya TIDAK menjangkau repo/module di luar workspace
// go.work saat ini — itu di luar kendali tool ini, lihat pesan di output.
//
// moduleScope kosong = target root project (go.mod dicari lewat
// gomod.DetectModulePath, naik ke direktori induk kalau perlu).
// moduleScope diisi = target modules/<moduleScope>/go.mod, yang harus
// sudah punya go.mod sendiri (pola go.work, lihat workspace.md).
func RenameModule(newPath, moduleScope string, dryRun bool) (*RenameResult, error) {
	targetDir, oldPath, err := resolveRenameTarget(moduleScope)
	if err != nil {
		return nil, err
	}
	if oldPath == newPath {
		return nil, fmt.Errorf("module path sudah %q, tidak ada yang diubah", newPath)
	}

	searchDirs := []string{targetDir}
	if workDirs, err := workspaceModuleDirs(targetDir); err == nil {
		for _, d := range workDirs {
			if !sameDir(d, targetDir) {
				searchDirs = append(searchDirs, d)
			}
		}
	}

	changedFiles, err := findImportingFiles(searchDirs, oldPath)
	if err != nil {
		return nil, err
	}

	result := &RenameResult{
		OldPath:   oldPath,
		NewPath:   newPath,
		GoModFile: filepath.Join(targetDir, "go.mod"),
		ChangedGo: changedFiles,
	}

	if dryRun {
		return result, nil
	}

	if err := rewriteGoModPath(targetDir, oldPath, newPath); err != nil {
		return nil, err
	}

	for _, f := range changedFiles {
		if _, err := rewriteImportsInFile(f, oldPath, newPath); err != nil {
			return nil, fmt.Errorf("ubah import di %s: %w", f, err)
		}
	}

	tidyDirs := map[string]bool{targetDir: true}
	for _, f := range changedFiles {
		tidyDirs[moduleDirOf(f, searchDirs)] = true
	}
	for d := range tidyDirs {
		if err := runIn(d, "go", "mod", "tidy"); err != nil {
			result.TidyErrors = append(result.TidyErrors, fmt.Sprintf("%s: %v", d, err))
		}
	}

	return result, nil
}

func sameDir(a, b string) bool {
	ra, err1 := filepath.Abs(a)
	rb, err2 := filepath.Abs(b)
	return err1 == nil && err2 == nil && ra == rb
}

func moduleDirOf(file string, candidates []string) string {
	best := filepath.Dir(file)
	for _, c := range candidates {
		rel, err := filepath.Rel(c, file)
		if err == nil && !strings.HasPrefix(rel, "..") {
			return c
		}
	}
	return best
}

func resolveRenameTarget(moduleScope string) (dir, oldPath string, err error) {
	if moduleScope == "" {
		path, err := gomod.DetectModulePath() // efek samping: chdir ke root kalau ketemu di induk
		if err != nil {
			return "", "", err
		}
		root, err := os.Getwd()
		if err != nil {
			return "", "", err
		}
		return root, path, nil
	}

	scopePkg := names.Package(moduleScope)
	dir = filepath.Join("modules", scopePkg)
	path, ok := gomod.ModulePathAt(dir)
	if !ok {
		return "", "", fmt.Errorf(
			"modules/%s tidak punya go.mod sendiri — rename-module hanya berlaku untuk module yang sudah ada go.mod-nya (lihat --module-path di 'make module' untuk membuat module baru)",
			scopePkg,
		)
	}
	return dir, path, nil
}

// workspaceModuleDirs mencari go.work mulai dari startDir naik ke induk,
// dan mengembalikan direktori absolut semua module yang didaftarkan di
// blok use()-nya. Mengembalikan error (bukan fatal bagi pemanggil) kalau
// tidak ada go.work — artinya ini bukan project workspace, jadi
// pencariannya cukup di target module itu sendiri saja.
func workspaceModuleDirs(startDir string) ([]string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return nil, err
	}

	for {
		workPath := filepath.Join(dir, "go.work")
		content, err := os.ReadFile(workPath)
		if err == nil {
			return parseGoWorkUse(string(content), dir), nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return nil, fmt.Errorf("go.work tidak ditemukan")
}

func parseGoWorkUse(content, workDir string) []string {
	var dirs []string
	inUse := false

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if idx := strings.Index(line, "//"); idx >= 0 {
			line = strings.TrimSpace(line[:idx])
		}
		if line == "" {
			continue
		}

		switch {
		case strings.HasPrefix(line, "use ("):
			inUse = true
		case inUse && line == ")":
			inUse = false
		case inUse:
			dirs = append(dirs, filepath.Join(workDir, line))
		case strings.HasPrefix(line, "use "):
			dirs = append(dirs, filepath.Join(workDir, strings.TrimSpace(strings.TrimPrefix(line, "use "))))
		}
	}

	return dirs
}

// findImportingFiles menyisir setiap dir (rekursif) untuk file .go yang
// meng-import tepat oldPath atau salah satu sub-package-nya
// ("oldPath/sub"), tidak termasuk paruh string yang cuma kebetulan sama
// prefix-nya (mis. "oldPath2" tidak akan match).
func findImportingFiles(dirs []string, oldPath string) ([]string, error) {
	pattern := importPattern(oldPath)
	var matched []string
	seen := map[string]bool{}

	for _, dir := range dirs {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() || !strings.HasSuffix(path, ".go") {
				return nil
			}

			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if pattern.Match(content) {
				abs, err := filepath.Abs(path)
				if err != nil {
					abs = path
				}
				if !seen[abs] {
					seen[abs] = true
					matched = append(matched, path)
				}
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("baca direktori %s: %w", dir, err)
		}
	}

	return matched, nil
}

func importPattern(path string) *regexp.Regexp {
	return regexp.MustCompile(`"` + regexp.QuoteMeta(path) + `(/[^"]*)?"`)
}

func rewriteGoModPath(dir, oldPath, newPath string) error {
	modPath := filepath.Join(dir, "go.mod")
	content, err := os.ReadFile(modPath)
	if err != nil {
		return fmt.Errorf("baca %s: %w", modPath, err)
	}

	lines := strings.Split(string(content), "\n")
	replaced := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "module ") && strings.TrimSpace(strings.TrimPrefix(trimmed, "module")) == oldPath {
			lines[i] = "module " + newPath
			replaced = true
			break
		}
	}
	if !replaced {
		return fmt.Errorf("baris 'module %s' tidak ditemukan di %s", oldPath, modPath)
	}

	return os.WriteFile(modPath, []byte(strings.Join(lines, "\n")), 0o644)
}

// rewriteImportsInFile mengganti setiap import oldPath (atau sub-package-
// nya) jadi newPath di satu file, lalu memastikan hasilnya tetap Go yang
// valid lewat go/format sebelum ditulis — kalau rewrite-nya somehow
// merusak sintaks, ketahuan di sini sebagai error, bukan jadi file rusak.
func rewriteImportsInFile(path, oldPath, newPath string) (changed bool, err error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}

	pattern := importPattern(oldPath)
	newContent := pattern.ReplaceAll(content, []byte(`"`+newPath+`${1}"`))

	if string(newContent) == string(content) {
		return false, nil
	}

	formatted, err := format.Source(newContent)
	if err != nil {
		return false, fmt.Errorf("hasil rewrite bukan Go yang valid: %w", err)
	}

	return true, os.WriteFile(path, formatted, 0o644)
}
