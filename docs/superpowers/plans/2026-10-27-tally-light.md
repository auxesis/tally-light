# tally-light Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A single Go binary that runs as a macOS LaunchDaemon (as the desk user) and turns a Hue lamp red while a named camera is in use, and green otherwise.

**Architecture:**
- Pure logic lives in `internal/tally` and is unit-tested with a fake lamp.
- Two small cgo packages talk to macOS: `internal/camera` (CoreMediaIO) and `internal/power` (IOKit sleep/wake).
- `main.go` runs a single-goroutine event loop that feeds camera, power and signal events into the controller.
- `internal/service` writes and loads the LaunchDaemon file.

**Tech Stack:**
- Go 1.26.8 (via mise), with cgo enabled.
- CoreMediaIO, IOKit and CoreFoundation frameworks.
- `github.com/BurntSushi/toml` v1.5.0.
- The Hue bridge's v1 local HTTP API.

**Spec:** `docs/superpowers/specs/2026-10-27-tally-light-design.md`

## Global Constraints

- macOS only. Tested on macOS 26.6, Apple Silicon.
- Runs as a LaunchDaemon at `/Library/LaunchDaemons/com.auxesis.tally-light.plist`, with `UserName` set to the invoking user.
  - Never a LaunchAgent: those can't reach the bridge.
  - The binary never switches user itself.
- Single binary, with no runtime dependencies (no `node`, no `hueadm`).
- Binary path: `/usr/local/bin/tally-light`.
- Config path: `$HOME/.config/tally-light/config.toml`.
- Log path: `$HOME/Library/Logs/tally-light.log`.
- Camera state comes only from CoreMediaIO `kCMIODevicePropertyDeviceIsRunningSomewhere`, never from the unified log.
- Lamp colours and brightness:
  - starting: `purple`
  - in use: `red`
  - not in use: `green`
  - stopped: `yellow`
  - brightness: `220`
  - Named colours map to `hue`/`sat` exactly as `hueadm` does: red `0`, yellow `10922`, green `21845`, purple `54613`, all with `sat` `254`.
- A configured camera that's missing counts as "not in use" (green). Set green before the Mac sleeps.
- If the bridge can't be reached, skip the change and log it. Never crash on a recoverable condition, and never retry in a tight loop.
- Never touch the lamp until the config has loaded and validated. A bad config must exit before the starting colour is sent, so launchd's restart loop can't make the lamp flicker.
- Working after login is enough. An empty camera list at startup is normal, so wait for cameras instead of exiting.

## Review Focus

Failure modes the spec implies, and the test that pins each one:

1. **Bridge unreachable at boot (network not up yet):** keep running and send the right colour once the bridge answers. Failures are logged once per outage, not every poll. Covered by `TestFailedSetIsRetriedAndLoggedOnce` (Task 3).
2. **Camera unplugged mid-call:** the lamp goes green on the next device-list change or poll. Covered by `TestCameraMissingCountsAsNotInUse` (Task 3).
3. **Switching between cameras inside one app** (the Logitech goes off and the built-in comes on in the same instant): exactly one green and no red flash. Covered by `TestObserveOtherCameraIgnored` and `TestObserveSameStateSendsOnce` (Task 3).
4. **Two cameras with the same name, or the name changes with a firmware update:** any matching camera in use counts as in use, and `camera_uid` overrides the name. Covered by `TestDuplicateNamesAnyInUse` and `TestUIDOverridesName` (Task 3).
5. **Missing or invalid config under launchd:** exit non-zero with a message naming the field, before any lamp call. Covered by `TestLoadRejectsMissingFields` (Task 2) and the ordering check in Task 6, Step 3.

---

### Task 1: Repo scaffold and Hue client

**Files:**
- Create: `go.mod`, `.mise.toml`, `.gitignore`, `main.go`
- Create: `internal/hue/hue.go`
- Test: `internal/hue/hue_test.go`

**Interfaces:**
- Produces:
  - `type hue.Colour struct { Hue uint16; Sat uint8 }`
  - `func hue.ParseColour(name string) (hue.Colour, error)`
  - `type hue.Client struct { Host, Username string; HTTP *http.Client }`
  - `func hue.NewClient(host, username string) *hue.Client`: HTTP timeout 3s.
  - `func (c *hue.Client) SetLight(ctx context.Context, id int, col hue.Colour, bri uint8) error`
  - `type hue.Light struct { Client *hue.Client; ID int; Brightness uint8 }`
  - `func (l hue.Light) Set(ctx context.Context, col hue.Colour) error`: calls `SetLight` with its ID and brightness.

