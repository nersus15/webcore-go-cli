package scaffold

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/nersus15/webcore-go-cli/internal/names"
)

// RemoveResult melaporkan apa yang disentuh RemoveModule — dipakai juga
// untuk --dry-run (isinya sama, tapi tidak ada yang ditulis/dihapus).
type RemoveResult struct {
	Pkg          string
	ModulesDir   string // modules/<pkg> — kosong kalau tidak ketemu
	DepsDir      string // <deps>/<pkg> — kosong kalau tidak ketemu/tidak relevan
	EnvFile      string // .env-<pkg>
	AirFile      string // air-<pkg>.toml
	MainGoFile   string // <deps>/main.go yang DICOBA dibersihkan (kosong kalau --deps-dir tidak diisi)
	MainGoEdited bool   // true kalau AST-edit berhasil
	MainGoNote   string // instruksi manual kalau AST-edit gagal/tidak relevan
}

// RemoveModule menghapus semua yang dibuat "make module <nama> ...":
//
//  1. modules/<pkg>/
//  2. <deps>/<pkg>/ — HANYA kalau depsDir diisi (pola composition-root-
//     per-service dari --deps-dir di "make module"); tanpa itu, dicoba
//     fallback ke lokasi default pola nested dari "new --module=<pkg>",
//     yakni deps/<pkg>.
//  3. Entry modul ini di switch service dan map validServices di
//     <deps>/main.go — best-effort lewat AST, HANYA kalau depsDir diisi
//     (pola nested/single-module dari "new" tidak punya switch/map ini
//     sama sekali, jadi tidak ada yang perlu dibersihkan di situ).
//  4. .env-<pkg> dan air-<pkg>.toml
//
// TIDAK menyentuh go.work — kalau modul ini standalone (punya go.mod
// sendiri), pemanggil harus hapus manual entry "./modules/<pkg>" dari
// blok use()-nya (konsisten dengan NewModule/RenameModule: tool ini tidak
// pernah mengedit go.work otomatis).
func RemoveModule(name, depsDir string, dryRun bool) (*RemoveResult, error) {
	pkg := names.Package(name)
	result := &RemoveResult{Pkg: pkg}

	modulesDir := filepath.Join("modules", pkg)
	if pathExists(modulesDir) {
		result.ModulesDir = modulesDir
	}

	envFile := fmt.Sprintf(".env-%s", pkg)
	if pathExists(envFile) {
		result.EnvFile = envFile
	}
	airFile := fmt.Sprintf("air-%s.toml", pkg)
	if pathExists(airFile) {
		result.AirFile = airFile
	}

	var importPath string
	if depsDir != "" {
		root, targetDir, depsModulePath, err := resolveDepsPaths(depsDir, pkg)
		if err != nil {
			return nil, err
		}
		if pathExists(targetDir) {
			result.DepsDir = targetDir
		}
		importPath, err = depsImportPath(root, targetDir, depsModulePath)
		if err != nil {
			return nil, err
		}
		result.MainGoFile = filepath.Join(root, "main.go")
	} else if defaultDeps := filepath.Join("deps", pkg); pathExists(defaultDeps) {
		result.DepsDir = defaultDeps
	}

	if result.ModulesDir == "" && result.DepsDir == "" && result.EnvFile == "" && result.AirFile == "" {
		return nil, fmt.Errorf(
			"tidak ada artefak modul %q ditemukan (sudah dicek: modules/%s, deps/%s, .env-%s, air-%s.toml)",
			pkg, pkg, pkg, pkg, pkg,
		)
	}

	if dryRun {
		return result, nil
	}

	if result.ModulesDir != "" {
		if err := os.RemoveAll(result.ModulesDir); err != nil {
			return nil, fmt.Errorf("hapus %s: %w", result.ModulesDir, err)
		}
	}
	if result.DepsDir != "" {
		if err := os.RemoveAll(result.DepsDir); err != nil {
			return nil, fmt.Errorf("hapus %s: %w", result.DepsDir, err)
		}
	}
	if result.EnvFile != "" {
		if err := os.Remove(result.EnvFile); err != nil {
			return nil, fmt.Errorf("hapus %s: %w", result.EnvFile, err)
		}
	}
	if result.AirFile != "" {
		if err := os.Remove(result.AirFile); err != nil {
			return nil, fmt.Errorf("hapus %s: %w", result.AirFile, err)
		}
	}

	if result.MainGoFile != "" {
		edited, note, err := unwireMainGo(result.MainGoFile, pkg, importPath)
		if err != nil {
			result.MainGoNote = fmt.Sprintf(
				"tidak bisa otomatis membersihkan %s (%v) — hapus manual case %q dan "+
					"entry validServices-nya, serta baris import %q.",
				result.MainGoFile, err, pkg, importPath,
			)
		} else {
			result.MainGoEdited = edited
			result.MainGoNote = note
		}
	}

	return result, nil
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// depsImportPath menghitung import path persis seperti WireDepsService
// saat menambahkan service ini — dibutuhkan supaya unwireMainGo tahu baris
// import mana yang harus dihapus dari <deps>/main.go.
func depsImportPath(root, targetDir, depsModulePath string) (string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("baca path %s: %w", root, err)
	}
	absTarget, err := filepath.Abs(targetDir)
	if err != nil {
		return "", fmt.Errorf("baca path %s: %w", targetDir, err)
	}

	relSuffix, err := filepath.Rel(absRoot, absTarget)
	if err != nil {
		return "", fmt.Errorf("hitung path relatif %s dari %s: %w", targetDir, root, err)
	}
	if relSuffix == "." {
		return depsModulePath, nil
	}
	return depsModulePath + "/" + filepath.ToSlash(relSuffix), nil
}

