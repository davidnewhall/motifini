package motifini

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/davidnewhall/motifini/pkg/messenger"
	"golift.io/securityspy"
)

const (
	eventStreamBuf = 1000
	eventRetry     = 5 * time.Second
)

// ProcessEventStream processes the securityspy event stream.
func (m *Motifini) ProcessEventStream() {
	e := make(chan securityspy.Event, eventStreamBuf)

	m.SSpy.Events.BindChan(securityspy.EventAllEvents, e)
	m.SSpy.Events.Watch(eventRetry, false)

	go m.handleEvents(e)
}

func (m *Motifini) handleEvents(e chan securityspy.Event) {
	m.Info.Println("Event Stream Watcher Started")
	defer m.Error.Println("Event Stream Watcher Closed")

	for event := range e {
		switch event.Type { // nolint:exhaustive // use default wisely
		case securityspy.EventKeepAlive:
			// ignore.
		case securityspy.EventMotionDetected:
			// v4 motion event.
			if strings.HasPrefix(m.SSpy.Info.Version, "4") {
				m.handleCameraMotion(event)
			}
		case securityspy.EventTriggerAction:
			// v5 action event. (motion detected, actions enabled)
			m.handleCameraMotion(event)
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
	m.Info.Println("SecuritySpy Configuration Changed! Stopping Webserver and iMessage to refresh SecuritySpy data.")
	m.Msgs.Stop()

	if err := m.HTTP.Stop(); err != nil {
		m.Error.Println("Stopping Webserver:", err)
	}

	if err := m.SSpy.Refresh(); err != nil {
		m.Error.Println("Refreshing SecuritySpy Configuration:", err)
	}

	if err := m.Msgs.Start(); err != nil {
		m.Error.Println("Starting Message Watcher Routines:", err)
	}

	time.Sleep(time.Second)
	m.HTTP.Start()
}

func (m *Motifini) handleCameraMotion(e securityspy.Event) {
	if e.Camera == nil {
		return // this wont happen. check anyway.
	}

	subs := m.Subs.GetSubscribers(e.Camera.Name)
	id := messenger.ReqID(messenger.IDLength)
	path := filepath.Join(m.Conf.Global.TempDir, fmt.Sprintf("motifini_camera_motion_%s_%s.jpg", id, e.Camera.Name))

	subCount := len(subs)
	if subCount < 1 {
		return // no one to notify of this camera's motion
	}

	err := e.Camera.SaveJPEG(&securityspy.VidOps{Quality: 40, Width: 1080}, path) // nolint:gomnd
	if err != nil {
		m.Error.Printf("[%v] event.Camera.SaveJPEG: %v", id, err)
	}

	m.Msgs.SendFileOrMsg(id, "", path, subs)

	for _, sub := range subs {
		delay, ok := sub.Events.RuleGetD(e.Camera.Name, "delay")
		if !ok {
			delay = DefaultRepeatDelay
		}

		_ = sub.Events.Pause(e.Camera.Name, delay)
	}

	m.Info.Printf("[%v] Event '%v' triggered subscription messages. Subscribers: %v", id, e.Camera.Name, subCount)
}
