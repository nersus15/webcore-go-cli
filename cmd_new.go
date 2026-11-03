package main

import (
	"github.com/spf13/cobra"

	"github.com/nersus15/webcore-go-cli/internal/scaffold"
)

func newCmd() *cobra.Command {
	var modulePath string
	var moduleName string
	var skipGet bool

	cmd := &cobra.Command{
		Use:   "new <nama-project>",
		Short: "Buat project webcore baru",
		Long: `Buat project webcore baru. Dua struktur, tergantung --module:

  - Kosong (default): struktur "tanpa modul" — satu go.mod di root,
    main.go + config.yaml saja, tidak ada deps/ atau modules/.

  - Diisi: WORKSPACE go.work penuh dengan satu modul bisnis default yang
    berdiri sendiri — go.work di root (tanpa go.mod/main.go di situ),
    deps/ sebagai composition root (go.mod + main.go sendiri), dan
    modules/<nama> sebagai modul bisnis (go.mod sendiri juga). Siap
    langsung dipakai dengan "make module --deps-dir=deps" dan
    "remove module --deps-dir=deps" begitu butuh modul kedua.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Args sudah divalidasi Cobra di titik ini — error dari sini
			// ke bawah adalah error bisnis (folder sudah ada, dll.), bukan
			// soal cara pakai, jadi usage block tidak perlu ditampilkan.
			cmd.SilenceUsage = true

			return scaffold.NewProject(scaffold.NewProjectOptions{
				Dir:        args[0],
				ModulePath: modulePath,
				ModuleName: moduleName,
				SkipGoGet:  skipGet,
			})
		},
	}

	cmd.Flags().StringVar(&modulePath, "module-path", "", "module path Go dasar — root go.mod (tanpa modul) atau deps/go.mod + basis modules/<nama>/go.mod (dengan modul) (default: nama project, lokal tanpa domain)")
	cmd.Flags().StringVar(&moduleName, "module", "", "bangun workspace go.work dengan satu modul bisnis default berdiri sendiri dengan nama ini (deps/ + modules/<nama>, masing-masing go.mod sendiri) — kosongkan utk project \"tanpa modul\" (satu go.mod di root, lihat Quick Start webcore-docs)")
	cmd.Flags().BoolVar(&skipGet, "skip-get", false, "lewati `go get github.com/webcore-go/webcore` otomatis")

	return cmd
}
