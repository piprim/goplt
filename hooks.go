package goplt

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/google/shlex"
)

// RunHooks executes the commands in m.Hooks.PostGenHooks sequentially,
// with outputDir as the working directory.
// Stops and returns an error on the first non-zero exit.
// Cancelling ctx terminates the running hook process.
func RunHooks(ctx context.Context, m *Manifest, outputDir string) error {
	for _, cmdStr := range m.Hooks.PostGenHooks {
		if err := runHook(ctx, cmdStr, outputDir); err != nil {
			return err
		}
	}

	return nil
}

func runHook(ctx context.Context, cmdStr, dir string) error {
	parts, err := shlex.Split(cmdStr)
	if err != nil {
		return fmt.Errorf("hook %q: parse command: %w", cmdStr, err)
	}

	if len(parts) == 0 {
		return nil
	}

	//nolint:gosec // The user is already warns about potential security breach.
	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Dir = dir

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("hook %q failed: %w\n%s", cmdStr, err, out)
	}

	return nil
}
