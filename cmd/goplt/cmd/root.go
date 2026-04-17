package cmd

import (
	"github.com/spf13/cobra"
)

func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "goplt",
		Short: "Cookiecutter-style project scaffolding for Go",
	}

	cmd.AddCommand(newGenerateCmd())

	return cmd
}
