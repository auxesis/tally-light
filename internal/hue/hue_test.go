package hue

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseColour(t *testing.T) {
	want := map[string]Colour{
		"red":    {0, 254},
		"yellow": {10922, 254},
		"green":  {21845, 254},
		"purple": {54613, 254},
	}
	for name, c := range want {
		got, err := ParseColour(name)
		if err != nil || got != c {
			t.Errorf("%s: got %v %v, want %v", name, got, err, c)
		}
	}
	if _, err := ParseColour("teal"); err == nil || !strings.Contains(err.Error(), "teal") {
		t.Errorf("want error naming teal, got %v", err)
	}
}

func TestSetLightSendsState(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		if r.URL.Path != "/api/abc/lights/16/state" {
			t.Errorf("path = %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		var got, want map[string]any
		if err := json.Unmarshal(body, &got); err != nil {
			t.Errorf("body %s: %v", body, err)
		}
		if err := json.Unmarshal([]byte(`{"on":true,"hue":0,"sat":254,"bri":220}`), &want); err != nil {
			t.Fatal(err)
		}
		if gb, _ := json.Marshal(got); string(gb) != mustMarshal(want) {
			t.Errorf("body = %s", body)
		}
		_, _ = w.Write([]byte(`[{"success":{}}]`))
	}))
	defer srv.Close()

	c := NewClient(srv.Listener.Addr().String(), "abc")
	if err := c.SetLight(context.Background(), 16, Colour{0, 254}, 220); err != nil {
		t.Fatalf("SetLight: %v", err)
	}
}

func TestSetLightBridgeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"error":{"type":3,"description":"resource, /lights/99, not available"}}]`))
	}))
	defer srv.Close()

	err := NewClient(srv.Listener.Addr().String(), "abc").SetLight(context.Background(), 99, Colour{0, 254}, 220)
	if err == nil || !strings.Contains(err.Error(), "not available") {
		t.Fatalf("want bridge error, got %v", err)
	}
}

func TestSetLightUnparsableBridgeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"error":"something odd"}]`))
	}))
	defer srv.Close()

	err := NewClient(srv.Listener.Addr().String(), "abc").SetLight(context.Background(), 16, Colour{0, 254}, 220)
	if err == nil || !strings.Contains(err.Error(), "something odd") {
		t.Fatalf("want error showing the raw bridge error, got %v", err)
	}
}

func TestSetLightUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	addr := srv.Listener.Addr().String()
	srv.Close()

	if err := NewClient(addr, "abc").SetLight(context.Background(), 16, Colour{0, 254}, 220); err == nil {
		t.Fatal("want error for unreachable bridge")
	}
}

func TestSetLightErrorHidesUsername(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	addr := srv.Listener.Addr().String()
	srv.Close()

	const username = "s3cret-bridge-user"
	err := NewClient(addr, username).SetLight(context.Background(), 16, Colour{0, 254}, 220)
	if err == nil {
		t.Fatal("want error for unreachable bridge")
	}
	if strings.Contains(err.Error(), username) {
		t.Errorf("error contains bridge username: %v", err)
	}
	if !strings.Contains(err.Error(), addr) {
		t.Errorf("error should still name the bridge address: %v", err)
	}
}

func mustMarshal(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