- [ ] **Step 1: Scaffold.**
  - `.mise.toml` sets `go = "1.26.8"`.
  - `go.mod` is `module github.com/auxesis/tally-light`, with `go 1.26`.
  - `.gitignore` ignores `/tally-light`.
  - `main.go` is `package main` with a `version` subcommand that prints `tally-light dev`.
  - Run: `mise trust && go build ./... && go run . version`. Expected: `tally-light dev`.

- [ ] **Step 2: Write failing tests in `internal/hue/hue_test.go`.**

```go
func TestParseColour(t *testing.T) {
	want := map[string]Colour{
		"red": {0, 254}, "yellow": {10922, 254},
		"green": {21845, 254}, "purple": {54613, 254},
	}
	for name, c := range want {
		got, err := ParseColour(name)
		if err != nil || got != c { t.Errorf("%s: got %v %v", name, got, err) }
	}
	if _, err := ParseColour("teal"); err == nil || !strings.Contains(err.Error(), "teal") {
		t.Errorf("want error naming teal, got %v", err)
	}
}

func TestSetLightSendsState(t *testing.T) {
	// httptest server asserts: method PUT, path /api/abc/lights/16/state,
	// JSON body exactly {"on":true,"hue":0,"sat":254,"bri":220};
	// responds [{"success":{}}].
	// NewClient(srv.Listener.Addr().String(), "abc").SetLight(ctx, 16, Colour{0, 254}, 220) == nil
}

func TestSetLightBridgeError(t *testing.T) {
	// server responds 200 with [{"error":{"type":3,"description":"resource, /lights/99, not available"}}]
	// SetLight returns error containing "not available"
}

func TestSetLightUnreachable(t *testing.T) {
	// start a server, take its address, close it; SetLight returns a non-nil error
}
```

- [ ] **Step 3: Run the tests to confirm they fail.** Run: `go test ./internal/hue/`. Expected: FAIL (undefined: `ParseColour`).

- [ ] **Step 4: Implement `internal/hue/hue.go`.**
  - The request URL is `"http://" + Host + "/api/" + Username + "/lights/" + id + "/state"`.
  - Decode the response as `[]map[string]json.RawMessage`. Any element with an `"error"` key becomes an error that carries its `description`.
  - A non-2xx status is also an error.

- [ ] **Step 5: Run the tests.** Run: `go test ./internal/hue/`. Expected: PASS.

- [ ] **Step 6: Commit.** Run: `git add -A && git commit -m "feat: add hue client and repo scaffold"`

---

### Task 2: Config

**Files:**
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`
- Modify: `go.mod`, `go.sum` (adds `github.com/BurntSushi/toml v1.5.0`)

**Interfaces:**
- Consumes: `hue.ParseColour`.
- Produces:

```go
type Config struct {
	Camera         string `toml:"camera"`
	CameraUID      string `toml:"camera_uid"`
	ActiveFromHour int    `toml:"active_from_hour"`
	Bridge struct {
		Host     string `toml:"host"`
		Username string `toml:"username"`
	} `toml:"bridge"`
	Light struct {
		ID         int `toml:"id"`
		Brightness int `toml:"brightness"`
	} `toml:"light"`
	Colours struct {
		Starting string `toml:"starting"`
		InUse    string `toml:"in_use"`
		NotInUse string `toml:"not_in_use"`
		Stopped  string `toml:"stopped"`
	} `toml:"colours"`
}
func Load(path string) (Config, error)
func DefaultPath(home string) string // home + "/.config/tally-light/config.toml"
```

- **Defaults** apply when a key is absent:
  - `Light.Brightness`: `220`
  - colours: `purple`, `red`, `green`, `yellow`
  - `ActiveFromHour`: `7`
  - Set `active_from_hour = 0` to disable the hour rule. To tell "absent" from "0", decode into a struct whose `ActiveFromHour` is `*int`.

- [ ] **Step 1: Write failing tests.**

```go
const minimal = `camera = "Logitech StreamCam"
[bridge]
host = "192.168.88.250"
username = "abc"
[light]
id = 16
`

func TestLoadAppliesDefaults(t *testing.T) {
	// write minimal to a temp file; Load succeeds;
	// Light.Brightness == 220; Colours == {purple, red, green, yellow}; ActiveFromHour == 7
}

