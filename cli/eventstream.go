package cli

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"golift.io/securityspy"
)

// processEventStream processes the securityspy event stream.
func (m *Motifini) processEventStream() {
	c := make(chan securityspy.Event, 100)
	m.SSpy.Events.BindChan(securityspy.EventAllEvents, c)
	go m.SSpy.Events.Watch(5*time.Second, true)
	go m.handleEvents(c)
}

func (m *Motifini) handleEvents(c chan securityspy.Event) {
	m.Info.Println("Event Stream Watcher Started")
	defer m.Warn.Println("Event Stream Watcher Closed")
	for event := range c {
		switch event.Type {
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
			m.saveSubDB()
			fallthrough
		default:
			camName := ""
			if event.Camera != nil {
				camName = "camera: " + event.Camera.Name
			}
			m.Debug.Println("Event:", event.String(), camName, event.Msg)
		}
	}
}

func (m *Motifini) handleCameraMotion(e securityspy.Event) {
	if e.Camera == nil {
		return // this wont happen. check anyway.
	}
	subs := m.Subs.GetSubscribers(e.Camera.Name)
	subCount := len(subs)
	if subCount < 1 {
		return // no one to notify of this camera's motion
	}
	id := ReqID(3)
	path := filepath.Join(m.Conf.Global.TempDir, fmt.Sprintf("camera_motion_%s_%s.jpg", id, e.Camera.Name))
	if err := e.Camera.SaveJPEG(&securityspy.VidOps{}, path); err != nil {
		m.Error.Printf("[%v] event.Camera.SaveJPEG: %v", id, err)
	}
	m.sendFileOrMsg(id, "", path, subs)
	for _, sub := range subs {
		delay, ok := sub.Events.RuleGetD(e.Camera.Name, "delay")
		if ok {
			_ = sub.Events.Pause(e.Camera.Name, delay)
		}
		_ = sub.Events.Pause(e.Camera.Name, time.Minute)
	}
	m.Info.Printf("[%v] Event '%v' triggered subscription messages. Subscribers: %v", id, e.Camera.Name, subCount)
}
