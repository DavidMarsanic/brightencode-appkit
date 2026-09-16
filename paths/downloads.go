// Package paths resolves where an applet's output files are written and
// avoids overwriting existing ones.
package paths

import (
	"fmt"
	"os"
	"path/filepath"
)

// ResolveDownloadsDir returns override (creating it if needed) or, if
// override is empty, the user's normal Downloads folder — the
// cross-platform default every applet falls back to when the caller
// hasn't picked an explicit output directory.
func ResolveDownloadsDir(override string) (string, error) {
	dir := override
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolving home directory: %w", err)
		}
		dir = filepath.Join(home, "Downloads")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating output directory %s: %w", dir, err)
	}
	return dir, nil
}

// ScratchDir returns a fresh, empty directory under the OS temp dir for
// one job's uploaded input, namespaced by appName and jobID — never the
// user's real Downloads/home, and always cleaned up by the caller once
// the job ends.
func ScratchDir(appName, jobID string) (string, error) {
	dir := filepath.Join(os.TempDir(), appName, jobID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating scratch directory: %w", err)
	}
	return dir, nil
}
