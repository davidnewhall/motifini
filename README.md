# Motifini

This application has a few features.

- Allows you to send messages via `iMessage` using `Messages.app` with
an HTTP API call.
- The webserver is optional.
- iMessages is optional. Telegram is optional too, but one is required.
- It also integrates with ffmpeg to capture videos from SecuritySpy.
- Allows subscribing to motion notifiations on SecuritySpy cameras.
- You can receive a video in Messages.app or Telegram when there is motion.
- SecuritySpy is not optional, and is the main feature of this application.

This code is still very crude, under construction and lacking full documentation.
Telegram was recently added (12/22021), and works very well with SecuritySpy.

My wife and I use this to receive videos in Telegram for cameras we care about.
I have a couple other family members relying on this app; they also use SecuritySpy.

I used to only use iMessage, but Montery broke sending files via messages with AppleScript.
If you don't use Montery this works just fine with Messages.app.

# SecuritySpy

This library taps into the SecuritySpy API and Event Stream. You do not need to do
much besides provide a URL. You can then subscribe to any camera.
It works with iMessages and/or Telegram simultaneously.

Add an account to securityspy, and give it access to your cameras.
I probably used admin, but you can certainly tune the permissions.
Sorry, I do not know what they are right now. :(

Put the url, username and password into the config file.

# iMessage

One of the optional feature in this application is to send iMessages from an HTTP API.
This is how. You must use a dedicated iMessage Apple ID for this to work _properly_.

1.  Make sure Messages has an iMessage account configured (Apple ID signed in).
1.  Send a message. Like this:
```shell
curl "http://127.0.0.1:8765/api/v1.0/send/imessage/msg/user@email1.com,user@email2.com&msg=Office%20Window%20Closed"
```

Later versions of macOS add restrictions. You may have to give motifini full disk access.
At least access to the chat database (path found in the config file). And you'll have to give
it access to "control" Messages.app with AppleScript. You should get a pop-up for that.

The ability to send files (videos and images) through Messages.app using AppleScript
[broke](https://apple.stackexchange.com/questions/429586/applescript-messages-app-monterey-problem)
with macOS 12.0 Montery. It may or may not get fixed; no idea. 12/2021.

# Telegram

Add a bot token and set a password in the config.
Once the app fires up, message your bot `/id <password>`.
Then message it `/help`.

# Example Config File

The only requirement in the config file for using the iMessages HTTP API is an `allowed_to` list.
Be sure to add your iMessage handle here or you cannot send yourself messages.
Only addresses added to the config can have messages sent to them.
This is also true if you want to use this to send things to Telegram (you do!).

-   Location: `/usr/local/etc/motifini.conf`
-   Example: [examples/motifini.conf.example](examples/motifini.conf.example)

**Setting `clear_messages` to true will delete every conversation in Messages.app.**

# HTTP Endpoints

If you enable the webserver, these are (some) of the endpoints.

-   /api/v1.0/send/imessage/video/{to}/{camera}
Uses FFMPEG to capture a video from an IP camera (or other URL).
    - **`to` (csv), list of message recipients**
    - **`cam` (string), camera name**
    - `width` (int), frame size, default: `1280`
    - `height` (int), frame size, default: `720`
    - `quality` (int), h264 quality setting, default: `20`
    - `time` (int), max video duration in seconds, default `15`
    - `rate` (int), output frame rate, default `5`
    - `size` (int), max file size, default: `2500000` (~2.5MB)

-   /api/v1.0/send/imessage/picture/{to}/{camera}
This method requires SecuritySpy be running.
    - **`to` (csv), list of message recipients**
    - **`cam` (string), camera name**

-   /api/v1.0/send/imessage/msg/{to}?msg={msg}
Just sends a plain-ol' message with iMessage.
    - **`to` (csv), list of message recipients**
    - **`msg` (string), text to send**

# IndigoDomo

This is an example showing how to trigger this app to send a picture or
message to someone via iMessage from [Indigo](http://indigodomo.com).
This works, and also works with Telegram now.
You can directly trigger "send a picture or video snippet to someone
via telegram or imessages" by hitting an http endpoint as shown here.

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
url = "http://127.0.0.1:8765/api/v1.0/send/imessage/msg/"+subs+"?msg="+msg

try:
    urllib2.urlopen(url)
    indigo.server.log(u"Dropped off message with Motifini!")
except Exception as err:
    indigo.server.log(u"Error with Motifini: {}".format(err))

```

# TODO

-   Better Usage/Install Documentation
-   Some reasonable way to add and control events, and how they fire.
    -   For instance if you want a motion detector to fire a camera, or two, or a text message, how do you define and action that?
    -   Or any combination of "event -> take some cool action (ie. send a specific type of notification)"
    -   notifications currently supported: text, video, picture.
-   Add more info to expvar data: events, cameras, subscribers, telegram stats.
