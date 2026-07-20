// Package motifini wires configuration, SecuritySpy, chat, messengers, and the HTTP server.
package motifini

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/davidnewhall/motifini/pkg/messenger"
	"golift.io/securityspy/v2"
)

const (
	eventStreamBuf = 1000
	eventRetry     = 5 * time.Second
	defaultLength  = 5 * time.Second
	defaultSize    = 1.5 * 1024 * 1024
	defaultCodec   = "ulaw"
	defaultHeight  = 500
)

// ProcessEventStream processes the securityspy event stream.
func (m *Motifini) ProcessEventStream() {
	events := make(chan securityspy.Event, eventStreamBuf)

	m.SSpy.Events.BindChan(securityspy.EventAllEvents, events)
	m.SSpy.Events.Watch(eventRetry, false)

	go m.handleEvents(events)
}

func (m *Motifini) handleEvents(events chan securityspy.Event) { //nolint:cyclop // it's not that bad.
	m.Info.Println("Event Stream Watcher Started")
	defer m.Error.Println("Event Stream Watcher Closed")

	for event := range events {
		switch event.Type { //nolint:exhaustive // use default wisely
		case securityspy.EventKeepAlive:
			// ignore.
		case securityspy.EventMotionDetected:
			// v4 motion event.
			if strings.HasPrefix(m.SSpy.Info.Version, "4") {
				m.handleCameraMotion(&event)
			}
		case securityspy.EventTriggerAction:
			// v5 action event. (motion detected, actions enabled)
			m.handleCameraMotion(&event)
		case securityspy.EventTriggerMotion:
			// ignore this for now.
		case securityspy.EventStreamConnect:
			m.Info.Println("SecuritySpy Event Stream Connected!")
		case securityspy.EventStreamDisconnect:
			m.Error.Println("SecuritySpy Event Stream Disconnected")
		case securityspy.EventConfigChange:
			m.handleConfigChange()
		default:
			camName := ""
			if event.Camera != nil {
				camName = "camera: " + event.Camera.Name
			}

			m.Debug.Println("Event:", event.String(), camName, event.Msg)
		}
	}
}

func (m *Motifini) handleConfigChange() {
	m.saveSubDB() // just because.
	m.Info.Println("SecuritySpy Configuration Changed! Stopping Webserver and Messages to refresh SecuritySpy data.")

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

	subs := m.Subs.GetSubscribers(event.Camera.Name)
	reqID := messenger.ReqID(messenger.IDLength)
	path := filepath.Join(
		m.Conf.Global.TempDir, fmt.Sprintf("motifini_camera_motion_%s_%s.mp4", reqID, event.Camera.Name))

	subCount := len(subs)
	if subCount < 1 {
		return // no one to notify of this camera's motion
	}

	err := event.Camera.SaveVideo(
		&securityspy.VidOps{ACodec: defaultCodec, Height: defaultHeight}, defaultLength, defaultSize, path)
	if err != nil {
		m.Error.Printf("[%v] event.Camera.SaveVideo: %v", reqID, err)
		return
	}

	m.Msgs.SendFileOrMsg(reqID, "", path, subs)

	for _, sub := range subs {
		delay, ok := sub.Events.RuleGetD(event.Camera.Name, "delay")
		if !ok {
			delay = DefaultRepeatDelay
		}

		_ = sub.Events.Pause(event.Camera.Name, delay)
	}

	m.Info.Printf("[%v] Event '%v' triggered subscription messages. Subscribers: %v",
		reqID, event.Camera.Name, subCount)
}
