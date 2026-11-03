// Command webcore adalah CLI scaffolding untuk framework WebCore
// (github.com/webcore-go/webcore) — mirip peran `php artisan` di Laravel.
//
// Jalankan "webcore-cli help" untuk daftar perintah.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/webcore-go/webcore-cli/internal/gomod"
	"github.com/webcore-go/webcore-cli/internal/scaffold"
)

// parseArgs mem-parse args ke fs, menerima flag di posisi mana pun (bukan
// hanya sebelum argumen positional seperti default flag.FlagSet) — supaya
// "webcore-cli make:module Orders --force" sama validnya dengan
// "webcore-cli make:module --force Orders".
func parseArgs(fs *flag.FlagSet, args []string) error {
	var flags, positional []string
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			flags = append(flags, a)
		} else {
			positional = append(positional, a)
		}
	}
	return fs.Parse(append(flags, positional...))
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	var err error
	switch cmd {
	case "new":
		err = cmdNew(args)
	case "make:module":
		err = cmdMakeModule(args)
	case "make:config":
		err = cmdMakeStandalone("make:config", args, scaffold.NewConfig)
	case "make:entity":
		err = cmdMakeStandalone("make:entity", args, scaffold.NewEntity)
	case "make:repository":
		err = cmdMakeStandalone("make:repository", args, scaffold.NewRepository)
	case "make:service":
		err = cmdMakeStandalone("make:service", args, scaffold.NewService)
	case "make:handler":
		err = cmdMakeStandalone("make:handler", args, scaffold.NewHandler)
	case "make:helper":
		err = cmdMakeHelper(args)
	case "make:library":
		err = cmdMakeLibrary(args)
	case "make:migration":
		err = cmdMakeMigration(args)
	case "help", "list", "-h", "--help":
		printUsage()
		return
	default:
		fmt.Fprintf(os.Stderr, "perintah tidak dikenal: %s\n\n", cmd)
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func cmdNew(args []string) error {
	fs := flag.NewFlagSet("new", flag.ExitOnError)
	modulePath := fs.String("module", "", "module path Go (default: nama project, lokal tanpa domain)")
	skipGet := fs.Bool("skip-get", false, "lewati `go get github.com/webcore-go/webcore` otomatis")
	if err := parseArgs(fs, args); err != nil {
		return err
	}

	rest := fs.Args()
	if len(rest) < 1 {
		return fmt.Errorf("usage: webcore-cli new <nama-project> [--module=<path>] [--skip-get]")
	}

	return scaffold.NewProject(scaffold.NewProjectOptions{
		Dir:        rest[0],
		ModulePath: *modulePath,
		SkipGoGet:  *skipGet,
	})
}

func cmdMakeModule(args []string) error {
	fs := flag.NewFlagSet("make:module", flag.ExitOnError)
	force := fs.Bool("force", false, "timpa file yang sudah ada")
	if err := parseArgs(fs, args); err != nil {
		return err
	}

	rest := fs.Args()
	if len(rest) < 1 {
		return fmt.Errorf("usage: webcore-cli make:module <Nama> [--force]")
	}

	modulePath, err := gomod.DetectModulePath()
	if err != nil {
		return err
	}

	return scaffold.NewModule(modulePath, rest[0], *force)
}

type standaloneFn func(modulePath, name, moduleScope string, force bool) error

// cmdMakeStandalone menangani make:config, make:entity, make:repository,
// make:service, make:handler — semuanya punya bentuk flag yang sama:
// nama positional + --module opsional (scope ke modul yang sudah ada,
// default rata di root project) + --force.
func cmdMakeStandalone(cmdName string, args []string, fn standaloneFn) error {
	fs := flag.NewFlagSet(cmdName, flag.ExitOnError)
	module := fs.String("module", "", "scope ke modul yang sudah ada, mis. --module=orders (default: root project, tanpa modul)")
	force := fs.Bool("force", false, "timpa file yang sudah ada")
	if err := parseArgs(fs, args); err != nil {
		return err
	}

	rest := fs.Args()
	if len(rest) < 1 {
		return fmt.Errorf("usage: webcore-cli %s <Nama> [--module=<nama-modul>] [--force]", cmdName)
	}

	modulePath, err := gomod.DetectModulePath()
	if err != nil {
		return err
	}

	return fn(modulePath, rest[0], *module, *force)
}

func cmdMakeHelper(args []string) error {
	fs := flag.NewFlagSet("make:helper", flag.ExitOnError)
	force := fs.Bool("force", false, "timpa file yang sudah ada")
	if err := parseArgs(fs, args); err != nil {
		return err
	}

	rest := fs.Args()
	if len(rest) < 1 {
		return fmt.Errorf("usage: webcore-cli make:helper <Nama> [--force]")
	}

	return scaffold.NewHelper(rest[0], *force)
}

func cmdMakeLibrary(args []string) error {
	fs := flag.NewFlagSet("make:library", flag.ExitOnError)
	force := fs.Bool("force", false, "timpa file yang sudah ada")
	if err := parseArgs(fs, args); err != nil {
		return err
	}

	rest := fs.Args()
	if len(rest) < 1 {
		return fmt.Errorf("usage: webcore-cli make:library <Nama> [--force]")
	}

	return scaffold.NewLibrary(rest[0], *force)
}

func cmdMakeMigration(args []string) error {
	fs := flag.NewFlagSet("make:migration", flag.ExitOnError)
	module := fs.String("module", "", "simpan di modules/<nama>/migrations alih-alih migrations/ root")
	force := fs.Bool("force", false, "timpa file yang sudah ada")
	if err := parseArgs(fs, args); err != nil {
		return err
	}

	rest := fs.Args()
	if len(rest) < 1 {
		return fmt.Errorf("usage: webcore-cli make:migration <nama> [--module=<nama-modul>] [--force]")
	}

	return scaffold.NewMigration(rest[0], *module, *force)
}

func printUsage() {
	fmt.Print(`webcore — CLI scaffolding untuk framework WebCore (github.com/webcore-go/webcore)

Usage:
  webcore-cli <command> [arguments] [flags]

Project:
  new <nama-project>              Buat project webcore baru
      --module=<path>               Module path Go (default: nama project)
      --skip-get                    Lewati "go get" otomatis

Modul (struktur lengkap: module.go + config/entity/repository/service/handler):
  make:module <Nama>              Buat modul baru di modules/<nama>/

Lapisan satu-file (bisa di root project ATAU di-scope ke modul yang sudah ada):
  make:config <Nama>               [--module=<nama-modul>] [--force]
  make:entity <Nama>                [--module=<nama-modul>] [--force]
  make:repository <Nama>            [--module=<nama-modul>] [--force]
  make:service <Nama>               [--module=<nama-modul>] [--force]
  make:handler <Nama>                [--module=<nama-modul>] [--force]

Lainnya:
  make:helper <Nama>                Buat package helper mandiri di helper/
  make:library <Nama>                Buat skeleton library kustom (port.Library/Connector)
  make:migration <nama>              [--module=<nama-modul>] [--force]

  help                              Tampilkan bantuan ini

Semua perintah "make:*" (kecuali "new") harus dijalankan dari root project
Go (tempat go.mod berada) — import path digenerate dari module path di
go.mod tersebut.
`)
}
