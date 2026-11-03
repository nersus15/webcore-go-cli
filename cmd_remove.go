package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nersus15/webcore-go-cli/internal/scaffold"
)

func removeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Hapus artefak yang sebelumnya dibuat generator",
	}

	cmd.AddCommand(removeModuleCmd())

	return cmd
}

func removeModuleCmd() *cobra.Command {
	var depsDir string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "module <nama>",
		Short: "Hapus modul: modules/<nama>, wiring di main.go, .env-<nama>, air-<nama>.toml, <deps>/<nama>",
		Long: `Menghapus semua yang dibuat "make module <nama> ...":

  1. modules/<nama>/
  2. <deps-dir>/<nama>/ — kalau --deps-dir diisi (pola composition-root-
     per-service), atau fallback ke deps/<nama> (pola "new --module=<nama>")
  3. Entry modul ini di switch service dan map validServices di
     <deps-dir>/main.go — best-effort lewat AST, HANYA kalau --deps-dir
     diisi (pola "new --module=<nama>" tidak punya switch/map ini sama
     sekali). Gagal/ambigu -> dicetak instruksi manual, bukan ditebak.
  4. .env-<nama> dan air-<nama>.toml

TIDAK menyentuh go.work — kalau modul ini standalone (punya go.mod
sendiri), hapus manual entry "./modules/<nama>" dari blok use()-nya.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			result, err := scaffold.RemoveModule(args[0], depsDir, dryRun)
			if err != nil {
				return err
			}

			printRemoveResult(result, dryRun)
			return nil
		},
	}

	cmd.Flags().StringVar(&depsDir, "deps-dir", "",
		"lokasi composition root yang dipakai saat 'make module ... --deps-dir=' membuat modul ini "+
			"— ikut hapus <deps-dir>/<nama> dan bersihkan wiring-nya di <deps-dir>/main.go")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "tampilkan apa yang AKAN dihapus tanpa benar-benar menghapus apa pun")

	return cmd
}

func printRemoveResult(r *scaffold.RemoveResult, dryRun bool) {
	label := "dihapus"
	if dryRun {
		label = "akan dihapus"
	}

	removedAny := false
	print := func(path string) {
		if path == "" {
			return
		}
		removedAny = true
		fmt.Printf("  %s: %s\n", label, path)
	}

	if dryRun {
		fmt.Printf("[dry-run] modul %q:\n", r.Pkg)
	} else {
		fmt.Printf("modul %q:\n", r.Pkg)
	}
	print(r.ModulesDir)
	print(r.DepsDir)
	print(r.EnvFile)
	print(r.AirFile)

	if !removedAny {
		fmt.Println("  (tidak ada file/folder yang cocok)")
	}

	switch {
	case dryRun && r.MainGoFile != "":
		fmt.Printf("  akan dicoba bersihkan: %s (case switch + entry validServices)\n", r.MainGoFile)
	case r.MainGoEdited:
		fmt.Printf("  dibersihkan: %s\n", r.MainGoFile)
		if r.MainGoNote != "" {
			fmt.Printf("\n%s\n", r.MainGoNote)
		}
	case r.MainGoNote != "":
		fmt.Printf("\n%s\n", r.MainGoNote)
	}

	if dryRun {
		fmt.Println("\nJalankan tanpa --dry-run untuk benar-benar menghapus.")
		return
	}

	fmt.Printf(`
Jangan lupa:
  - Kalau modul %q standalone (punya go.mod sendiri), hapus entry
    "./modules/%s" dari blok use() di go.work.
  - go mod tidy di root project/workspace kalau perlu.
`, r.Pkg, r.Pkg)
}
