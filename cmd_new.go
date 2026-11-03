package main

import (
	"github.com/spf13/cobra"

	"github.com/nersus15/webcore-cli/internal/scaffold"
)

func newCmd() *cobra.Command {
	var modulePath string
	var skipGet bool

	cmd := &cobra.Command{
		Use:   "new <nama-project>",
		Short: "Buat project webcore baru",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return scaffold.NewProject(scaffold.NewProjectOptions{
				Dir:        args[0],
				ModulePath: modulePath,
				SkipGoGet:  skipGet,
			})
		},
	}

	cmd.Flags().StringVar(&modulePath, "module", "", "module path Go (default: nama project, lokal tanpa domain)")
	cmd.Flags().BoolVar(&skipGet, "skip-get", false, "lewati `go get github.com/webcore-go/webcore` otomatis")

	return cmd
}
