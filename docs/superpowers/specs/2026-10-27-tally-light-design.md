# tally-light: design

Date: 2026-10-27
Status: draft. The restart and overnight-sleep checks are still pending (see "Open items").

## Intent

`bin/broadcast-meeting-status` is a shell prototype. When the desk webcam is in use, it turns a Hue lamp red, so people at home know not to interrupt, and it turns the lamp green when the camera is off.

The goal is to extract it into a standalone tool that:

- ships as a single binary, written in Rust or Go;
- runs as a background service on macOS;
- starts automatically and needs no terminal open.

This document records what the prototype taught us, so the tool can be planned without repeating the experiments.

## Goals

- When the configured camera starts being used, turn the lamp red. When it stops, turn the lamp green.
- Start automatically at boot and keep running through sleep and wake, and through the camera being unplugged and plugged back in.
- Let the user switch to a different camera later by changing one setting.
- Install as one binary with no runtime dependencies. Not `node`, not `hueadm`.

## Non-goals

- Reacting to the built-in MacBook camera, or to any camera other than the configured one. The user only wants the lamp to react when they're at their desk, and the desk camera being in use is the signal for that.
- Working away from home. If the Hue bridge can't be reached, the tool does nothing.
- Supporting more than one lamp, or other smart-light systems. Possible later, but out of scope here.
- Supporting operating systems other than macOS.

## Anti-goals

These would make the tool a failure even if it otherwise works:

- The lamp flickering or cycling because the service keeps restarting. The prototype does this when it can't work out the camera's starting state.
- A login service (LaunchAgent) that silently can't reach the bridge (see the findings below).
- Needing an app package, code signing, or a manual permission grant to work.

## Behaviour

### Decided by the user

- **"At my desk" means the configured camera is in use.** Only that camera's on/off changes affect the lamp. This replaces the prototype's public-IP "am I at home?" check, which is dropped.
- **If the bridge can't be reached, skip the change.** Log it and carry on, without crashing or retrying in a tight loop.
- **The camera is configured by name**, for example `camera = "Logitech StreamCam"`. The tool also needs a way to list the camera names available.
- **A missing camera counts as "not in use".** If the configured camera disappears (unplugged mid-call, laptop taken away) or isn't there at startup, the lamp turns green. A device-list change that removes the camera must trigger this, because no in-use change fires on a device that's gone.
- **Turn the lamp green just before the Mac sleeps.** A lamp stuck on red when no call is happening is worse than a brief wrong green. On wake, re-read the camera's current state.
- **The code lives in its own repository**, separate from these dotfiles, because it's meant to be published. It's installed with `go install` or `cargo install`, or from a release download. The tool's own `install` command then copies the binary to `/usr/local/bin` (root-owned, so the service's binary can't be swapped without the admin password) and writes and loads the LaunchDaemon file pointing at it.
- **Working after login is enough.** The lamp doesn't need to work before anyone logs in. Because it's a LaunchDaemon, the service may start before login, and before login the camera list may be empty (as it was for `nobody`). The tool must treat an empty or incomplete camera list as normal: wait for the configured camera to appear, using the device-list listener, rather than exiting.
- **Success bar for the concept:**
  1. after a restart, the lamp turns on with no terminal open;
  2. calls turn it red, and leaving them turns it green;
  3. it still works after the Mac has slept overnight.

### Carried over from the prototype (builder may keep or make configurable)

| Event | Lamp |
|---|---|
| Service starts | purple, brightness 220 |
| Camera in use | red, brightness 220 |
| Camera not in use | green, brightness 220 |
| Service stops | yellow, brightness 220 |

- **Lamp:** Hue light ID `16` ("Desk lamp") on the bridge at `192.168.88.250`. The bridge username currently lives in `~/.hueadm.json`.
- **Working hours:** the prototype ignores camera changes before 07:00. Whether to keep this is left to the builder; making it configurable is a sensible default.

## Proven constraints

Tested on 2026-10-27 on macOS 26.6 (Apple Silicon MacBook Air), with a Logitech StreamCam and the built-in camera. Each row was tested directly; none are assumed.

### Where the service runs

| Setup | Reaches Hue bridge | Sees camera on/off |
|---|---|---|
| Login service (LaunchAgent in `~/Library/LaunchAgents`), any binary | **No.** Blocked by macOS local-network privacy (`EHOSTUNREACH`) | not tested |
| System service (LaunchDaemon) as root | Yes | Yes |
| System service with `UserName` = the user (an admin) | Yes | Yes |
| System service with `UserName` = `nobody` | Yes | **No.** System log: `Must be admin`. Camera API: empty camera list, no error |

The login-service block was confirmed across six binary setups:
- plain Go;
- Go started through `/bin/sh`;
- `node` started directly (signed with a registered developer ID);
- `node` through the mise shim;
- `node` through `/bin/sh`;
- Go with an embedded `Info.plist` and bundle ID.

A bare command-line binary can't be listed under System Settings → Privacy & Security → Local Network, so the user has nothing to approve. Run from a terminal, the same binaries work, because they inherit the terminal's permission.

The network checks in the system-service rows used a compiled, ad-hoc-signed Go binary (`net/http` GET of `http://<bridge>/api/config`), not just `sh` or `node`. So a plain Go binary is proven to reach the bridge from a LaunchDaemon, both as root and with `UserName` set. The camera checks used a compiled C binary (CoreMediaIO) and `/usr/bin/log`.

