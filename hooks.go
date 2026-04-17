package goplt

import (
	"fmt"
	"os/exec"
	"strings"
)

// RunHooks executes the commands in m.Hooks.PostGenHooks sequentially,
// with outputDir as the working directory.
// Stops and returns an error on the first non-zero exit.
func RunHooks(m *Manifest, outputDir string) error {
	for _, cmdStr := range m.Hooks.PostGenHooks {
		if err := runHook(cmdStr, outputDir); err != nil {
			return err
		}
	}

	return nil
}

func runHook(cmdStr, dir string) error {
	parts := strings.Fields(cmdStr)

	if len(parts) == 0 {
		return nil
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Dir = dir

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("hook %q failed: %w\n%s", cmdStr, err, out)
	}

	return nil
}
