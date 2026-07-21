# Motifini — SecuritySpy Telegram Bot

Motifini is a small daemon that connects [SecuritySpy](https://www.bensoftware.com/securityspy/) to a Telegram bot — with a full interactive menu, not just slash commands.

When a camera triggers motion (or human / vehicle / animal classification), Motifini captures a short live clip and sends it to whoever subscribed to that camera. Each person configures their own subscriptions, pauses, and repeat delays. Admins tune per-camera clip quality for everyone. Snapshots, on-demand video, and system events (app started, event stream up/down, cameras online/offline) are all a tap away in Telegram.

It starts even if SecuritySpy is temporarily down, retries in the background, and keeps the Telegram bot usable meanwhile. Video capture is pure Go (no ffmpeg binary).

## Install

macOS (Homebrew tap):

```bash
brew install --cask golift/mugs/motifini
# edit $(brew --prefix)/etc/motifini.conf (copied from the example on install), then:
motifini --config="$(brew --prefix)/etc/motifini.conf"
```

The example config defaults to Apple Silicon Homebrew paths under `/opt/homebrew`
(`state_file`, `log_file`, `event_log`). If `brew --prefix` is `/usr/local` (Intel)
or anything else, change those paths to match — e.g. `$(brew --prefix)/var/...` —
before starting, or Motifini may fail to write state/logs.

Homebrew casks do not support Formula-style `brew services`; run Motifini in the
foreground (or under your own launchd unit). Running without `--config` uses the
binary default `/opt/homebrew/etc/motifini.conf`.

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

Clip quality is shared for that camera (motion alerts and `/vid`): scale (full / half / quarter), length (2–15s), and max size (500k–3MB). Half requests slightly under half native height so SecuritySpy recompresses HEVC instead of stream-copying the full frame.

**Built-in system events** (subscribe like any other event)

- Motifini Started
- Event Stream Up / Down
- Camera Online / Offline
- SecuritySpy Error

## Configuration

All options are documented in the example file:

[`https://github.com/davidnewhall/motifini/blob/main/examples/motifini.conf.example`](https://github.com/davidnewhall/motifini/blob/main/examples/motifini.conf.example)

Environment variables can override config values with prefix `MO_` (change with `--prefix`).