**Consequence:** run as a LaunchDaemon in `/Library/LaunchDaemons`, with `UserName` set to the desk user. It never needs root at runtime, and installing needs the admin password once. launchd starts the process as the user. The tool should not start as root and switch user itself: that was never tested, and it isn't needed (decided by the user). Untested: whether a real user without admin rights would work. The camera API needs a real user, not `nobody`, but admin rights may not be required.

### How the camera is watched

| Method | Plug-in USB camera | Built-in camera | Works in system service as user |
|---|---|---|---|
| Unified log stream: `log stream --predicate 'subsystem contains "com.apple.UVCExtension" and composedMessage contains "Post PowerLog"'`, field `VDCAssistant_Power_State` (the prototype's method) | Yes | **No.** Only private, unstable `cameracaptured` debug lines | Yes (needs admin) |
| CoreMediaIO public API: `kCMIOHardwarePropertyDevices`, per device `kCMIOObjectPropertyName`, `kCMIODevicePropertyDeviceUID`, `kCMIODevicePropertyDeviceIsRunningSomewhere`, with change listeners | Yes | Yes | Yes. Listeners fire immediately; polling not needed |

**Consequence:** use CoreMediaIO.
- It reports every camera by name and ID, and reads the current state directly at startup. That removes the prototype's 24-hour log lookback and the crash it causes when the camera hasn't been used in 24 hours.
- On first open it was also faster: it saw the camera in use 9 seconds before the log message arrived. Later switches were within a second of each other.
- The listener on the device list fires when cameras are plugged in or removed. New devices need a fresh listener on their in-use property.
- A working C reference probe is in "Reference" below.

Camera names and IDs seen on this machine:

```
Logitech StreamCam             uid=0x222000046d0893
MacBook Air Camera             uid=6C707041-05AC-0011-0004-000000000001
MacBook Air Desk View Camera   uid=6C707041-05AC-0011-0004-000000000002
```

### Resilience already observed with the prototype

Running as a LaunchDaemon (as the user), the prototype survived sleep, unplugging all cables, plugging them back in, and waking. There was no restart, and it handled every camera event after waking.

## Calls made during design (check these)

- **Talk to the Hue bridge directly over its local HTTP API** (v1: `PUT /api/<username>/lights/<id>/state`) rather than shelling out to `hueadm`. This is required for a single binary.
- **Config file in the user's home directory**, for example `~/.config/<tool>/config.toml`, holding the bridge host, bridge username, light ID, camera name and colours. The service runs as the user, with `HOME` set in the service file.
- **Match the camera by name, with the ID as an optional override.** Names are readable, but two identical cameras would share one.
- **Include an uninstall command** to match `install`: unload the LaunchDaemon and remove its file and the binary. `install` sets `UserName` to the user who ran it via `sudo` (`SUDO_USER`). The manual `launchctl` steps used during testing were error-prone.
- **Name:** `tally-light` (decided by the user). Repo `~/src/github.com/auxesis/tally-light`, Go (decided by the user).

## Left to the builder

- Rust or Go. Both can call CoreMediaIO through C bindings: cgo, or Rust FFI / `objc2`-family crates.
- Config format and exact keys, logging format, and log location.
- How to handle the bridge being slow or offline (timeouts and back-off), within the anti-goal of no flicker or crash loops.
- Whether to keep the 07:00 working-hours rule.
- Which setting chooses the lamp at startup if the camera is already in use (red immediately is the obvious answer).

## Open items

- **Restart test:** does the lamp come on before anyone logs in, and does the first log line after boot show a clean state? Deferred by the user.
- **Overnight sleep test:** pending.
- **Real user without admin rights:** does the camera API work for one? Untested, and only matters if running as a dedicated account is wanted later.

## Reference

### Prototype service file used for testing

Installed at `/Library/LaunchDaemons/com.auxesis.broadcast-meeting-status.plist`:

```xml
<key>Label</key><string>com.auxesis.broadcast-meeting-status</string>
<key>UserName</key><string>auxesis</string>
<key>ProgramArguments</key><array><string>/Users/auxesis/bin/broadcast-meeting-status</string></array>
<key>EnvironmentVariables</key><dict>
  <key>HOME</key><string>/Users/auxesis</string>
  <key>PATH</key><string>/Users/auxesis/.local/share/mise/shims:/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin</string>
</dict>
<key>RunAtLoad</key><true/>
<key>KeepAlive</key><true/>
```

### CoreMediaIO probe (C), as used in the tests

The core of the probe that proved camera detection in a system service. Build it with `cc probe.c -framework CoreMediaIO -framework CoreFoundation`.

```c
// Is this camera in use by any process?
CMIOObjectPropertyAddress a = {
    kCMIODevicePropertyDeviceIsRunningSomewhere,
    kCMIOObjectPropertyScopeWildcard,
    kCMIOObjectPropertyElementWildcard};
UInt32 v = 0, used = 0;
CMIOObjectGetPropertyData(dev, &a, 0, NULL, sizeof v, &used, &v);

// Be told when it changes.
CMIOObjectAddPropertyListenerBlock(dev, &a, dispatch_get_main_queue(),
    ^(UInt32 n, const CMIOObjectPropertyAddress *addrs) { /* re-read v */ });

// Camera list (and listen on the same address for plug/unplug):
// kCMIOHardwarePropertyDevices on kCMIOObjectSystemObject.
// Name: kCMIOObjectPropertyName. ID: kCMIODevicePropertyDeviceUID (CFString).
```

The process must keep a run loop running (`CFRunLoopRun()` or `dispatch_main()`) for listeners to fire.
