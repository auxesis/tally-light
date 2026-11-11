// Package tally decides what colour the light should show.
package tally

import (
	"context"
	"log"
	"time"

	"github.com/auxesis/tally-light/internal/camera"
	"github.com/auxesis/tally-light/internal/hue"
)

// Light is anything whose colour can be set.
type Light interface {
	Set(ctx context.Context, c hue.Colour) error
}

// Colours is the colour shown in each state.
type Colours struct {
	Starting, InUse, NotInUse, Stopped hue.Colour
}

// Controller turns camera observations and lifecycle events into light
// changes. It is not safe for concurrent use.
type Controller struct {
	light      Light
	colours    Colours
	name, uid  string
	activeFrom int
	now        func() time.Time
	log        *log.Logger

	sent    *hue.Colour // last colour the light accepted
	failed  bool        // the last send failed, so resend even if unchanged
	failing bool        // a failure has been logged and not yet recovered
}

func New(l Light, cols Colours, name, uid string, activeFromHour int,
	now func() time.Time, logger *log.Logger) *Controller {
	return &Controller{light: l, colours: cols, name: name, uid: uid,
		activeFrom: activeFromHour, now: now, log: logger}
}

func (c *Controller) Start(ctx context.Context) {
	c.send(ctx, c.colours.Starting, "starting")
}

// Observe updates the light from a snapshot of all cameras. It only sends
// when the wanted colour changed or the previous send failed, and ignores
// observations before the active hour.
func (c *Controller) Observe(ctx context.Context, cams []camera.Camera) {
	if c.now().Hour() < c.activeFrom {
		return
	}
	want, msg := c.colours.NotInUse, "camera not in use"
	if camera.InUse(cams, c.name, c.uid) {
		want, msg = c.colours.InUse, "camera in use"
	}
	if c.sent != nil && *c.sent == want && !c.failed {
		return
	}
	c.send(ctx, want, msg)
}

// Sleep shows not-in-use before the Mac sleeps, unless the light already
// shows it: dark wakes cause repeated sleeps overnight.
func (c *Controller) Sleep(ctx context.Context) {
	if c.sent != nil && *c.sent == c.colours.NotInUse && !c.failed {
		return
	}
	c.send(ctx, c.colours.NotInUse, "going to sleep")
}

func (c *Controller) Stop(ctx context.Context) {
	c.send(ctx, c.colours.Stopped, "stopping")
}

func (c *Controller) send(ctx context.Context, col hue.Colour, msg string) {
	if err := c.light.Set(ctx, col); err != nil {
		if !c.failing {
			c.log.Printf("%s: light update failed: %v", msg, err)
		}
		c.failing, c.failed = true, true
		return
	}
	if c.failing {
		c.log.Print("light reachable again")
	}
	c.failing, c.failed = false, false
	c.sent = &col
	c.log.Print(msg)
}