func TestLoadActiveFromHourZeroDisables(t *testing.T) {
	// minimal + "active_from_hour = 0\n" (placed before [bridge]) → ActiveFromHour == 0
}

func TestLoadRejectsMissingFields(t *testing.T) {
	// table, each removes one line from minimal; error must contain the named field:
	// no camera (and no camera_uid) → "camera"; no host → "bridge.host";
	// no username → "bridge.username"; no light id → "light.id"
	// and: camera_uid = "0x222000046d0893" with no camera → succeeds
}

func TestLoadRejectsUnknownColour(t *testing.T) {
	// minimal + [colours] in_use = "teal" → error contains "teal"
}

func TestLoadMissingFile(t *testing.T) {
	// Load("/nonexistent/config.toml") → error contains "/nonexistent/config.toml"
}

func TestLoadRejectsBadBrightness(t *testing.T) {
	// brightness = 300 → error contains "light.brightness" (valid range 1-254)
}
```

- [ ] **Step 2: Run the tests to confirm they fail.** Run: `go test ./internal/config/`. Expected: FAIL.
- [ ] **Step 3: Implement `Load` and `DefaultPath`.** Validate colours with `hue.ParseColour`.
- [ ] **Step 4: Run the tests.** Run: `go test ./internal/config/`. Expected: PASS.
- [ ] **Step 5: Commit.** Run: `git add -A && git commit -m "feat: add config loading and validation"`

---

### Task 3: Camera matching and the controller

The core behaviour, all pure Go. No cgo in this task.

**Files:**
- Create: `internal/camera/camera.go`: types and matching only, no cgo.
- Create: `internal/tally/controller.go`
- Test: `internal/tally/controller_test.go`

**Interfaces:**
- Consumes: `hue.Colour`.
- Produces:

```go
// internal/camera
type Camera struct { Name, UID string; InUse bool }
// InUse reports whether the configured camera is in use. When uid is set it
// matches on UID only; otherwise on Name. Any matching camera in use → true.
// No match → false (a missing camera counts as not in use).
func InUse(cams []Camera, name, uid string) bool

// internal/tally
type Lamp interface { Set(ctx context.Context, c hue.Colour) error }
type Colours struct { Starting, InUse, NotInUse, Stopped hue.Colour }
type Controller struct { /* unexported */ }
func New(l Lamp, cols Colours, name, uid string, activeFromHour int,
	now func() time.Time, logger *log.Logger) *Controller
func (c *Controller) Start(ctx context.Context)                   // sends Starting
func (c *Controller) Observe(ctx context.Context, cams []camera.Camera)
func (c *Controller) Sleep(ctx context.Context)                   // sends NotInUse
func (c *Controller) Stop(ctx context.Context)                    // sends Stopped
```

**Rules:**
- `Observe` works out the wanted colour (`InUse` or `NotInUse`). It sends only when that differs from the last *successfully sent* colour, or when the last send failed.
- `Observe` does nothing when `now().Hour() < activeFromHour`.
- `Sleep` sends `NotInUse` and records it as last sent, so the first `Observe` after waking sends red if a call is still going.
- `Start` and `Stop` always send.
- Log one line per state change, for example `camera in use: lamp red`.
- On the first failure after a success, log `lamp update failed: <err>`. Log nothing for later consecutive failures. On the next success, log `lamp reachable again`.
- The controller isn't safe for concurrent use. Only the run loop (Task 6) calls it.

- [ ] **Step 1: Write failing tests.** These use a `fakeLamp` that records each colour and pops errors from a queue, and `now` fixed at 10:00 local time. `logi := camera.Camera{"Logitech StreamCam", "0x222000046d0893", true}`.

```go
TestStartSetsStartingColour         // Start → sent [purple]
TestObserveInUseSetsRed             // Observe([logi]) → [red]
TestObserveSameStateSendsOnce       // Observe([logi]) twice → [red]
TestObserveOtherCameraIgnored       // Observe([{MacBook Air Camera, x, true}, {Logitech StreamCam, y, false}]) → [green]
TestCameraMissingCountsAsNotInUse   // Observe([logi]); Observe(nil) → [red, green]
TestUIDOverridesName                // uid "0x22": Observe([{Logitech StreamCam, other, true}, {Renamed, 0x22, false}]) → [green]
TestDuplicateNamesAnyInUse          // two "Logitech StreamCam", only the second in use → [red]
TestFailedSetIsRetriedAndLoggedOnce // errors queued for first 2 sends; Observe([logi]) ×3 → attempts [red, red, red];
                                    // log has exactly one "lamp update failed" and one "lamp reachable again"
