// Command webcore-cli adalah CLI scaffolding untuk framework WebCore
// (github.com/webcore-go/webcore) — mirip peran `php artisan` di Laravel.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:          "webcore-cli",
		Short:        "CLI scaffolding untuk framework WebCore",
		SilenceUsage: true,
	}

	rootCmd.AddCommand(
		newCmd(),
		makeCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
