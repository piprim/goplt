package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var debugMode bool

func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "goplt",
		Short: "Cookiecutter-style project scaffolding for Go",
		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true
		},
	}

	cmd.PersistentFlags().BoolVar(&debugMode, "debug", false, "Print debug output")

	cmd.AddCommand(newGenerateCmd())
	cmd.AddCommand(newVersionCmd())
	cmd.AddCommand(newTestCmd())

	return cmd
}

// debugf prints to stderr only when --debug is set.
func debugf(format string, args ...any) {
	if debugMode {
		fmt.Fprintf(os.Stderr, "debug: "+format+"\n", args...)
	}
}
