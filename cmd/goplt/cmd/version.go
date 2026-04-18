// cmd/goplt/cmd/version.go
package cmd

import (
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"
)

// Build-time variables injected via -ldflags.
// When installed via `go install` these keep their defaults and resolvedVersion
// falls back to the module version embedded by the Go toolchain.
var (
	Version   = ""
	Commit    = "none"
	BuildDate = "unknown"
)

// resolvedVersion returns the version string to display.
// Ldflags take priority; otherwise the Go module version from build info is used.
func resolvedVersion() string {
	if Version != "" {
		return Version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprintf(cmd.OutOrStdout(), "goplt %s (commit: %s, built: %s)\n", resolvedVersion(), Commit, BuildDate)
		},
	}
}
