# `tally-light`

Turns a Philips Hue light red while your webcam is in use, and green when it isn't, so the people around you know when you're on a call.

Runs as you as a background system service.

## How it works

- **Watch the camera.** `tally-light` asks macOS whether a camera is in use. It works with any camera macOS knows about: USB webcams, the built-in camera, or Continuity Camera. Only the camera you configure affects the light.
- **Change the light.** It talks to the Hue bridge directly over its local HTTP API.
- **Run in the background.** It [runs as a LaunchDaemon](#why-a-launchdaemon) that launchd starts as your user account.

| Event | Light colour |
|---|---|
| Service starts | purple |
| Camera in use | red |
| Camera not in use, or unplugged | green |
| Mac about to sleep | green |
| Service stops | yellow |

If the bridge can't be reached, for example because you're away from home, the change is skipped and logged once. `tally-light` keeps running and catches up when the bridge comes back.

## Requirements

- macOS on Apple Silicon. Tested on macOS 26.
- [Homebrew](https://brew.sh).
- A Philips Hue bridge on your local network, plus a bridge username (see below).
- A real user account to run the service as. It has been tested with an admin account. The system account `nobody` sees no cameras, and a standard (non-admin) account hasn't been tested yet.

## Install

1. Install with [Homebrew](https://brew.sh):

   ```sh
   brew install auxesis/tap/tally-light
   ```

2. Find the name of your camera:

   ```sh
   tally-light list-cameras
   ```

   ```
   Logitech StreamCam	0x222000046d0893	not in use
   MacBook Air Camera	6C707041-05AC-0011-0004-000000000001	not in use
   ```

3. Write `~/.config/tally-light/config.toml` with that camera name and your bridge details (see [Configuration](#configuration)).

4. Start the service:

   ```sh
   sudo "$(brew --prefix)/bin/tally-light" install
   ```

   The light turns purple, then green.

To upgrade, run `brew upgrade tally-light`. The service restarts on the new version by itself.

<details>
<summary>Install without Homebrew</summary>

### Download a release

Download the latest release, check it, and unpack it:

```sh
curl -fsSLO https://github.com/auxesis/tally-light/releases/latest/download/tally-light-darwin-arm64.tar.gz
curl -fsSLO https://github.com/auxesis/tally-light/releases/latest/download/tally-light-darwin-arm64.tar.gz.sha256
shasum -a 256 -c tally-light-darwin-arm64.tar.gz.sha256
tar -xzf tally-light-darwin-arm64.tar.gz
```

The binary isn't signed by an Apple developer account. Downloading with `curl` as above works. If you download it in a browser instead, macOS will refuse to run it until you clear the quarantine flag: `xattr -d com.apple.quarantine tally-light`.

### Build from source

You need the Xcode Command Line Tools and [mise](https://mise.jdx.dev), which installs the pinned Go version:

```sh
git clone https://github.com/auxesis/tally-light.git
cd tally-light
mise install
mise run build
```

### Finish installing

Either way, you now have `./tally-light`. Follow steps 2 to 4 above, running `./tally-light` instead of `tally-light`, and `sudo ./tally-light install` to start the service. `install` copies the binary to `/usr/local/bin/tally-light`, so from then on `tally-light` is on your `PATH`.

To upgrade, get the new binary the same way and run `sudo ./tally-light install` again. It replaces the binary and restarts the service.

</details>

## Configuration

`~/.config/tally-light/config.toml`:

```toml
# Camera to watch, as shown by `tally-light list-cameras`.
camera = "Logitech StreamCam"

# Optional: match on the camera's unique ID instead of its name. Prefer the
# name: USB cameras get a different ID when plugged into a different port.
# Only use an ID to tell apart two cameras with the same name.
# camera_uid = "0x222000046d0893"

# Ignore camera changes before this hour (local time). 0 disables. Default 7.
# active_from_hour = 7

[bridge]
host = "192.168.1.2"
username = "your-bridge-username"

[light]
id = 16            # from the Hue app, or GET /api/<username>/lights
# brightness = 220 # 1-254

# Optional. Supported colours: red, yellow, green, purple.
# [colours]
# starting = "purple"
# in_use = "red"
# not_in_use = "green"
# stopped = "yellow"
```

Because the bridge username works as a password, keep the configuration file readable only by you (`chmod 600`).

```sh
chmod 600 ~/.config/tally-light/config.toml
```

The service reads the config when it starts. After editing it, restart the service:

```sh
sudo launchctl kickstart -k system/com.auxesis.tally-light
```

### Getting a bridge username

1. Press the link button on the Hue bridge.
2. Within 30 seconds, run:

   ```sh
   curl -X POST -d '{"devicetype":"tally-light"}' http://<bridge-ip>/api
   ```

3. The response contains `"username": "..."`. Put that value in the config.

## Commands

| Command | What it does |
|---|---|
| `tally-light run [-config path]` | Run in the foreground. This is what the service runs. |
| `tally-light list-cameras` | List cameras with their unique IDs and whether each is in use. |
| `sudo tally-light install` | Install the binary and the service, and start it. |
| `sudo tally-light uninstall` | Stop the service and remove the binary and its service file. |
| `tally-light version` | Print the version. |

## Logs and troubleshooting

Logs go to `~/Library/Logs/tally-light.log`:

```
2026/10/27 15:52:58 tally-light dev watching camera "Logitech StreamCam"
2026/10/27 15:52:58 starting
2026/10/27 15:52:58 camera not in use
```

Check the service is running, and as which user:

```sh
launchctl print system/com.auxesis.tally-light | grep -E 'state|username|last exit'
```

Common problems:

- **The light never changes, and the log shows `light update failed: ... no route to host`.**
  - Check `bridge.host`.
  - If you've installed it some other way, as a LaunchAgent in `~/Library/LaunchAgents`, macOS blocks it from your local network. Use `sudo tally-light install`.
- **The camera is never seen as in use.**
  - Compare the `camera` setting with `tally-light list-cameras`. Names must match exactly.
  - If you set `camera_uid` for a USB camera, it may have changed after moving the camera to another USB port. Switch to `camera` (the name).
  - The service must run as a real user account (tested with an admin account), not a system account like `nobody`.
- **The light stays purple in the morning.** Camera changes before `active_from_hour` are ignored.

## Uninstall

```sh
sudo tally-light uninstall
brew uninstall tally-light  # if installed with Homebrew
rm -r ~/.config/tally-light ~/Library/Logs/tally-light.log
```

The light changes to yellow before the service stops.

## Why a LaunchDaemon?

These were tested on macOS 26 while designing the tool.

| Runs as | Reaches Hue bridge | Sees camera state |
|---|---|---|
| LaunchAgent, any binary | No: blocked by Local Network privacy | — |
| LaunchDaemon with `UserName` = an admin user | Yes | Yes |
| LaunchDaemon with `UserName` = `nobody` | Yes | No |

Since macOS 15, Local Network privacy blocks background programs started from a LaunchAgent from reaching devices on your network. A plain command-line binary can't be listed under *System Settings* → *Privacy & Security* → *Local Network*, so there's nothing to approve. LaunchDaemons aren't subject to this.

`tally-light` never runs as root. launchd starts it as your user account.

## Development

See [DEVELOPMENT.md](DEVELOPMENT.md).
