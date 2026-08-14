# Motifini — SecuritySpy Telegram Bot

Motifini is a small daemon that connects [SecuritySpy](https://www.bensoftware.com/securityspy/) to a Telegram bot — with a full interactive menu, not just slash commands.

When a camera triggers motion (or human / vehicle / animal classification), Motifini captures a short live clip and sends it to whoever subscribed to that camera. Each person configures their own subscriptions, pauses, and repeat delays. Admins tune per-camera clip quality for everyone. Snapshots, on-demand video, and system events (app started, event stream up/down, cameras online/offline) are all a tap away in Telegram.

It starts even if SecuritySpy is temporarily down, retries in the background, and keeps the Telegram bot usable meanwhile. Video capture is pure Go (no ffmpeg binary).

## Install

macOS (Homebrew tap):

```bash
brew install --cask golift/mugs/motifini
# edit $(brew --prefix)/etc/motifini.conf, then:
launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/io.golift.motifini.plist
```

The cask installs a LaunchAgent plist. First install leaves it unloaded until you edit the config and `--start`. Later `brew upgrade --cask` restarts it automatically when `motifini.conf` already exists. Control it with:

```bash
motifini --start    # load at login / start now
motifini --restart  # restart the running agent
motifini --stop     # unload / stop
```

These wrap:

- `--start` → `launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/io.golift.motifini.plist`
- `--restart` → `launchctl kickstart -k gui/$(id -u)/io.golift.motifini`
- `--stop` → `launchctl bootout gui/$(id -u)/io.golift.motifini`

`brew services` only manages formulae, not casks.

The example config defaults to Apple Silicon Homebrew paths under `/opt/homebrew`
(`state_file`, `log_file`, `event_log`). If `brew --prefix` is `/usr/local` (Intel)
or anything else, change those paths to match — e.g. `$(brew --prefix)/var/...` —
before starting, or Motifini may fail to write state/logs.

Running without `--config` uses the binary default `/opt/homebrew/etc/motifini.conf`.
Logs from the LaunchAgent go to `$(brew --prefix)/var/log/motifini.log`.

Or download binaries for macOS (universal), Linux, FreeBSD, and Windows from
[GitHub Releases](https://github.com/davidnewhall/motifini/releases).

## Quick start

1. Create a SecuritySpy web account with access to your cameras.
2. Create a Telegram bot with [@BotFather](https://t.me/BotFather).
3. Copy the example config and fill in URL, credentials, bot token, and password:

   [`https://github.com/davidnewhall/motifini/blob/main/examples/motifini.conf.example`](https://github.com/davidnewhall/motifini/blob/main/examples/motifini.conf.example)

   Default config path (no flags): `/opt/homebrew/etc/motifini.conf`. Override with
   `--config=/path/to/file`. After `brew install --cask`, prefer
   `$(brew --prefix)/etc/motifini.conf`.

4. Run Motifini, then message the bot:

   ```text
   /id <telegram.password>
   /help
   ```

## Using the bot

The Telegram UI is a full-blown button menu. Browse cameras, subscribe to events, pause alerts, set delays, pull a snapshot or clip — almost everything is tappable.
Slash commands still work if you prefer typing (`/sub`, `/subs`, `/stop`, `/delay`, `/cams`, `/pics`, `/vid`, …); `/help` lists them.

**Allowing users**

New chats get no reply until they are allowed:

1. Self-serve: `/id <password>` (from config), or
2. Admin: after they message once, `/allow <telegramIdOrUsername>` (also `/auth`). Revoke with `/deny <id>`.

`/admin <user>` grants admin commands only; it does **not** unlock the bot — use `/allow` for that.

Display name when someone has no `@username`: `/name <chatId> Jane Doe` (aliases: `/rename`, `/nick`).

**Per-subscriber configuration**

Every allowed chat has its own settings. One person can watch the driveway for cars, another only humans at the front door, and a third can pause the porch for an hour — without affecting anyone else.

- Subscribe / unsubscribe per camera and classification (motion, human, vehicle, animal), or to named system events
- Per-subscription repeat delay (how long before another clip for the same trigger)
- Pause all alerts or a single camera (`/stop` / menu), then resume when ready
- On-demand snapshot or video from any camera you can see

**Per-camera clip settings** (admins — `/camset` or Cams → camera → Clip settings)

Clip quality is shared for that camera (motion alerts and `/vid`): scale (full / half / third / quarter), length (2–15s), max size (500k–3MB), and output codec (h265 default / h264 / auto). Half requests slightly under half native height so SecuritySpy recompresses HEVC instead of stream-copying the full frame.

**Built-in system events** (subscribe like any other event)

- Motifini Started
- Event Stream Up / Down
- Camera Online / Offline
- SecuritySpy Error

## Home Assistant

Home Assistant can fire Motifini event notifications — text, a camera photo, or a short video clip — over HTTP. Telegram users subscribe to those events in the bot's Events menu (under **— Home Assistant —**); Home Assistant never talks to Telegram, and no Telegram credentials live in HA.

**1. Enable the webserver** in `motifini.conf`:

```toml
[webserver]
  enable = true
  port = 8765
  # HA on another host? Bind a LAN address and set an API key:
  # listen_addr = "0.0.0.0"
  # api_key = "output-of: openssl rand -hex 24"
```

When `api_key` is set, every API request needs it (`Authorization: Bearer <key>` header, `X-API-Key` header, or `?apikey=` query param). Keep the default localhost bind and it stays optional; expose the port to your LAN and the key is **required** — Motifini refuses to start with a non-localhost `listen_addr` and no `api_key`.

**2. Install the integration** from this repo with HACS (*Custom repositories* → *Integration*), or copy [`custom_components/motifini`](custom_components/motifini/) into your HA `config/custom_components/`. Then add **Motifini** under *Settings → Integrations* (host + port + the same API key; it checks connectivity against the events API).

**3. Call the services** from automations:

```yaml
# message + photo
- action: motifini.notify
  data:
    event: big_garage_opened
    message: "Big garage door opened"
    camera: "Garage"
    media: photo

# photo only (camera may be a SecuritySpy name or number)
- action: motifini.notify
  data:
    event: driveway_motion
    camera: "3"
    media: photo

# message only
- action: motifini.notify
  data:
    event: power_restored
    message: "House power is back"
```

`media` is `none` | `photo` | `video` and defaults to `photo` when a camera is given. `camera` is required for photo/video. A call with neither `message` nor camera media is rejected.

**Event lifecycle.** The first `motifini.notify` (or an explicit `motifini.register_event` with a `description`) creates the catalog entry, so it shows up in the Telegram Events menu — subscribe there once and every later notify lands in your chat. Deleting an automation in HA does **not** remove the event from Motifini: call `motifini.remove_event` once when you retire an event (this also unsubscribes everyone in Telegram), or just unsubscribe in the bot and leave the orphan entry.

**Note:** Motifini long-polls the Telegram bot. Do not configure HA's built-in `telegram` / `telegram_bot` integration with the *same* bot token — two pollers on one token steal each other's updates.

**Raw HTTP API** (for anything that isn't HA):

| Method | Path | Purpose |
|--------|------|---------|
| `PUT` | `/api/v1.0/event/{event}` | Register/update an event (`description` form field or JSON body) |
| `GET` | `/api/v1.0/events` | List the event catalog as JSON |
| `POST` | `/api/v1.0/event/notify/{event}` | Notify subscribers; form fields `msg`, `camera`, `media`, `description` |
| `POST` | `/api/v1.0/event/remove/{event}` | Remove an event and all its subscriptions |

## Configuration

All options are documented in the example file:

[`https://github.com/davidnewhall/motifini/blob/main/examples/motifini.conf.example`](https://github.com/davidnewhall/motifini/blob/main/examples/motifini.conf.example)

Environment variables can override config values with prefix `MO_` (change with `--prefix`).
