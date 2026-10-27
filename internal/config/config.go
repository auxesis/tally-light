// Package config loads and validates tally-light's TOML config file.
package config

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/BurntSushi/toml"

	"github.com/auxesis/tally-light/internal/hue"
)

type Bridge struct {
	Host     string `toml:"host"`
	Username string `toml:"username"`
}

type Light struct {
	ID         int `toml:"id"`
	Brightness int `toml:"brightness"`
}

type Colours struct {
	Starting string `toml:"starting"`
	InUse    string `toml:"in_use"`
	NotInUse string `toml:"not_in_use"`
	Stopped  string `toml:"stopped"`
}

type Config struct {
	Camera         string  `toml:"camera"`
	CameraUID      string  `toml:"camera_uid"`
	ActiveFromHour int     `toml:"active_from_hour"`
	Bridge         Bridge  `toml:"bridge"`
	Light          Light   `toml:"light"`
	Colours        Colours `toml:"colours"`
}

// file mirrors Config, but uses a pointer so an absent active_from_hour can
// be told apart from an explicit 0.
type file struct {
	Camera         string  `toml:"camera"`
	CameraUID      string  `toml:"camera_uid"`
	ActiveFromHour *int    `toml:"active_from_hour"`
	Bridge         Bridge  `toml:"bridge"`
	Light          Light   `toml:"light"`
	Colours        Colours `toml:"colours"`
}

// DefaultPath is where the config lives for a user with the given home.
func DefaultPath(home string) string {
	return filepath.Join(home, ".config", "tally-light", "config.toml")
}

// Load reads the config at path, applies defaults and validates it.
func Load(path string) (Config, error) {
	var f file
	if _, err := toml.DecodeFile(path, &f); err != nil {
		return Config{}, fmt.Errorf("config %s: %w", path, err)
	}

	c := Config{
		Camera:         f.Camera,
		CameraUID:      f.CameraUID,
		ActiveFromHour: 7,
		Bridge:         f.Bridge,
		Light:          f.Light,
		Colours:        f.Colours,
	}
	if f.ActiveFromHour != nil {
		c.ActiveFromHour = *f.ActiveFromHour
	}
	if c.Light.Brightness == 0 {
		c.Light.Brightness = 220
	}
	defaultTo(&c.Colours.Starting, "purple")
	defaultTo(&c.Colours.InUse, "red")
	defaultTo(&c.Colours.NotInUse, "green")
	defaultTo(&c.Colours.Stopped, "yellow")

	if err := c.validate(); err != nil {
		return Config{}, fmt.Errorf("config %s: %w", path, err)
	}
	return c, nil
}

func defaultTo(s *string, v string) {
	if *s == "" {
		*s = v
	}
}

func (c Config) validate() error {
	var errs []error
	if c.Camera == "" && c.CameraUID == "" {
		errs = append(errs, errors.New("camera (or camera_uid) is required"))
	}
	if c.Bridge.Host == "" {
		errs = append(errs, errors.New("bridge.host is required"))
	}
	if c.Bridge.Username == "" {
		errs = append(errs, errors.New("bridge.username is required"))
	}
	if c.Light.ID <= 0 {
		errs = append(errs, errors.New("light.id is required"))
	}
	if c.Light.Brightness < 1 || c.Light.Brightness > 254 {
		errs = append(errs, fmt.Errorf("light.brightness must be 1-254, got %d", c.Light.Brightness))
	}
	for _, name := range []string{c.Colours.Starting, c.Colours.InUse, c.Colours.NotInUse, c.Colours.Stopped} {
		if _, err := hue.ParseColour(name); err != nil {
			errs = append(errs, fmt.Errorf("colours: %w", err))
		}
	}
	return errors.Join(errs...)
}
