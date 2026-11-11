// Package service installs tally-light as a LaunchDaemon that launchd runs
// as the desk user. A LaunchAgent can't be used: macOS blocks those from
// reaching devices on the local network.
package service

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const (
	Label     = "com.auxesis.tally-light"
	PlistPath = "/Library/LaunchDaemons/com.auxesis.tally-light.plist"
	BinPath   = "/usr/local/bin/tally-light"
)

const plistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>%[1]s</string>
  <key>UserName</key>
  <string>%[2]s</string>
  <key>ProgramArguments</key>
  <array>
    <string>%[3]s</string>
    <string>run</string>
  </array>
  <key>EnvironmentVariables</key>
  <dict>
    <key>HOME</key>
    <string>%[4]s</string>
  </dict>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>StandardOutPath</key>
  <string>%[5]s</string>
  <key>StandardErrorPath</key>
  <string>%[5]s</string>
</dict>
</plist>
`

// Plist returns the LaunchDaemon definition for user, whose home is home.
func Plist(user, home string) []byte {
	logPath := filepath.Join(home, "Library", "Logs", "tally-light.log")
	return fmt.Appendf(nil, plistTemplate,
		esc(Label), esc(user), esc(BinPath), esc(home), esc(logPath))
}

func esc(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s)) // writes to a bytes.Buffer never fail
	return b.String()
}

// Install copies the binary at self to BinPath, writes the LaunchDaemon and
// (re)loads it. Must run as root.
func Install(self, user, home string) error {
	if err := copyBinary(self, BinPath); err != nil {
		return fmt.Errorf("installing binary: %w", err)
	}
	if err := writeRootFile(PlistPath, Plist(user, home), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", PlistPath, err)
	}
	// Fails when the service isn't loaded yet, which is fine.
	_ = exec.Command("launchctl", "bootout", "system/"+Label).Run()

	// bootout finishes asynchronously; bootstrap can fail briefly after it.
	var out []byte
	var err error
	for range 5 {
		out, err = exec.Command("launchctl", "bootstrap", "system", PlistPath).CombinedOutput()
		if err == nil {
			return nil
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("launchctl bootstrap: %v: %s", err, out)
}

// Uninstall stops the service and removes its files.
func Uninstall() error {
	// Fails when the service isn't loaded, which is fine.
	_ = exec.Command("launchctl", "bootout", "system/"+Label).Run()
	for _, p := range []string{PlistPath, BinPath} {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func copyBinary(src, dst string) error {
	in, err := os.Open(src) //nolint:gosec // src is this program's own executable
	if err != nil {
		return err
	}
	defer in.Close()
	data, err := io.ReadAll(in)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil { //nolint:gosec // /usr/local/bin must be readable by every user
		return err
	}
	return writeRootFile(dst, data, 0o755)
}

// writeRootFile writes data to a temp file beside path and renames it into
// place, so a running binary is replaced rather than overwritten.
func writeRootFile(path string, data []byte, mode os.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, mode); err != nil {
		return err
	}
	if err := os.Chmod(tmp, mode); err != nil {
		return err
	}
	if err := os.Chown(tmp, 0, 0); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
