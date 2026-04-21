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

// ExitCodeError carries a subprocess exit code that main should forward to
// os.Exit rather than printing as an error message.
type ExitCodeError struct{ Code int }

func (e *ExitCodeError) Error() string { return fmt.Sprintf("exit status %d", e.Code) }

// debugf prints to stderr only when --debug is set.
func debugf(format string, args ...any) {
	if debugMode {
		fmt.Fprintf(os.Stderr, "debug: "+format+"\n", args...)
	}
}