TestBeforeActiveHourIgnored         // now 06:30, activeFromHour 7: Observe([logi]) → []
TestActiveHourZeroNeverIgnores      // now 00:30, activeFromHour 0: Observe([logi]) → [red]
TestSleepThenWakeInCall             // Observe([logi]); Sleep; Observe([logi]) → [red, green, red]
TestStopSetsStopped                 // Stop → [yellow]
```

- [ ] **Step 2: Run the tests to confirm they fail.** Run: `go test ./internal/tally/`. Expected: FAIL.
- [ ] **Step 3: Implement `camera.InUse` and the `Controller`** following the rules above.
- [ ] **Step 4: Run the tests.** Run: `go test ./internal/...`. Expected: PASS.
- [ ] **Step 5: Commit.** Run: `git add -A && git commit -m "feat: add controller deciding lamp colour from camera state"`

---

### Task 4: CoreMediaIO camera watcher and `list-cameras`

**Files:**
- Create: `internal/camera/cmio_darwin.go` (cgo Go side)
- Create: `internal/camera/cmio_darwin.c` and `internal/camera/cmio_darwin.h` (C side)
- Test: `internal/camera/cmio_darwin_test.go`
- Modify: `main.go` (adds the `list-cameras` subcommand)

**Interfaces:**
- Consumes: `camera.Camera`.
- Produces:
  - `func camera.List() ([]camera.Camera, error)`: a snapshot of every CoreMediaIO device, giving `Name` (`kCMIOObjectPropertyName`), `UID` (`kCMIODevicePropertyDeviceUID`) and `InUse` (`kCMIODevicePropertyDeviceIsRunningSomewhere`, read with wildcard scope and element).
  - `func camera.Watch() (<-chan struct{}, error)`: a channel with buffer 1, sent to without blocking on any device-list change or any device's in-use change. It carries no payload, so callers call `List()`.

**Decisions:**
- **Listener thread:** first set `kCMIOHardwarePropertyRunLoop` on `kCMIOObjectSystemObject` to a `NULL` `CFRunLoopRef`, so CoreMediaIO delivers notifications on its own thread. Register every listener block on a private serial queue: `dispatch_queue_create("tally-light.camera", NULL)`.
  - Do not use `dispatch_get_main_queue()`. The Go main thread doesn't run a dispatch or CF run loop.
- **Callbacks into Go:** listener blocks call an exported Go function `tallyCameraChanged()`, which does the non-blocking send.
- **New devices:** on a device-list change, the C side registers in-use listeners on any device IDs it hasn't seen before. It keeps an array of up to 32 device IDs, as in the spec's reference probe.
- **Allow screen-capture devices:** set `kCMIOHardwarePropertyAllowScreenCaptureDevices` to `1`, as the proven probe did.
- **`list-cameras` output:** one line per camera, `<name>\t<uid>\t<in use|not in use>`.

- [ ] **Step 1: Write failing tests.**

```go
func TestListFindsCameras(t *testing.T) {
	// skip if os.Getenv("CI") != ""
	// cams, err := List(); err == nil; len(cams) >= 1; every cam has non-empty Name and UID
}

func TestWatchSeesCameraUse(t *testing.T) {
	// skip unless os.Getenv("TALLY_MANUAL") == "1"
	// ch, err := Watch(); t.Log("open Photo Booth now"); wait up to 60s for a receive on ch,
	// then List() must report at least one camera with InUse == true
}
```

- [ ] **Step 2: Run the test to confirm it fails.** Run: `go test ./internal/camera/ -run TestListFindsCameras`. Expected: FAIL (undefined: `List`).
- [ ] **Step 3: Implement the cgo package.** Link with `#cgo LDFLAGS: -framework CoreMediaIO -framework CoreFoundation`. Port the property reads from the spec's reference probe.
- [ ] **Step 4: Run the automated test.** Run: `go test ./internal/camera/`. Expected: PASS.
- [ ] **Step 5: Add `list-cameras` and check its output.** Run: `go run . list-cameras`. Expected: three lines, including `Logitech StreamCam	0x222000046d0893` and `MacBook Air Camera	6C707041-05AC-0011-0004-000000000001`.
- [ ] **Step 6: Run the manual watch test** (the user opens Photo Booth). Run: `TALLY_MANUAL=1 go test ./internal/camera/ -run TestWatchSeesCameraUse -v`. Expected: PASS.
- [ ] **Step 7: Commit.** Run: `git add -A && git commit -m "feat: watch cameras via CoreMediaIO, add list-cameras"`

