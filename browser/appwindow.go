// Package browser opens an app's local server in a borderless "app mode"
// browser window, and opens/reveals files in the OS-native way — the
// windowing and shell-out half of every applet's UI, shared verbatim
// across apps since none of it is app-specific beyond a profile-dir name.
package browser

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// OpenAppWindow opens target in a borderless, chrome-less "app mode"
// window using whichever Chromium-based browser is installed (Chrome,
// Edge, Brave, Chromium — checked in that order): no tabs, no address
// bar, its own window and Dock/taskbar entry. That's as close to a real
// native window as a stdlib-only, no-cgo Go binary can get — an actual
// embedded webview needs a cgo binding to the OS's native WebKit/
// WebView2, which would break the CGO_ENABLED=0 cross-compile every
// applet requires. Falls back to Open (a normal browser tab) if none of
// those browsers are found.
//
// appName is used only to namespace the dedicated browser profile
// directory (see appProfileDir) — pass your app's own name so two
// different applets never share one Chrome profile.
func OpenAppWindow(appName, target string) error {
	if exe := findAppCapableBrowser(); exe != "" {
		args := []string{
			"--app=" + target,
			"--user-data-dir=" + appProfileDir(appName),
			// Starts small (just the URL bar); the page itself grows the
			// window to fit its content as sections appear.
			"--window-size=820,560",
			"--no-first-run",
			"--no-default-browser-check",
		}
		cmd := exec.Command(exe, args...)
		if err := cmd.Start(); err == nil {
			go cmd.Wait() // reap without blocking; the window can outlive this call
			return nil
		}
	}
	return Open(target)
}

// OpenIfNotHosted calls OpenAppWindow unless the launcher has set
// SECUREXE_HOSTED — meaning it's spawning this process directly and will
// host the UI in its own native window instead, so a second, redundant
// Chrome window should never open. Every applet's main() should call this
// rather than OpenAppWindow directly.
func OpenIfNotHosted(appName, target string) error {
	if os.Getenv("SECUREXE_HOSTED") != "" {
		return nil
	}
	return OpenAppWindow(appName, target)
}

// appProfileDir is a dedicated, persistent browser profile just for this
// app's app-mode window — never the user's real Chrome profile — so it
// opens with no bookmarks bar, extensions, or existing tabs/history
// bleeding in.
func appProfileDir(appName string) string {
	base, err := os.UserCacheDir()
	if err != nil {
		base = os.TempDir()
	}
	dir := filepath.Join(base, appName, "app-window-profile")
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

func findAppCapableBrowser() string {
	switch runtime.GOOS {
	case "darwin":
		return firstExisting([]string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
			"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
		})
	case "windows":
		programFiles := os.Getenv("ProgramFiles")
		programFilesX86 := os.Getenv("ProgramFiles(x86)")
		localAppData := os.Getenv("LocalAppData")
		return firstExisting([]string{
			filepath.Join(programFiles, "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(programFilesX86, "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(localAppData, "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(programFiles, "Microsoft", "Edge", "Application", "msedge.exe"),
			filepath.Join(programFilesX86, "Microsoft", "Edge", "Application", "msedge.exe"),
		})
	default: // linux and other unix-likes
		for _, name := range []string{
			"google-chrome", "google-chrome-stable", "chromium",
			"chromium-browser", "brave-browser", "microsoft-edge",
		} {
			if p, err := exec.LookPath(name); err == nil {
				return p
			}
		}
		return ""
	}
}

func firstExisting(paths []string) string {
	for _, p := range paths {
		if p == "" {
			continue
		}
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p
		}
	}
	return ""
}
