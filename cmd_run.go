package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/nersus15/webcore-go-cli/internal/names"
)

func runCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run [module]",
		Short: "Jalankan air -c air[-<module>].toml (dev, hot-reload)",
		Long: `Jalankan air untuk dev dengan hot-reload.

Argumen module OPSIONAL, tapi dicek dulu terhadap jenis project:
  - Project TANPA modul (tidak ada folder modules/ berisi apa pun):
    argumen boleh dikosongkan, jalankan "air -c air.toml".
  - Project DENGAN modul (folder modules/ berisi minimal satu modul):
    argumen WAJIB — tanpa itu, ambigu modul mana yang mau dijalankan,
    jadi dikembalikan sebagai error, bukan menebak salah satu.

File air[-<module>].toml dan .env[-<module>] digenerate otomatis oleh
"make module ... --deps-dir=..." atau "new ... --module=..." — kalau
belum ada, buat dulu lewat situ.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			hasModules := projectHasModules()

			if len(args) == 0 {
				if hasModules {
					moduleNames, _ := listModuleNames()
					return fmt.Errorf(
						"project ini punya modul (%v) — sebutkan nama modul yang mau dijalankan: "+
							"webcore-go-cli run <nama>",
						moduleNames,
					)
				}
				return runAir("air.toml", "project tanpa modul — belum ada generator otomatis utk air.toml, buat manual (lihat air-<module>.toml yang digenerate 'make module ... --deps-dir=...' sebagai contoh pola-nya)")
			}

			pkg := names.Package(args[0])
			airFile := fmt.Sprintf("air-%s.toml", pkg)
			return runAir(airFile, fmt.Sprintf("generate dulu lewat 'webcore-go-cli make module %s --module-path=<path> --deps-dir=<folder>'", args[0]))
		},
	}

	_ = cmd.RegisterFlagCompletionFunc("module", completeModuleNames)
	cmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completeModuleNames(cmd, args, toComplete)
	}

	return cmd
}

// projectHasModules menandai jenis project: true kalau folder modules/
// ada dan berisi minimal satu entry — dipakai "run" utk menentukan apakah
// argumen nama modul wajib diisi atau boleh dikosongkan.
func projectHasModules() bool {
	moduleNames, _ := listModuleNames()
	return len(moduleNames) > 0
}

func listModuleNames() ([]string, error) {
	entries, err := os.ReadDir("modules")
	if err != nil {
		return nil, err
	}

	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	return out, nil
}

func runAir(airFile, hint string) error {
	if _, err := os.Stat(airFile); err != nil {
		return fmt.Errorf("%s tidak ditemukan di direktori ini — %s", airFile, hint)
	}

	airBin, err := findAirBinary()
	if err != nil {
		return err
	}

	c := exec.Command(airBin, "-c", airFile)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

// findAirBinary cari "air" lewat PATH dulu (kasus normal: go install
// github.com/air-verse/air@latest menaruhnya di $GOBIN yang ada di PATH).
// Fallback ke "~/air" kalau tidak ketemu — banyak setup nge-alias "air" ke
// binary yang didownload manual ke home directory; alias shell semacam
// itu TIDAK pernah ter-resolve oleh exec.LookPath (exec.Command tidak
// lewat shell, jadi alias tidak berlaku), makanya perlu dicek manual.
func findAirBinary() (string, error) {
	if p, err := exec.LookPath("air"); err == nil {
		return p, nil
	}

	home, err := os.UserHomeDir()
	if err == nil {
		candidate := filepath.Join(home, "air")
		if info, statErr := os.Stat(candidate); statErr == nil && !info.IsDir() {
			return candidate, nil
		}
	}

	return "", fmt.Errorf(`binary "air" tidak ditemukan di PATH maupun ~/air — install: go install github.com/air-verse/air@latest`)
}
