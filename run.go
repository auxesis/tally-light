package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/auxesis/tally-light/internal/camera"
	"github.com/auxesis/tally-light/internal/config"
	"github.com/auxesis/tally-light/internal/hue"
	"github.com/auxesis/tally-light/internal/power"
	"github.com/auxesis/tally-light/internal/service"
	"github.com/auxesis/tally-light/internal/tally"
)

// pollInterval is a safety net for missed camera notifications.
const pollInterval = 2 * time.Second

func run(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	path := fs.String("config", config.DefaultPath(os.Getenv("HOME")), "config file")
	_ = fs.Parse(args) // ExitOnError: Parse exits rather than returning an error
	logger := log.New(os.Stdout, "", log.LstdFlags)

	// Validate before touching the lamp, so a bad config under launchd's
	// restart loop can't make the lamp flicker.
	cfg, err := config.Load(*path)
	if err != nil {
		logger.Print(err)
		os.Exit(1)
	}

	colour := func(name string) hue.Colour {
		c, _ := hue.ParseColour(name) // validated by config.Load
		return c
	}
	lamp := hue.Light{
		Client:     hue.NewClient(cfg.Bridge.Host, cfg.Bridge.Username),
		ID:         cfg.Light.ID,
		Brightness: uint8(cfg.Light.Brightness), //nolint:gosec // config.Load checks it is 1-254
	}
	ctrl := tally.New(lamp, tally.Colours{
		Starting: colour(cfg.Colours.Starting),
		InUse:    colour(cfg.Colours.InUse),
		NotInUse: colour(cfg.Colours.NotInUse),
		Stopped:  colour(cfg.Colours.Stopped),
	}, cfg.Camera, cfg.CameraUID, cfg.ActiveFromHour, time.Now, logger)

	withTimeout := func(f func(context.Context)) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		f(ctx)
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)

	logger.Printf("tally-light %s watching camera %q", version, cfg.Camera+cfg.CameraUID)
	withTimeout(ctrl.Start)

	changes, err := camera.Watch()
	if err != nil {
		logger.Printf("camera notifications unavailable, polling only: %v", err)
	}
	notices, err := power.Watch()
	if err != nil {
		logger.Printf("sleep/wake notifications unavailable: %v", err)
	}

	// After `brew upgrade`, exit so launchd (KeepAlive) restarts the service
	// on the new version.
	var upgrade *service.Upgrade
	if self, err := os.Executable(); err == nil {
		upgrade = service.WatchUpgrade(self)
	}

	var listFailing bool
	observe := func() {
		cams, err := camera.List()
		if err != nil {
			if !listFailing {
				logger.Print(err)
			}
			listFailing = true
			return
		}
		listFailing = false
		withTimeout(func(ctx context.Context) { ctrl.Observe(ctx, cams) })
	}
	observe()

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-changes:
			observe()
		case <-ticker.C:
			if upgrade != nil && upgrade.Happened() {
				logger.Print("binary upgraded, exiting to restart on the new version")
				return
			}
			observe()
		case n := <-notices:
			if n.Event == power.WillSleep {
				withTimeout(ctrl.Sleep)
				n.Done()
			} else {
				logger.Print("woke from sleep")
				observe()
			}
		case <-signals:
			withTimeout(ctrl.Stop)
			return
		}
	}
}
