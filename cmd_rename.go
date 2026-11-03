package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nersus15/webcore-go-cli/internal/scaffold"
)

func renameModuleCmd() *cobra.Command {
	var module string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "rename-module <path-baru>",
		Short: "Ganti module path (go.mod) sebuah module dan perbaiki semua import ke path lama",
		Long: `Ganti module path (baris "module" di go.mod) dan perbaiki SEMUA import ke
path lama itu — baik self-import di dalam module itu sendiri maupun
import dari module lain dalam workspace go.work yang sama yang
memakainya sebagai dependency.

Tidak menjangkau repo/module di luar workspace go.work saat ini — itu di
luar kendali tool ini.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			result, err := scaffold.RenameModule(args[0], module, dryRun)
			if err != nil {
				return err
			}

			if dryRun {
				fmt.Printf("[dry-run] %s -> %s\n\n", result.OldPath, result.NewPath)
				fmt.Printf("go.mod yang akan diubah:\n  %s\n\n", result.GoModFile)
				if len(result.ChangedGo) == 0 {
					fmt.Println("Tidak ada file .go lain yang meng-import path ini.")
				} else {
					fmt.Printf("File .go yang import-nya akan diubah (%d):\n", len(result.ChangedGo))
					for _, f := range result.ChangedGo {
						fmt.Printf("  %s\n", f)
					}
				}
				fmt.Println("\nJalankan tanpa --dry-run untuk benar-benar menerapkan.")
				return nil
			}

			fmt.Printf("%s -> %s\n\n", result.OldPath, result.NewPath)
			fmt.Printf("go.mod diubah:\n  %s\n", result.GoModFile)
			if len(result.ChangedGo) > 0 {
				fmt.Printf("\nImport diperbaiki di %d file:\n", len(result.ChangedGo))
				for _, f := range result.ChangedGo {
					fmt.Printf("  %s\n", f)
				}
			}
			for _, e := range result.TidyErrors {
				fmt.Printf("\nperingatan: go mod tidy gagal di %s\n", e)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&module, "module", "", "ganti go.mod milik modules/<nama> (default: root project)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "tampilkan apa yang AKAN diubah tanpa benar-benar menulis apa pun")
	_ = cmd.RegisterFlagCompletionFunc("module", completeModuleNames)

	return cmd
}
