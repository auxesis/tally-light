// Package hue sets the state of a light through a Philips Hue bridge's
// local v1 HTTP API.
package hue

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// Colour is a Hue hue/saturation pair.
type Colour struct {
	Hue uint16
	Sat uint8
}

// Named colours, matching what hueadm sends for the same names.
var colours = map[string]Colour{
	"red":    {0, 254},
	"yellow": {10922, 254},
	"green":  {21845, 254},
	"purple": {54613, 254},
}

// ParseColour returns the Colour for a colour name.
func ParseColour(name string) (Colour, error) {
	c, ok := colours[name]
	if !ok {
		return Colour{}, fmt.Errorf("unknown colour %q (want red, yellow, green or purple)", name)
	}
	return c, nil
}

// Client talks to one bridge as one registered user.
type Client struct {
	Host     string
	Username string
	HTTP     *http.Client
}

// NewClient returns a Client with a 3 second request timeout.
func NewClient(host, username string) *Client {
	return &Client{Host: host, Username: username, HTTP: &http.Client{Timeout: 3 * time.Second}}
}

// SetLight turns light id on with the given colour and brightness.
func (c *Client) SetLight(ctx context.Context, id int, col Colour, bri uint8) error {
	body, err := json.Marshal(map[string]any{"on": true, "hue": col.Hue, "sat": col.Sat, "bri": bri})
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/lights/%d/state", id)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, "http://"+c.Host+"/api/"+c.Username+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		// The username is a credential and errors end up in the log, so
		// replace the URL net/http puts in the error with one without it.
		var ue *url.Error
		if errors.As(err, &ue) {
			ue.URL = "http://" + c.Host + "/api/<username>" + path
		}
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("hue bridge: HTTP %d", resp.StatusCode)
	}

	var results []map[string]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return fmt.Errorf("hue bridge: decoding response: %w", err)
	}
	for _, r := range results {
		if raw, ok := r["error"]; ok {
			var e struct {
				Description string `json:"description"`
			}
			json.Unmarshal(raw, &e)
			return fmt.Errorf("hue bridge: %s", e.Description)
		}
	}
	return nil
}

// Light is one light on a bridge, at a fixed brightness.
type Light struct {
	Client     *Client
	ID         int
	Brightness uint8
}

// Set changes the light's colour.
func (l Light) Set(ctx context.Context, col Colour) error {
	return l.Client.SetLight(ctx, l.ID, col, l.Brightness)
}
