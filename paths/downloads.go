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
