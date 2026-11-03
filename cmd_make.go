package main

import (
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nersus15/webcore-go-cli/internal/scaffold"
)

// makeCmd adalah parent "make" — sengaja noun terpisah spasi (make module,
// bukan make:module) supaya shell completion bash tidak kena masalah
// COMP_WORDBREAKS (":" ada di daftar word-break default bash, bikin
// completion nama subcommand ber-colon tidak ter-insert dengan benar).
func makeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "make",
		Short: "Generate boilerplate: modul, config, entity, repository, service, handler, helper, library, migration",
	}

	cmd.AddCommand(
		makeModuleCmd(),
		makeStandaloneCmd("config", "ConfigObject satu-file (SetDefaults/SetEnvBindings)", scaffold.NewConfig),
		makeStandaloneCmd("entity", "Struct entity + Input", scaffold.NewEntity),
		makeStandaloneCmd("repository", "Repository di atas port.IDatabase", scaffold.NewRepository),
		makeStandaloneCmd("service", "Service layer", scaffold.NewService),
		makeStandaloneCmd("handler", "Fiber handler (list/get/create)", scaffold.NewHandler),
		makeHelperCmd(),
		makeLibraryCmd(),
		makeMigrationCmd(),
	)

	return cmd
}

func makeModuleCmd() *cobra.Command {
	var force bool
	var modulePath string

	cmd := &cobra.Command{
		Use:   "module <Nama>",
		Short: "Modul lengkap: module.go + config/entity/repository/service/handler",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true // error dari sini adalah error bisnis, bukan soal cara pakai
			return scaffold.NewModule(args[0], modulePath, force)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "timpa file yang sudah ada")
	cmd.Flags().StringVar(&modulePath, "module-path", "",
		"buat modul ini sebagai Go module BARU yang berdiri sendiri dengan path ini "+
			"(go mod init dijalankan otomatis) — dipakai di project multi-module/go.work, "+
			"di mana nama direktori boleh berbeda dari import path")
	return cmd
}

type standaloneFn func(name, moduleScope string, force bool) error

// makeStandaloneCmd menangani config/entity/repository/service/handler —
// semuanya punya bentuk flag yang sama: nama positional + --module opsional
// (scope ke modul yang sudah ada, default rata di root project) + --force.
// Resolusi module path (termasuk deteksi go.work, dan go.mod milik modul
// yang di-scope) didelegasikan ke package scaffold — lihat resolveScope.
func makeStandaloneCmd(noun, short string, fn standaloneFn) *cobra.Command {
	var module string
	var force bool

	cmd := &cobra.Command{
		Use:   noun + " <Nama>",
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			return fn(args[0], module, force)
		},
	}

	cmd.Flags().StringVar(&module, "module", "", "scope ke modul yang sudah ada, mis. --module=orders (default: root project, tanpa modul)")
	cmd.Flags().BoolVar(&force, "force", false, "timpa file yang sudah ada")
	_ = cmd.RegisterFlagCompletionFunc("module", completeModuleNames)

	return cmd
}

func makeHelperCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "helper <Nama>",
		Short: "Buat package helper mandiri di helper/",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			return scaffold.NewHelper(args[0], force)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "timpa file yang sudah ada")
	return cmd
}

func makeLibraryCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "library <Nama>",
		Short: "Buat skeleton library kustom (port.Library/Connector/LibraryLoader)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			return scaffold.NewLibrary(args[0], force)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "timpa file yang sudah ada")
	return cmd
}

func makeMigrationCmd() *cobra.Command {
	var module string
	var force bool

	cmd := &cobra.Command{
		Use:   "migration <nama>",
		Short: "Buat file migrasi goose (NNNNN_nama.sql, auto-increment)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			return scaffold.NewMigration(args[0], module, force)
		},
	}

	cmd.Flags().StringVar(&module, "module", "", "simpan di modules/<nama>/migrations alih-alih migrations/ root")
	cmd.Flags().BoolVar(&force, "force", false, "timpa file yang sudah ada")
	_ = cmd.RegisterFlagCompletionFunc("module", completeModuleNames)

	return cmd
}

// completeModuleNames menyarankan nama modul yang SUDAH ADA (nama folder
// di modules/ project saat ini) untuk flag --module — completion dinamis,
// mirip "git checkout <TAB>" menyarankan branch yang sungguh ada, bukan
// daftar statis.
func completeModuleNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	entries, err := os.ReadDir("modules")
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if strings.HasPrefix(e.Name(), toComplete) {
			names = append(names, e.Name())
		}
	}

	return names, cobra.ShellCompDirectiveNoFileComp
}
