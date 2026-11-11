package service

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlist(t *testing.T) {
	p := string(Plist(BinPath, "auxesis", "/Users/auxesis"))
	for _, want := range []string{
		"<key>UserName</key>", "<string>auxesis</string>",
		"<string>/usr/local/bin/tally-light</string>", "<string>run</string>",
		"<key>HOME</key>", "<string>/Users/auxesis</string>",
		"<string>/Users/auxesis/Library/Logs/tally-light.log</string>",
		"<key>KeepAlive</key>", "<key>RunAtLoad</key>",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("plist missing %q", want)
		}
	}

	path := filepath.Join(t.TempDir(), "x.plist")
	if err := os.WriteFile(path, []byte(p), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("plutil", "-lint", path).CombinedOutput(); err != nil {
		t.Fatalf("plutil -lint: %v\n%s", err, out)
	}
}

func TestPlistEscapesXML(t *testing.T) {
	p := string(Plist(BinPath, "auxesis", "/Users/a&b"))
	if !strings.Contains(p, "<string>/Users/a&amp;b</string>") {
		t.Errorf("home not XML-escaped:\n%s", p)
	}
}

// homebrew lays out prefix the way Homebrew does for version v: the binary
// in the Cellar, with opt/ and bin/ symlinks pointing at it.
func homebrew(t *testing.T, prefix, v string) {
	t.Helper()
	cellarBin := filepath.Join(prefix, "Cellar", "tally-light", v, "bin")
	if err := os.MkdirAll(cellarBin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cellarBin, "tally-light"), []byte(v), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"opt", "bin"} {
		if err := os.MkdirAll(filepath.Join(prefix, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	opt := filepath.Join(prefix, "opt", "tally-light")
	os.Remove(opt)
	if err := os.Symlink(filepath.Join("..", "Cellar", "tally-light", v), opt); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(prefix, "bin", "tally-light")
	os.Remove(bin)
	if err := os.Symlink(filepath.Join("..", "opt", "tally-light", "bin", "tally-light"), bin); err != nil {
		t.Fatal(err)
	}
}

func realTempDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestTargetHomebrewUsesOptPath(t *testing.T) {
	prefix := realTempDir(t)
	homebrew(t, prefix, "1.0.0")

	bin, copyIt, err := Target(filepath.Join(prefix, "bin", "tally-light"))
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(prefix, "opt", "tally-light", "bin", "tally-light"); bin != want {
		t.Errorf("bin = %s, want %s", bin, want)
	}
	if copyIt {
		t.Error("Homebrew install should not be copied")
	}
}

func TestTargetOtherBinaryIsCopied(t *testing.T) {
	self := filepath.Join(realTempDir(t), "tally-light")
	if err := os.WriteFile(self, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	bin, copyIt, err := Target(self)
	if err != nil {
		t.Fatal(err)
	}
	if bin != BinPath || !copyIt {
		t.Errorf("got (%s, %v), want (%s, true)", bin, copyIt, BinPath)
	}
}

func TestUpgradeSeenWhenOptRetargeted(t *testing.T) {
	prefix := realTempDir(t)
	homebrew(t, prefix, "1.0.0")
	self := filepath.Join(prefix, "opt", "tally-light", "bin", "tally-light")

	u := WatchUpgrade(self)
	if u.Happened() {
		t.Fatal("upgrade reported before one happened")
	}
	homebrew(t, prefix, "1.1.0")
	if !u.Happened() {
		t.Error("upgrade to 1.1.0 not seen")
	}
}

func TestUpgradeIgnoresMissingBinary(t *testing.T) {
	prefix := realTempDir(t)
	homebrew(t, prefix, "1.0.0")
	self := filepath.Join(prefix, "opt", "tally-light", "bin", "tally-light")

	u := WatchUpgrade(self)
	// Mid-upgrade, the opt link can briefly point nowhere.
	if err := os.Remove(filepath.Join(prefix, "opt", "tally-light")); err != nil {
		t.Fatal(err)
	}
	if u.Happened() {
		t.Error("a missing binary is not an upgrade")
	}
}
