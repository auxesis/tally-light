package camera

import (
	"os"
	"testing"
	"time"
)

func TestListFindsCameras(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("no cameras in CI")
	}
	cams, err := List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(cams) == 0 {
		t.Fatal("no cameras found")
	}
	for _, c := range cams {
		if c.Name == "" || c.UID == "" {
			t.Errorf("camera missing name or UID: %+v", c)
		}
	}
}

func TestWatchSeesCameraUse(t *testing.T) {
	if os.Getenv("TALLY_MANUAL") != "1" {
		t.Skip("set TALLY_MANUAL=1 and open Photo Booth to run")
	}
	ch, err := Watch()
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}
	t.Log("open Photo Booth now")
	select {
	case <-ch:
	case <-time.After(60 * time.Second):
		t.Fatal("no camera change within 60s")
	}
	// The change may be the camera list being re-read before the device
	// starts; give it a moment to report in use.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		cams, err := List()
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		for _, c := range cams {
			if c.InUse {
				t.Logf("in use: %s", c.Name)
				return
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("change seen but no camera reported in use")
}
