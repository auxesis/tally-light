package service

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlist(t *testing.T) {
	p := string(Plist("auxesis", "/Users/auxesis"))
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
	p := string(Plist("auxesis", "/Users/a&b"))
	if !strings.Contains(p, "<string>/Users/a&amp;b</string>") {
		t.Errorf("home not XML-escaped:\n%s", p)
	}
}