---

### Task 5: Sleep and wake notifications

**Files:**
- Create: `internal/power/power_darwin.go` and `internal/power/power_darwin.c`
- Test: `internal/power/power_darwin_test.go`

**Interfaces:**
- Produces:

```go
type Event int
const ( WillSleep Event = iota; DidWake )
type Notice struct {
	Event Event
	Done  func() // WillSleep only: must be called to let the Mac sleep (IOAllowPowerChange). No-op for DidWake.
}
func Watch() (<-chan Notice, error) // buffered 4
```

**Decisions:**
- Register with `IORegisterForSystemPower`, and attach the notification port with `IONotificationPortSetDispatchQueue` on a private serial queue (no run loop).
- `kIOMessageCanSystemSleep`: allow immediately on the C side.
- `kIOMessageSystemWillSleep`: deliver a `WillSleep` notice whose `Done` calls `IOAllowPowerChange(rootPort, notificationID)`.
- `kIOMessageSystemHasPoweredOn`: deliver `DidWake`.
- If the Go channel is full, the C callback must still allow sleep, rather than drop the notice and block sleep. In that case the exported Go callback calls `Done` itself.

- [ ] **Step 1: Write a failing test.** `TestWatchRegisters`: `ch, err := Watch()` gives `err == nil` and `ch != nil`.
- [ ] **Step 2: Run the test to confirm it fails.** Run: `go test ./internal/power/`. Expected: FAIL.
- [ ] **Step 3: Implement.** Link with `#cgo LDFLAGS: -framework IOKit -framework CoreFoundation`.
- [ ] **Step 4: Run the test.** Run: `go test ./internal/power/`. Expected: PASS. Real sleep and wake are checked at the next natural sleep, in Task 7.
- [ ] **Step 5: Commit.** Run: `git add -A && git commit -m "feat: add sleep/wake notifications"`

---

### Task 6: The `run` command

**Files:**
- Modify: `main.go`
- Create: `run.go` (package `main`)

**Interfaces:**
- Consumes: `config.Load`, `config.DefaultPath`, `hue.NewClient`, `hue.Light`, `hue.ParseColour`, `tally.New` and its methods, `camera.List`, `camera.Watch`, `power.Watch`.
- Produces:
  - `tally-light run [-config path]`: the default path is `config.DefaultPath(os.Getenv("HOME"))`.
  - Logs go to stdout with `log.LstdFlags`. launchd redirects them to the log file.

**Run loop**, in this order:
1. Load the config. On error, print it and `os.Exit(1)` before any lamp call.
2. Build the colours and the `hue.Light`.
3. Call `Start`.
4. Start `camera.Watch` and `power.Watch`. If `power.Watch` fails, log it and carry on without sleep handling. If `camera.Watch` fails, log it and rely on the poll.
5. Call `Observe(List())` once.
6. Then loop over `select`:
   - a camera change, or a 2-second ticker (the poll, kept as a safety net): `Observe(List())`, logging and skipping if `List` fails;
   - `WillSleep`: `Sleep(ctx)`, then `Done()`;
   - `DidWake`: `Observe(List())`;
   - SIGTERM or SIGINT: `Stop(ctx)`, then exit 0.

Each lamp call gets a context with a 3-second timeout.

- [ ] **Step 1: Implement `run`** as described.
- [ ] **Step 2: Write a real config.** Run:
  ```
  mkdir -p ~/.config/tally-light && jq -r '"camera = \"Logitech StreamCam\"\n[bridge]\nhost = \"\(.host)\"\nusername = \"\(.user)\"\n[light]\nid = 16"' ~/.hueadm.json > ~/.config/tally-light/config.toml
  ```
  Expected: the file has a camera, a bridge host and username, and a light ID.
- [ ] **Step 3: Check that a bad config never touches the lamp.** Run: `go run . run -config /nonexistent.toml; echo $?`. Expected: the error names the path, the exit status is `1`, and the lamp doesn't change colour.
- [ ] **Step 4: Check the end-to-end loop from a terminal.** A terminal has local-network permission. First stop the prototype with `sudo launchctl bootout system/com.auxesis.broadcast-meeting-status` so the two don't fight over the lamp.
  - Run: `go run . run`.
  - The user opens Photo Booth, switches to the MacBook Air Camera, switches back, quits, then presses Ctrl-C.
  - Expected log lines and lamp colours, in order: purple, green, `camera in use: lamp red`, green (on the switch to the built-in), red (on the switch back), green, then yellow on Ctrl-C.
