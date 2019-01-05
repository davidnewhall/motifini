# Motifini

This application allows you to send messages via iMessage using Messages.app with
an API call. It also integrates with ffmpeg to capture videos from IP cameras,
and SecuritySpy to capture still images (or.. videos).

This code is still very crude, under construction and lacking full documentation.
The built-in Messages.app handler will not work in 10.3.4 or newer. Apple killed it.
You can still integrate with the subscription api using another service and send
messages/pics/vids using iMessage.

# Usage

1. Make sure Messages has an iMessage account configured.
2. Send a message. Like this:
```shell
curl "http://127.0.0.1:8765/api/v1.0/send/imessage/msg/user@email1.com,user@email2.com&msg=Office%20Window%20Closed%21%20%2811/28/18%2003%3A09%3A16%29"
```

## Example Config File

The only requirement in the config file is an `allowed_to` list. Be sure to add
your iMessage handle here or you cannot send yourself messages. Only addresses
added to the config can have messages sent to them.

Location: `/usr/local/etc/motifini.conf`
```toml
port = 8765
temp_dir = "/tmp/"
allowed_to = [ "user@domain1.com", "+12099114678", "email@address.tv"]
queue = 20
clear_messages = true

[cameras.Gate]
  url = "rtsp://admin:admin@192.168.1.13:554/live"
[cameras.Porch]
  url =  "rtsp://admin:admin@192.168.1.12:554/live"
```
Defining cameras is optional, but required to send videos.

**Setting `clear_messages` to true will delete every conversation in Messages.app.**

## Endpoints

- /api/v1.0/send/imessage/video/{to}/{camera}
  Uses FFMPEG to capture a video from an IP camera (or other URL).
  - **`to` (csv), list of message recipients**
  - **`cam` (string), camera name**
  - `level` (string), h264 quality setting, default: `3.0`, allowed: `3.0`, `3.1`, `4.0`, `4.1`, `4.2`
  - `width` (int), frame size, default: `1280`
  - `height` (int), frame size, default: `720`
  - `crf` (int), h264 quality setting, default: `20`
  - `time` (int), max video duration in seconds, default `15`
  - `audio` (bool), pass `true` to include audio.
  - `rate` (int), output frame rate, default `5`
  - `size` (int), max file size, default: `2500000` (~2.5MB)
  - `prof` (string), default: `main`, allowed: `baseline`, `high`

- /api/v1.0/send/imessage/picture/{to}/{camera}
  This method requires SecuritySpy be running.
  - **`to` (csv), list of message recipients**
  - **`cam` (string), camera name**

- /api/v1.0/send/imessage/msg/{to}?msg={msg}
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

You can use the following simple script to send yourself a picture any time motion is detected.

```applescript
-- Change Gate to a real camera name to test this in Script Editor
property TestCam : "Gate"
property Subscriber : "user@email.tld"

on run arg
	if (count of arg) is not 2 then set arg to {0, TestCam}
	set Camera to item 2 of arg -- item 1 is the cam number.
  do shell script ("curl 'http://127.0.0.1:8765/api/v1.0/send/imessage/picture/" & Subscriber & "/" & Camera & "'")
end run

```

If you're going for the full subscription integration, use this script instead,
and only recipients subscribed to the camera will be notified.
```applescript
-- Change Porch to a real camera name to test this in Script Editor
property TestCam : "Porch"

on run arg
	if (count of arg) is not 2 then set arg to {0, TestCam}
	set Camera to item 2 of arg -- item 1 is the cam number.
	do shell script ("curl -s -X POST -A SecuritySpy 'http://127.0.0.1:8765/api/v1.0/event/notify/" & Camera & "'")
end run
```
The above script is installed into `~/SecuritySpy/Scripts` when you use `make install`.

# TODO

- Better Usage/Install Documentation
- Cleanup config file to require less duplication.
- Hard define between cameras or securityspy.
  - build in solid securityspy support/library.
    - look into dedicating the module and building in things like motion alerts/hooks.
    - if dedicated, full securityspy api support would be ideal.
  - make direct-camera support "optional"
    - document the differences.
- Some reasonable way to add and control events, and how they fire.
  - For instance if you want a motion detector to fire a camera, or two, or a text message, how do you define and action that?
  - Or any combination of "event -> take some cool action (ie. send a specific type of notification)"
  - notifications currently supported: text, video, picture. (imessage only)
- Add support for notifications via other providers. pushover, skype, others? what supports video?! srsly, nothing out there yet.
- Add more info to expvar data: events, cameras, subscribers.
