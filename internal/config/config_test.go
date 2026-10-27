package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const minimal = `camera = "Logitech StreamCam"
[bridge]
host = "192.168.88.250"
username = "abc"
[light]
id = 16
`

func write(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadAppliesDefaults(t *testing.T) {
	c, err := Load(write(t, minimal))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Light.Brightness != 220 {
		t.Errorf("Brightness = %d, want 220", c.Light.Brightness)
	}
	cols := c.Colours
	if cols.Starting != "purple" || cols.InUse != "red" || cols.NotInUse != "green" || cols.Stopped != "yellow" {
		t.Errorf("Colours = %+v", cols)
	}
	if c.ActiveFromHour != 7 {
		t.Errorf("ActiveFromHour = %d, want 7", c.ActiveFromHour)
	}
}

func TestLoadActiveFromHourZeroDisables(t *testing.T) {
	c, err := Load(write(t, "active_from_hour = 0\n"+minimal))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.ActiveFromHour != 0 {
		t.Errorf("ActiveFromHour = %d, want 0", c.ActiveFromHour)
	}
}

func TestLoadRejectsMissingFields(t *testing.T) {
	cases := []struct {
		drop, field string
	}{
		{`camera = "Logitech StreamCam"`, "camera"},
		{`host = "192.168.88.250"`, "bridge.host"},
		{`username = "abc"`, "bridge.username"},
		{`id = 16`, "light.id"},
	}
	for _, tc := range cases {
		_, err := Load(write(t, strings.Replace(minimal, tc.drop+"\n", "", 1)))
		if err == nil || !strings.Contains(err.Error(), tc.field) {
			t.Errorf("without %s: want error naming %q, got %v", tc.drop, tc.field, err)
		}
	}

	uidOnly := strings.Replace(minimal, `camera = "Logitech StreamCam"`, `camera_uid = "0x222000046d0893"`, 1)
	if _, err := Load(write(t, uidOnly)); err != nil {
		t.Errorf("camera_uid without camera: %v", err)
	}
}

func TestLoadRejectsUnknownColour(t *testing.T) {
	_, err := Load(write(t, minimal+"[colours]\nin_use = \"teal\"\n"))
	if err == nil || !strings.Contains(err.Error(), "teal") {
		t.Fatalf("want error naming teal, got %v", err)
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load("/nonexistent/config.toml")
	if err == nil || !strings.Contains(err.Error(), "/nonexistent/config.toml") {
		t.Fatalf("want error naming path, got %v", err)
	}
}

func TestLoadRejectsBadBrightness(t *testing.T) {
	_, err := Load(write(t, minimal+"brightness = 300\n"))
	if err == nil || !strings.Contains(err.Error(), "light.brightness") {
		t.Fatalf("want error naming light.brightness, got %v", err)
	}
}