// unwireMainGo adalah kebalikan dari wireMainGo: menghapus case %q di
// switch service, entry %q di map validServices, dan (kalau ketemu dan
// AMAN) baris import importPath — lewat AST, bukan regex, supaya
// posisinya pasti benar dan hasilnya pasti Go yang valid. Gagal
// (dikembalikan sebagai error) kalau case atau entry-nya tidak ketemu
// PERSIS SATU kali — pemanggil (RemoveModule) menangkap error ini sebagai
// instruksi manual, BUKAN fatal bagi keseluruhan operasi (file modul/deps
// sudah terhapus duluan).
//
// Import HANYA dihapus kalau identifier paketnya (pkg) tidak dipakai di
// mana pun selain di dalam case yang sedang dihapus — pola main.go nyata
// kadang punya "default:" yang diam-diam menduplikasi pemakaian salah
// satu case (lihat contoh integrasi-stunting) alih-alih kosong seperti
// template bawaan; menghapus importnya begitu saja di kasus itu akan
// merusak build ("undefined: pkg"). Kalau masih dipakai di luar situ,
// import dibiarkan dan note non-fatal dikembalikan supaya operator cek
// manual.
func unwireMainGo(mainGoPath, pkg, importPath string) (edited bool, note string, err error) {
	src, err := os.ReadFile(mainGoPath)
	if err != nil {
		return false, "", fmt.Errorf("baca %s: %w", mainGoPath, err)
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, mainGoPath, src, parser.ParseComments)
	if err != nil {
		return false, "", fmt.Errorf("parse %s: %w", mainGoPath, err)
	}

	caseStart, caseEnd, err := findSwitchCaseRange(file, pkg)
	if err != nil {
		return false, "", err
	}
	mapStart, mapEnd, err := findValidServicesEntryRange(file, pkg)
	if err != nil {
		return false, "", err
	}

	type byteSpan struct{ start, end int }
	offsetOf := func(p token.Pos) int { return fset.Position(p).Offset }

	spans := []byteSpan{
		{offsetOf(caseStart), offsetOf(caseEnd)},
		{offsetOf(mapStart), offsetOf(mapEnd)},
	}

	if impStart, impEnd, ok := findImportSpecRange(file, importPath); ok {
		if identUsedOutsideRange(file, pkg, caseStart, caseEnd) {
			note = fmt.Sprintf(
				"import %q sengaja TIDAK dihapus dari %s — identifier %q masih dipakai "+
					"di luar case yang baru dihapus (mis. di \"default:\"), cek manual apakah "+
					"pemakaian itu seharusnya ikut dibersihkan atau diganti.",
				importPath, mainGoPath, pkg,
			)
		} else {
			s, e := extendLineRemoval(src, offsetOf(impStart), offsetOf(impEnd))
			spans = append(spans, byteSpan{s, e})
		}
	}

	// urutkan dari posisi paling akhir ke paling awal, supaya penghapusan
	// sebelumnya tidak mengubah offset byte untuk penghapusan berikutnya.
	sort.Slice(spans, func(i, j int) bool { return spans[i].start > spans[j].start })

	result := append([]byte(nil), src...)
	for _, sp := range spans {
		result = append(result[:sp.start:sp.start], result[sp.end:]...)
	}

	formatted, err := format.Source(result)
	if err != nil {
		return false, "", fmt.Errorf("hasil edit bukan Go yang valid: %w", err)
	}

	return true, note, os.WriteFile(mainGoPath, formatted, 0o644)
}

