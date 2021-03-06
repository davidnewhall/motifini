# Motifini

This application allows you to send messages via iMessage using Messages.app with
an API call. It also integrates with ffmpeg to capture videos from SecuritySpy.

This code is still very crude, under construction and lacking full documentation.

# Usage

1.  Make sure Messages has an iMessage account configured.
1.  Send a message. Like this:
```shell
curl "http://127.0.0.1:8765/api/v1.0/send/imessage/msg/user@email1.com,user@email2.com&msg=Office%20Window%20Closed"
```

## Example Config File

The only requirement in the config file is an `allowed_to` list. Be sure to add
your iMessage handle here or you cannot send yourself messages. Only addresses
added to the config can have messages sent to them.

-   Location: `/usr/local/etc/motifini.conf`
-   Example: [examples/motifini.conf.example](examples/motifini.conf.example)

**Setting `clear_messages` to true will delete every conversation in Messages.app.**

## Endpoints

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

## Indigo

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

## SecuritySpy

This library taps into the SecuritySpy API and Event Stream. You do not need to do
much besides provide a URL. You can then subscribe to any camera.

# TODO

-   Better Usage/Install Documentation
-   Some reasonable way to add and control events, and how they fire.
    -   For instance if you want a motion detector to fire a camera, or two, or a text message, how do you define and action that?
    -   Or any combination of "event -> take some cool action (ie. send a specific type of notification)"
    -   notifications currently supported: text, video, picture. (imessage only)
-   Add support for notifications via other providers. pushover, skype, others? what supports video?! srsly, nothing out there yet.
-   Add more info to expvar data: events, cameras, subscribers.
