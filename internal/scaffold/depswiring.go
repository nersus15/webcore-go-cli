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

	"github.com/nersus15/webcore-go-cli/internal/gomod"
)

// WireDepsService dipakai setelah NewModule membuat module baru berdiri
// sendiri (--module-path), untuk project yang mengikuti pola "composition
// root per service" seperti di contoh satusehat-fhir-webcore:
//
//	<depsDir>/
//	├── main.go              # dispatch berdasarkan argumen CLI/service
//	└── <pkg>/
//	    ├── libraries.go      # APP_LIBRARIES khusus service ini
//	    └── packages.go       # APP_PACKAGES khusus service ini
//
// Langkah 1 (generate libraries.go/packages.go) selalu aman — file baru,
// tidak menyentuh apa pun yang sudah ada. Langkah 2 (edit main.go) best-
// effort lewat AST: hanya dilakukan kalau pola "validServices map[string]bool"
// dan "switch service { case \"x\": ... default: }" ketemu PERSIS SATU
// kali di file itu — kalau ambigu atau tidak ketemu, tidak menebak, cuma
// mencetak instruksi manual.
func WireDepsService(depsDir, pkg, modulePathOverride string, force bool) error {
	depsModulePath, ok := gomod.ModulePathAt(depsDir)
	if !ok {
		return fmt.Errorf("%s tidak punya go.mod — tidak bisa generate wiring service di situ", depsDir)
	}

	data := map[string]string{
		"Pkg":                pkg,
		"ModulePathOverride": modulePathOverride,
	}

	base := filepath.Join(depsDir, pkg)
	files := map[string]string{
		"libraries.go": depsLibrariesTmpl,
		"packages.go":  depsPackagesTmpl,
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

	mainGoPath := filepath.Join(depsDir, "main.go")
	importPath := depsModulePath + "/" + pkg
	if err := wireMainGo(mainGoPath, pkg, importPath); err != nil {
		fmt.Printf(`
Tidak bisa otomatis mengubah %s (%v).

Tambahkan manual:
  1. Import %q
  2. Entry "%s": true di map validServices (atau sejenisnya)
  3. Case %q di switch service { ... }:

    case %q:
        libraries = %s.APP_LIBRARIES
        packages = %s.APP_PACKAGES
`, mainGoPath, err, importPath, pkg, pkg, pkg, pkg, pkg)
		return nil
	}

	fmt.Printf("  updated: %s\n", mainGoPath)
	fmt.Printf(`
Jangan lupa tambahkan "./modules/%s" ke blok use() di go.work, lalu:
  cd %s && go mod tidy
`, pkg, depsDir)

	return nil
}

const depsLibrariesTmpl = `package {{.Pkg}}

import (
	"github.com/webcore-go/webcore/app/core"
)

// APP_LIBRARIES khusus service "{{.Pkg}}" — tidak dibagi ke service lain.
var APP_LIBRARIES = map[string]core.LibraryLoader{
	// "database:postgres": &postgres.PostgresLoader{},
}
`

const depsPackagesTmpl = `package {{.Pkg}}

import (
	{{.Pkg}} "{{.ModulePathOverride}}"

	"github.com/webcore-go/webcore/app/core"
)

var APP_PACKAGES = []core.Module{
	{{.Pkg}}.NewModule(),
}
`

// wireMainGo menyisipkan entry baru ke map validServices dan switch
// service{} di mainGoPath, lewat AST (bukan regex) supaya posisinya pasti
// benar dan hasilnya pasti Go yang valid — gagal (dikembalikan sebagai
// error, bukan dibiarkan menebak) kalau polanya tidak ketemu PERSIS SATU
// kali di file itu.
func wireMainGo(mainGoPath, pkg, importPath string) error {
	src, err := os.ReadFile(mainGoPath)
	if err != nil {
		return fmt.Errorf("baca %s: %w", mainGoPath, err)
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, mainGoPath, src, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("parse %s: %w", mainGoPath, err)
	}

	mapPos, err := findValidServicesMapPos(file)
	if err != nil {
		return err
	}
	casePos, err := findSwitchDefaultPos(file)
	if err != nil {
		return err
	}

	type edit struct {
		pos  token.Pos
		text string
	}
	edits := []edit{
		{casePos, fmt.Sprintf("case %q:\n\tlibraries = %s.APP_LIBRARIES\n\tpackages = %s.APP_PACKAGES\n", pkg, pkg, pkg)},
		{mapPos, fmt.Sprintf("%q: true,\n", pkg)},
	}
	if importDecl := findImportBlockParen(file); importDecl != token.NoPos {
		edits = append(edits, edit{importDecl, fmt.Sprintf("%q\n", importPath)})
	}

	// urutkan dari posisi paling akhir ke paling awal, supaya insersi
	// sebelumnya tidak mengubah offset byte untuk insersi berikutnya.
	sort.Slice(edits, func(i, j int) bool { return edits[i].pos > edits[j].pos })

	result := append([]byte(nil), src...)
	for _, e := range edits {
		offset := fset.Position(e.pos).Offset
		result = append(result[:offset:offset], append([]byte(e.text), result[offset:]...)...)
	}

	formatted, err := format.Source(result)
	if err != nil {
		return fmt.Errorf("hasil edit bukan Go yang valid: %w", err)
	}

	return os.WriteFile(mainGoPath, formatted, 0o644)
}

// findValidServicesMapPos mencari literal map[string]bool di file dan
// mengembalikan posisi tepat sebelum elemen pertamanya (atau sebelum "}"
// kalau kosong). Error kalau ketemu nol atau lebih dari satu kandidat —
// demi keamanan, tidak menebak kalau ambigu.
func findValidServicesMapPos(file *ast.File) (token.Pos, error) {
	var candidates []*ast.CompositeLit

	ast.Inspect(file, func(n ast.Node) bool {
		cl, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		mt, ok := cl.Type.(*ast.MapType)
		if !ok {
			return true
		}
		key, ok1 := mt.Key.(*ast.Ident)
		val, ok2 := mt.Value.(*ast.Ident)
		if ok1 && ok2 && key.Name == "string" && val.Name == "bool" {
			candidates = append(candidates, cl)
		}
		return true
	})

	if len(candidates) != 1 {
		return 0, fmt.Errorf("ketemu %d literal map[string]bool (butuh tepat 1)", len(candidates))
	}

	cl := candidates[0]
	if len(cl.Elts) == 0 {
		return cl.Rbrace, nil
	}
	return cl.Elts[0].Pos(), nil
}

// findSwitchDefaultPos mencari switch statement yang semua case-nya
// string literal plus satu default (pola "switch service { case \"x\":
// ... default: }"), dan mengembalikan posisi tepat di case "default:"-nya
// supaya case baru bisa disisipkan sebelum situ. Error kalau ketemu nol
// atau lebih dari satu kandidat.
func findSwitchDefaultPos(file *ast.File) (token.Pos, error) {
	var candidates []*ast.SwitchStmt

	ast.Inspect(file, func(n ast.Node) bool {
		sw, ok := n.(*ast.SwitchStmt)
		if !ok || sw.Tag == nil || sw.Body == nil {
			return true
		}

		hasDefault := false
		allStringCases := true
		caseCount := 0
		for _, stmt := range sw.Body.List {
			cc, ok := stmt.(*ast.CaseClause)
			if !ok {
				continue
			}
			if cc.List == nil {
				hasDefault = true
				continue
			}
			caseCount++
			for _, expr := range cc.List {
				lit, ok := expr.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					allStringCases = false
				}
			}
		}

		if hasDefault && allStringCases && caseCount > 0 {
			candidates = append(candidates, sw)
		}
		return true
	})

	if len(candidates) != 1 {
		return 0, fmt.Errorf(`ketemu %d switch berpola 'switch x { case "a": ... default: }' (butuh tepat 1)`, len(candidates))
	}

	for _, stmt := range candidates[0].Body.List {
		cc, ok := stmt.(*ast.CaseClause)
		if ok && cc.List == nil {
			return cc.Pos(), nil
		}
	}
	return 0, fmt.Errorf("case default tidak ketemu")
}

// findImportBlockParen mencari blok import berbentuk "import (...)" (bukan
// single-import tanpa kurung) dan mengembalikan posisi ")" penutupnya.
// Mengembalikan token.NoPos kalau tidak ada — pemanggil cukup lewati
// penambahan import otomatis, bukan menganggapnya fatal.
func findImportBlockParen(file *ast.File) token.Pos {
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if ok && gd.Tok == token.IMPORT && gd.Rparen.IsValid() {
			return gd.Rparen
		}
	}
	return token.NoPos
}
