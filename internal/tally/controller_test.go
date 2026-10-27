package tally

import (
	"bytes"
	"context"
	"errors"
	"log"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/auxesis/tally-light/internal/camera"
	"github.com/auxesis/tally-light/internal/hue"
)

var (
	purple = hue.Colour{Hue: 54613, Sat: 254}
	red    = hue.Colour{Hue: 0, Sat: 254}
	green  = hue.Colour{Hue: 21845, Sat: 254}
	yellow = hue.Colour{Hue: 10922, Sat: 254}

	cols = Colours{Starting: purple, InUse: red, NotInUse: green, Stopped: yellow}
	logi = camera.Camera{Name: "Logitech StreamCam", UID: "0x222000046d0893", InUse: true}
	ctx  = context.Background()
)

type fakeLamp struct {
	sent []hue.Colour
	errs []error
}

func (f *fakeLamp) Set(_ context.Context, c hue.Colour) error {
	f.sent = append(f.sent, c)
	if len(f.errs) > 0 {
		err := f.errs[0]
		f.errs = f.errs[1:]
		return err
	}
	return nil
}

func at(hour, min int) func() time.Time {
	return func() time.Time { return time.Date(2026, 10, 27, hour, min, 0, 0, time.Local) }
}

func newController(lamp *fakeLamp, uid string, activeFrom int, now func() time.Time, logs *bytes.Buffer) *Controller {
	if logs == nil {
		logs = &bytes.Buffer{}
	}
	return New(lamp, cols, "Logitech StreamCam", uid, activeFrom, now, log.New(logs, "", 0))
}

func assertSent(t *testing.T, lamp *fakeLamp, want ...hue.Colour) {
	t.Helper()
	if !slices.Equal(lamp.sent, want) {
		t.Errorf("sent %v, want %v", lamp.sent, want)
	}
}

func TestStartSetsStartingColour(t *testing.T) {
	lamp := &fakeLamp{}
	newController(lamp, "", 7, at(10, 0), nil).Start(ctx)
	assertSent(t, lamp, purple)
}

func TestObserveInUseSetsRed(t *testing.T) {
	lamp := &fakeLamp{}
	newController(lamp, "", 7, at(10, 0), nil).Observe(ctx, []camera.Camera{logi})
	assertSent(t, lamp, red)
}

func TestObserveSameStateSendsOnce(t *testing.T) {
	lamp := &fakeLamp{}
	c := newController(lamp, "", 7, at(10, 0), nil)
	c.Observe(ctx, []camera.Camera{logi})
	c.Observe(ctx, []camera.Camera{logi})
	assertSent(t, lamp, red)
}

func TestObserveOtherCameraIgnored(t *testing.T) {
	lamp := &fakeLamp{}
	newController(lamp, "", 7, at(10, 0), nil).Observe(ctx, []camera.Camera{
		{Name: "MacBook Air Camera", UID: "x", InUse: true},
		{Name: "Logitech StreamCam", UID: "y", InUse: false},
	})
	assertSent(t, lamp, green)
}

func TestCameraMissingCountsAsNotInUse(t *testing.T) {
	lamp := &fakeLamp{}
	c := newController(lamp, "", 7, at(10, 0), nil)
	c.Observe(ctx, []camera.Camera{logi})
	c.Observe(ctx, nil)
	assertSent(t, lamp, red, green)
}

func TestUIDOverridesName(t *testing.T) {
	lamp := &fakeLamp{}
	newController(lamp, "0x22", 7, at(10, 0), nil).Observe(ctx, []camera.Camera{
		{Name: "Logitech StreamCam", UID: "other", InUse: true},
		{Name: "Renamed", UID: "0x22", InUse: false},
	})
	assertSent(t, lamp, green)
}

func TestDuplicateNamesAnyInUse(t *testing.T) {
	lamp := &fakeLamp{}
	newController(lamp, "", 7, at(10, 0), nil).Observe(ctx, []camera.Camera{
		{Name: "Logitech StreamCam", UID: "a", InUse: false},
		{Name: "Logitech StreamCam", UID: "b", InUse: true},
	})
	assertSent(t, lamp, red)
}

func TestFailedSetIsRetriedAndLoggedOnce(t *testing.T) {
	logs := &bytes.Buffer{}
	unreachable := errors.New("connect: no route to host")
	lamp := &fakeLamp{errs: []error{unreachable, unreachable}}
	c := newController(lamp, "", 7, at(10, 0), logs)
	for range 3 {
		c.Observe(ctx, []camera.Camera{logi})
	}
	assertSent(t, lamp, red, red, red)
	if n := strings.Count(logs.String(), "lamp update failed"); n != 1 {
		t.Errorf("logged %d failures, want 1:\n%s", n, logs)
	}
	if n := strings.Count(logs.String(), "lamp reachable again"); n != 1 {
		t.Errorf("logged %d recoveries, want 1:\n%s", n, logs)
	}
}

func TestBeforeActiveHourIgnored(t *testing.T) {
	lamp := &fakeLamp{}
	newController(lamp, "", 7, at(6, 30), nil).Observe(ctx, []camera.Camera{logi})
	assertSent(t, lamp)
}

func TestActiveHourZeroNeverIgnores(t *testing.T) {
	lamp := &fakeLamp{}
	newController(lamp, "", 0, at(0, 30), nil).Observe(ctx, []camera.Camera{logi})
	assertSent(t, lamp, red)
}

func TestSleepThenWakeInCall(t *testing.T) {
	lamp := &fakeLamp{}
	c := newController(lamp, "", 7, at(10, 0), nil)
	c.Observe(ctx, []camera.Camera{logi})
	c.Sleep(ctx)
	c.Observe(ctx, []camera.Camera{logi})
	assertSent(t, lamp, red, green, red)
}

func TestStopSetsStopped(t *testing.T) {
	lamp := &fakeLamp{}
	newController(lamp, "", 7, at(10, 0), nil).Stop(ctx)
	assertSent(t, lamp, yellow)
}
