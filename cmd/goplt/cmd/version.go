package cmd

import (
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"
)

const defaultVersion = "none"

// Version and Name are injected at build time via -ldflags.
var (
	Version = defaultVersion
	Name    string
)

type info struct {
	time, arch, os, revision string
	dirty                    bool
}

func buildInfo() string {
	vinfo := vcsInfo()
	if vinfo == nil || vinfo.revision == "" {
		return fmt.Sprintf("%s %s", Name, Version)
	}

	dirtyStr := ""
	if vinfo.dirty {
		dirtyStr = "-dirty"
	}

	return fmt.Sprintf(`
Name: %s
Version: %s
Arch: %s
OS: %s
Revision: %s%s
Built at: %s
`, Name, Version, vinfo.arch, vinfo.os, vinfo.revision, dirtyStr, vinfo.time)
}

func vcsInfo() *info {
	dinfo, ok := debug.ReadBuildInfo()
	if !ok {
		return nil
	}

	out := new(info)

	for _, s := range dinfo.Settings {
		switch s.Key {
		case "GOARCH":
			out.arch = s.Value
		case "GOOS":
			out.os = s.Value
		case "vcs.revision":
			out.revision = s.Value
		case "vcs.time":
			out.time = s.Value
		case "vcs.modified":
			out.dirty = s.Value == "true"
		}
	}

	return out
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information and quit",
		Run: func(cmd *cobra.Command, _ []string) {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), buildInfo())
		},
	}
}
