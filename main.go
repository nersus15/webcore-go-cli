// Command webcore-go-cli adalah CLI scaffolding untuk framework WebCore
// (github.com/webcore-go/webcore) — mirip peran `php artisan` di Laravel.
package main

import (
	"os"

	"github.com/spf13/cobra"
)

func main() {
	// SilenceUsage TIDAK di-set di sini (biarkan default false): kalau
	// di-set true di root, itu menjalar ke SEMUA subcommand dan mematikan
	// usage block untuk kasus di mana usage justru membantu, mis. argumen
	// kurang/invalid di "make service". Error dari logic bisnis (bukan
	// argumen) disilent per-RunE lewat cmd.SilenceUsage = true, lihat
	// cmd_new.go/cmd_make.go.
	rootCmd := &cobra.Command{
		Use:   "webcore-go-cli",
		Short: "CLI scaffolding untuk framework WebCore",
	}

	rootCmd.AddCommand(
		newCmd(),
		makeCmd(),
		renameModuleCmd(),
	)

	// Tidak perlu print err di sini — Cobra sudah mencetak error (dan
	// usage block kalau relevan) sebelum Execute() mengembalikan err.
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
