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
		Use:   "run <module>",
		Short: "Jalankan air -c air-<module>.toml (dev, hot-reload)",
		Long: `Jalankan "air -c air-<module>.toml" untuk dev dengan hot-reload.

File air-<module>.toml dan .env-<module> digenerate otomatis oleh
"make module <module> --module-path=... --deps-dir=..." — kalau belum
ada, buat dulu lewat situ.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			pkg := names.Package(args[0])
			airFile := fmt.Sprintf("air-%s.toml", pkg)

			if _, err := os.Stat(airFile); err != nil {
				return fmt.Errorf(
					"%s tidak ditemukan di direktori ini — generate dulu lewat "+
						"'webcore-go-cli make module %s --module-path=<path> --deps-dir=<folder>'",
					airFile, args[0],
				)
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
		},
	}

	_ = cmd.RegisterFlagCompletionFunc("module", completeModuleNames)
	cmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return completeModuleNames(cmd, args, toComplete)
	}

	return cmd
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
