package cmd

import "github.com/spf13/cobra"

func newGenerateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "generate",
		Short: "Generate files from a template directory",
	}
}
