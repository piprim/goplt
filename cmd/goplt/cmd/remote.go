// cmd/goplt/cmd/remote.go
package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const timeout = 60 * time.Second

// isRemoteRef reports whether s is a remote Go module reference rather than a
// local path. Local paths start with ".", "/", or "~". A remote reference
// begins with a hostname that contains a dot (e.g. "github.com").
//
// Note: paths starting with "~" are treated as local but are NOT expanded
// by os.DirFS — the caller is responsible for any shell-style expansion.
func isRemoteRef(s string) bool {
	if s == "" || s[0] == '.' || s[0] == '/' || s[0] == '~' {
		return false
	}
	host := strings.SplitN(s, "/", 2)[0]

	return strings.Contains(host, ".")
}

// parseRemoteRef splits a reference of the form "module/path[@version]" into
// its module path and version. Version defaults to "latest" when absent.
func parseRemoteRef(ref string) (module, version string) {
	if before, after, ok := strings.Cut(ref, "@"); ok {
		return before, after
	}

	return ref, "latest"
}

// resolveRemote fetches the Go module identified by ref using `go mod download
// -json` and returns the path to its local cache directory. The ref format is
// "module/path[@version]". Requires `go` in PATH.
func resolveRemote(ctx context.Context, ref string) (string, error) {
	module, version := parseRemoteRef(ref)
	arg := module + "@" + version

	tCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var stderr bytes.Buffer
	debugf("fetching module %s", arg)
	cmd := exec.CommandContext(tCtx, "go", "mod", "download", "-json", arg)
	cmd.Stderr = &stderr
	out, err := cmd.Output()

	// go mod download writes JSON to stdout even on non-zero exit.
	// Parse it first to surface the descriptive Error field before falling back.
	var result struct {
		Dir   string `json:"Dir"`   //nolint:tagliatelle // Go mod download wants it
		Error string `json:"Error"` //nolint:tagliatelle // Go mod download wants it
	}
	if jsonErr := json.Unmarshal(out, &result); jsonErr == nil && result.Error != "" {
		return "", fmt.Errorf("module %q: %s", arg, result.Error)
	}

	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return "", fmt.Errorf("go mod download %q: %s", arg, msg)
		}

		return "", fmt.Errorf("go mod download %q: %w", arg, err)
	}

	if result.Dir == "" {
		return "", fmt.Errorf("go mod download %q: empty directory in response", arg)
	}

	return result.Dir, nil
}