// identUsedOutsideRange mengecek apakah identifier "name" dipakai di mana
// pun di file INI, DI LUAR jangkauan [rangeStart, rangeEnd) — dipakai
// unwireMainGo untuk memastikan menghapus import tidak meninggalkan
// pemakaian identifier yang sekarang tidak terdefinisi.
func identUsedOutsideRange(file *ast.File, name string, rangeStart, rangeEnd token.Pos) bool {
	used := false
	ast.Inspect(file, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if !ok || id.Name != name {
			return true
		}
		if id.Pos() >= rangeStart && id.Pos() < rangeEnd {
			return true // di dalam case yang memang sedang dihapus, tidak dihitung
		}
		used = true
		return true
	})
	return used
}

// findSwitchCaseRange mencari case %q di switch validServices (lihat
// findValidServicesSwitch di depswiring.go) dan mengembalikan jangkauan
// byte [start, end) yang kalau dihapus, membuang case itu beserta
// body-nya secara bersih — end adalah posisi clause SETELAHNYA (atau
// "}" penutup switch kalau ini clause terakhir), sehingga whitespace/
// koma di antaranya ikut terbuang (go/format merapikan sisanya).
func findSwitchCaseRange(file *ast.File, pkg string) (start, end token.Pos, err error) {
	sw, err := findValidServicesSwitch(file)
	if err != nil {
		return 0, 0, err
	}

	idx := -1
	for i, stmt := range sw.Body.List {
		cc, ok := stmt.(*ast.CaseClause)
		if !ok || len(cc.List) != 1 {
			continue
		}
		lit, ok := cc.List[0].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			continue
		}
		if val, uerr := strconv.Unquote(lit.Value); uerr == nil && val == pkg {
			idx = i
			break
		}
	}
	if idx == -1 {
		return 0, 0, fmt.Errorf("case %q tidak ditemukan di switch service", pkg)
	}

	start = sw.Body.List[idx].Pos()
	if idx+1 < len(sw.Body.List) {
		end = sw.Body.List[idx+1].Pos()
	} else {
		end = sw.Body.Rbrace
	}
	return start, end, nil
}

// findValidServicesEntryRange mencari entry %q: true di map validServices
// (lihat findValidServicesMap di depswiring.go) dan mengembalikan
// jangkauan byte [start, end) dengan logika yang sama seperti
// findSwitchCaseRange — end adalah Pos() elemen berikutnya, atau "}"
// penutup map kalau ini elemen terakhir.
func findValidServicesEntryRange(file *ast.File, pkg string) (start, end token.Pos, err error) {
	cl, err := findValidServicesMap(file)
	if err != nil {
		return 0, 0, err
	}

	idx := -1
	for i, elt := range cl.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		lit, ok := kv.Key.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			continue
		}
		if val, uerr := strconv.Unquote(lit.Value); uerr == nil && val == pkg {
			idx = i
			break
		}
	}
	if idx == -1 {
		return 0, 0, fmt.Errorf("entry %q tidak ditemukan di map validServices", pkg)
	}

	start = cl.Elts[idx].Pos()
	if idx+1 < len(cl.Elts) {
		end = cl.Elts[idx+1].Pos()
	} else {
		end = cl.Rbrace
	}
	return start, end, nil
}

// findImportSpecRange mencari import spec dengan path persis importPath.
// Mengembalikan ok=false kalau tidak ketemu — pemanggil (unwireMainGo)
// cukup melewati penghapusan import, bukan menganggapnya fatal (mis.
// kalau importnya sudah dihapus manual sebelumnya).
func findImportSpecRange(file *ast.File, importPath string) (start, end token.Pos, ok bool) {
	for _, imp := range file.Imports {
		val, err := strconv.Unquote(imp.Path.Value)
		if err == nil && val == importPath {
			return imp.Pos(), imp.End(), true
		}
	}
	return 0, 0, false
}

// extendLineRemoval melebarkan [start,end) supaya ikut makan indentasi di
// depan (sampai awal baris) dan sisa baris + newline di belakang — dipakai
// khusus untuk menghapus satu baris import spec (yang, beda dari
// case/entry di atas, tidak punya "elemen berikutnya" buat dijadikan
// batas akhir alami) supaya tidak menyisakan baris kosong.
func extendLineRemoval(src []byte, start, end int) (int, int) {
	for start > 0 && (src[start-1] == ' ' || src[start-1] == '\t') {
		start--
	}
	for end < len(src) && src[end] != '\n' {
		end++
	}
	if end < len(src) && src[end] == '\n' {
		end++
	}
	return start, end
}
