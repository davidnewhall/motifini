// Package motifini wires configuration, SecuritySpy, chat, messengers, and the HTTP server.
package motifini

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/davidnewhall/motifini/pkg/chat"
	"github.com/davidnewhall/motifini/pkg/messenger"
	"golift.io/securityspy/v2"
)

const (
	eventStreamBuf = 1000
	eventRetry     = 5 * time.Second
	defaultLength  = 6 * time.Second
	defaultSize    = 1.5 * 1024 * 1024
	defaultCodec   = "aac"
	defaultHeight  = 720
)

// ProcessEventStream processes the securityspy event stream.
func (m *Motifini) ProcessEventStream() {
	events := make(chan securityspy.Event, eventStreamBuf)

	m.SSpy.Events.BindChan(securityspy.EventAllEvents, events)
	m.SSpy.Events.Watch(eventRetry, false)

	go m.handleEvents(events)
}

func (m *Motifini) handleEvents(events chan securityspy.Event) {
	m.Info.Println("Event Stream Watcher Started")
	defer m.Error.Println("Event Stream Watcher Closed")

	for event := range events {
		m.dispatchEvent(&event)
	}
}

func (m *Motifini) dispatchEvent(event *securityspy.Event) {
	m.logStreamEvent(event)

	switch event.Type { //nolint:exhaustive // use default wisely
	case securityspy.EventKeepAlive, securityspy.EventTriggerMotion:
		// ignore.
	case securityspy.EventMotionDetected:
		// v4 motion event.
		if strings.HasPrefix(m.SSpy.Info.Version, "4") {
			m.handleCameraMotion(event)
		}
	case securityspy.EventTriggerAction:
		// v5 action event. (motion detected, actions enabled)
		m.handleCameraMotion(event)
	case securityspy.EventStreamConnect:
		m.Info.Println("SecuritySpy Event Stream Connected!")
		m.notifySystemEvent(chat.EventStreamUp, "SecuritySpy event stream is back up.")
	case securityspy.EventStreamDisconnect:
		m.handleStreamDisconnect(event)
	case securityspy.EventOffline:
		m.notifySystemEvent(chat.EventCameraOffline, "Camera went offline: "+eventCameraName(event))
	case securityspy.EventOnline:
		m.notifySystemEvent(chat.EventCameraOnline, "Camera came online: "+eventCameraName(event))
	case securityspy.EventSecSpyError:
		msg := "SecuritySpy error"
		if event.Msg != "" {
			msg += ": " + event.Msg
		}

		m.notifySystemEvent(chat.EventSecSpyError, msg)
	case securityspy.EventConfigChange:
		m.handleConfigChange()
	}
}

// logStreamEvent writes SecuritySpy events to the optional event log file.
// Keep-alives are omitted (too noisy). When event_log is unset, Event discards.
func (m *Motifini) logStreamEvent(event *securityspy.Event) {
	if event.Type == securityspy.EventKeepAlive {
		return
	}

	camName := ""
	if event.Camera != nil {
		camName = "camera: " + event.Camera.Name
	}

	m.Event.Println(event.String(), camName, event.Msg)
}

func (m *Motifini) handleStreamDisconnect(event *securityspy.Event) {
	m.Error.Println("SecuritySpy Event Stream Disconnected")

	msg := "SecuritySpy event stream went down."
	if event.Msg != "" {
		msg += "\n" + event.Msg
	}

	m.notifySystemEvent(chat.EventStreamDown, msg)
}

func eventCameraName(event *securityspy.Event) string {
	if event.Camera != nil {
		return event.Camera.Name
	}

	return "unknown camera"
}

func (m *Motifini) handleConfigChange() {
	m.saveSubDB() // just because.
	m.Info.Println("SecuritySpy Configuration Changed! Stopping webserver and messenger to refresh SecuritySpy data.")

	err := m.HTTP.Stop()
	if err != nil {
		m.Error.Println("Stopping Webserver:", err)
	}
	defer m.HTTP.Start()

	if m.Msgs != nil {
		m.Msgs.Stop()

		defer func() {
			err := m.Msgs.Start()
			if err != nil {
				m.Error.Println("Starting Message Watcher Routines:", err)
			}
		}()
	}

	err = m.SSpy.Refresh()
	if err != nil {
		m.Error.Println("Refreshing SecuritySpy Configuration:", err)
	}

	time.Sleep(time.Second)
}

func (m *Motifini) handleCameraMotion(event *securityspy.Event) {
	if event.Camera == nil {
		return // this wont happen. check anyway.
	}

	keys := chat.NotifyKeys(event.Camera.Name, event.Reasons)
	subs := chat.CollectSubscribers(m.Subs, keys)
	reqID := messenger.ReqID(messenger.IDLength)
	path := filepath.Join(
		m.Conf.Global.TempDir, fmt.Sprintf("motifini_camera_motion_%s_%s.mp4", reqID, event.Camera.Name))

	subCount := len(subs)
	if subCount < 1 {
		return // no one to notify of this camera's motion
	}

	err := event.Camera.SaveVideo(
		&securityspy.VidOps{
			ACodec: defaultCodec,
			Height: defaultHeight,
			VCodec: event.Camera.PreferredVCodec(),
		}, defaultLength, defaultSize, path)
	if err != nil {
		m.Error.Printf("[%v] event.Camera.SaveVideo: %v", reqID, err)
		return
	}

	m.Msgs.SendFileOrMsg(reqID, chat.EventCaption(event.Camera.Name, event.Reasons), path, subs)

	for _, sub := range subs {
		for _, key := range chat.ActiveKeysAmong(sub, keys) {
			delay, ok := sub.Events.RuleGetD(key, "delay")
			if !ok {
				delay = DefaultRepeatDelay
			}

			_ = sub.Events.Pause(key, delay)
		}
	}

	names := make([]string, 0, subCount)
	for _, sub := range subs {
		name := sub.Contact
		if name == "" {
			name = "?"
		}
		names = append(names, fmt.Sprintf("%d:%s", sub.ID, name))
	}

	m.Info.Printf("[%v] Event '%v' triggered subscription messages. Subscribers: %v (%s) keys: %v",
		reqID, event.Camera.Name, subCount, strings.Join(names, ", "), keys)
}

// notifySystemEvent texts subscribers of a built-in non-camera event (no video attachment).
func (m *Motifini) notifySystemEvent(eventName, msg string) {
	subs := m.Subs.GetSubscribers(eventName)
	if len(subs) < 1 {
		return
	}

	reqID := messenger.ReqID(messenger.IDLength)
	m.Msgs.SendFileOrMsg(reqID, msg, "", subs)

	for _, sub := range subs {
		delay, ok := sub.Events.RuleGetD(eventName, "delay")
		if !ok {
			delay = DefaultRepeatDelay
		}

		_ = sub.Events.Pause(eventName, delay)
	}

	names := make([]string, 0, len(subs))
	for _, sub := range subs {
		name := sub.Contact
		if name == "" {
			name = "?"
		}
		names = append(names, fmt.Sprintf("%d:%s", sub.ID, name))
	}

	m.Info.Printf("[%v] System event '%v' notified %d subscriber(s): %s",
		reqID, eventName, len(subs), strings.Join(names, ", "))
}