- [ ] **Step 5: Run the full test suite.** Run: `go vet ./... && go test ./...`. Expected: PASS.
- [ ] **Step 6: Commit.** Run: `git add -A && git commit -m "feat: add run command wiring camera, power and lamp"`

---

### Task 7: `install` and `uninstall`

**Files:**
- Create: `internal/service/launchd.go`
- Test: `internal/service/launchd_test.go`
- Modify: `main.go`

**Interfaces:**
- Consumes: `config.Load`, `config.DefaultPath`.
- Produces:

```go
const (
	Label     = "com.auxesis.tally-light"
	PlistPath = "/Library/LaunchDaemons/com.auxesis.tally-light.plist"
	BinPath   = "/usr/local/bin/tally-light"
)
func Plist(user, home string) []byte
func Install(self, user, home string) error
func Uninstall() error
```

**What `Plist` contains:**
- `Label`
- `UserName` = user
- `ProgramArguments` = [`BinPath`, `run`]
- `EnvironmentVariables` with `HOME` = home
- `RunAtLoad` true and `KeepAlive` true
- `StandardOutPath` and `StandardErrorPath` = `home + "/Library/Logs/tally-light.log"`

**What `Install` does, in order:**
1. Copy `self` (from `os.Executable()`) to `BinPath`, with mode `0755`, owned by `root:wheel`.
2. Write `PlistPath` with mode `0644`, owned by `root:wheel`.
3. Run `launchctl bootout system/<Label>`, ignoring any error.
4. Run `launchctl bootstrap system <PlistPath>`.

**What `Uninstall` does:** `bootout`, then remove `PlistPath` and `BinPath`. A missing file isn't an error.

**What `main.go` checks before installing:**
- It's running as root (`os.Geteuid() == 0`). Otherwise exit 1 with `run with sudo`.
- `SUDO_USER` is set and isn't `root`.
- It resolves home with `user.Lookup(SUDO_USER).HomeDir`.
- `config.Load(config.DefaultPath(home))` succeeds. A broken config must never be installed, because it would restart-loop.

- [ ] **Step 1: Write a failing test.**

```go
func TestPlist(t *testing.T) {
	p := string(Plist("auxesis", "/Users/auxesis"))
	for _, want := range []string{
		"<key>UserName</key>", "<string>auxesis</string>",
		"<string>/usr/local/bin/tally-light</string>", "<string>run</string>",
		"<key>HOME</key>", "<string>/Users/auxesis</string>",
		"<string>/Users/auxesis/Library/Logs/tally-light.log</string>",
		"<key>KeepAlive</key>", "<key>RunAtLoad</key>",
	} {
		if !strings.Contains(p, want) { t.Errorf("plist missing %q", want) }
	}
	// write p to t.TempDir()/x.plist and exec `plutil -lint` on it: must exit 0
}
```

- [ ] **Step 2: Run the test to confirm it fails.** Run: `go test ./internal/service/`. Expected: FAIL.
- [ ] **Step 3: Implement `Plist`, `Install`, `Uninstall` and the two subcommands.**
- [ ] **Step 4: Run the tests.** Run: `go vet ./... && go test ./...`. Expected: PASS.
- [ ] **Step 5: Commit.** Run: `git add -A && git commit -m "feat: add install and uninstall for LaunchDaemon"`
- [ ] **Step 6: Install and retire the prototype.** The user runs this in a terminal, because it needs a password:
  ```
  go build -o tally-light . && sudo ./tally-light install && sudo rm -f /Library/LaunchDaemons/com.auxesis.broadcast-meeting-status.plist /usr/local/libexec/cmio-probe /Library/LaunchDaemons/com.auxesis.cmio-probe-nobody.plist; sudo launchctl bootout system/com.auxesis.cmio-probe-nobody 2>/dev/null
  ```
  Expected:
  - `launchctl print system/com.auxesis.tally-light` shows `state = running` and `username = auxesis`;
  - the lamp turns purple, then green;
  - `~/Library/Logs/tally-light.log` shows the start.
- [ ] **Step 7: Check the success bar.**
  1. Open Photo Booth: the lamp goes red. Quit it: green. Check this in the log.
  2. At the next natural sleep and wake, the log shows a green on sleep and a fresh observation on wake.
  3. After the next restart (whenever convenient), the lamp comes on without a terminal.

  Record the results in the spec's "Open items".
