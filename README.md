# Motifini

This application has a few features.

- Send Telegram messages via an optional HTTP API.
- Captures short SecuritySpy video clips in pure Go (no ffmpeg binary).
- Subscribe to motion / human / vehicle / animal notifications on SecuritySpy cameras.
- Receive alert videos in Telegram when something happens.
- SecuritySpy is required; Telegram is the notification channel.

My wife and I use this to receive videos in Telegram for cameras we care about.
I have a couple other family members relying on this app; they also use SecuritySpy.

## SecuritySpy

This library taps into the SecuritySpy API and Event Stream. You do not need to do
much besides provide a URL. You can then subscribe to any camera.
It works with Telegram.

Add an account to securityspy, and give it access to your cameras.
I probably used admin, but you can certainly tune the permissions.
Sorry, I do not know what they are right now. :(

Put the url, username and password into the config file.

## Telegram

Add a bot token and set a password in the config.
Once the app fires up, message your bot `/id <password>`.
Then message it `/help`.

New users who message without auth are logged and get **no reply** until they are allowed:

1. **Self-serve:** they send `/id <telegram.password>` (from config), or
2. **Admin:** after they message once, you send `/allow <telegramIdOrUsername>` (also `/auth`).
   Revoke with `/deny <id>`.

`/admin <user>` only grants admin commands; it does **not** unlock the bot. Use `/allow` for that.

Set a display name when someone has no Telegram `@username`:

```text
/name <chatId> Jane Doe
```

Aliases: `/rename`, `/nick`.

## Example Config File

Only Telegram chat IDs listed in `allowed_to` can receive messages from the HTTP API.

- Location: `/usr/local/etc/motifini.conf`
- Example: [examples/motifini.conf.example](examples/motifini.conf.example)

Optional `[motifini]` settings:

- `debug` — verbose Telegram/HTTP diagnostics (also written to `log_file` when set)
- `log_file` — path for a rotating app log (when set, further logs leave stdout/stderr; config and log paths are still printed to stdout first)
- `event_log` — path for a rotating SecuritySpy event-stream log (omit to disable; must differ from `log_file`)
- `log_file_mb` — max size per rotated log file in MB (default `5`)
- `log_files` — number of rotated log files to keep (default `10`)
- `security_spy_retry` — how often to retry connecting when SecuritySpy is down at startup (Go duration, default `5s`)

Built-in system events (subscribe via Events / `/sub`):

- **Motifini Started** — text notification when Motifini finishes booting (Telegram ready)
- Event Stream Up / Down, Camera Online / Offline, SecuritySpy Error

Admin Telegram commands:

- `/camset` (aka `/clipset`) — per-camera clip profile used for motion alerts and `/vid` (everyone gets the same clip)
  - **Scale:** full / half / quarter of native resolution (default half)
  - **Length:** 2–15 seconds (default 6s)
  - **Size:** 500k–3MB max file size (default 1.5MB)
  - Also available from **Cameras → camera → Clip settings** for admins

## HTTP Endpoints

If you enable the webserver, these are (some) of the endpoints.

- /api/v1.0/send/telegram/video/{to}/{camera}
Captures a short live clip from SecuritySpy (no ffmpeg binary).
  - **`to` (csv), list of message recipients**
  - **`camera` (string), camera name**
  - `width` (int), frame size
  - `height` (int), frame size
  - `crf` / quality (int)
  - `time` (duration or seconds), max video length
  - `rate` (int), output frame rate
  - `size` (int), max file size

- /api/v1.0/send/telegram/picture/{to}/{camera}
Snapshot from SecuritySpy.
  - **`to` (csv), list of message recipients**
  - **`camera` (string), camera name**

- /api/v1.0/send/telegram/msg/{to}?msg={msg}
Plain Telegram text.
  - **`to` (csv), list of message recipients**
  - **`msg` (string), text to send**

- /api/v1.0/sub/{subscribe|unsubscribe|pause|unpause}/{api}/{contact}/{event}
Manage a subscriber's event subscription (Telegram `api` + chat id or contact name).
  - `minutes` (pause only, default `60`)

- /api/v1.0/event/{notify|remove}/{event}
Notify subscribers of a named event (optional camera snapshot when the name matches a camera),
or remove an event and all its subscriptions.

- /debug/vars
expvar stats (HTTP counts, subscribers, cameras, …).

## IndigoDomo

This is an example showing how to trigger this app to send a picture or
message to someone via Telegram from [Indigo](http://indigodomo.com).
This works, and also works with Telegram now.
You can directly trigger "send a picture or video snippet to someone
via telegram" by hitting an http endpoint as shown here.

Create two variables in Indigo.
Name one variable `Subscribers` and the other `SendMessage`
Create a trigger when `SendMessage` changes to run an Action.

Run this Action; replace the variable IDs with your own:

```python
import urllib
import urllib2
import socket
timeout = 1
socket.setdefaulttimeout(timeout)

subs = urllib.quote(indigo.variables[1891888064].value, "")
msg = urllib.quote(indigo.variables[1023892794].value, "")
url = "http://127.0.0.1:8765/api/v1.0/send/telegram/msg/"+subs+"?msg="+msg

try:
    urllib2.urlopen(url)
    indigo.server.log(u"Dropped off message with Motifini!")
except Exception as err:
    indigo.server.log(u"Error with Motifini: {}".format(err))
```

Subscriptions and motion classifications are managed in Telegram (`/sub`, `/subs`, `/stop`, `/delay`).
Home automation can still fire notifications via the HTTP endpoints above.
