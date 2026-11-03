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
		Args:  cobra.ExactArgs(1),
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

	cmd.Flags().StringVar(&modulePath, "module-path", "", "module path Go, baris \"module\" di go.mod (default: nama project, lokal tanpa domain)")
	cmd.Flags().StringVar(&moduleName, "module", "", "buat satu modul bisnis default dengan nama ini (modules/<nama> + deps/<nama>) — kosongkan utk project \"tanpa modul\" (lihat Quick Start webcore-docs)")
	cmd.Flags().BoolVar(&skipGet, "skip-get", false, "lewati `go get github.com/webcore-go/webcore` otomatis")

	return cmd
}
