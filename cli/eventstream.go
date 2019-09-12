package cli

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	"golift.io/securityspy"
)

// processEventStream processes the securityspy event stream.
func (m *Motifini) processEventStream() {
	c := make(chan securityspy.Event, 100)
	m.Spy.Events.BindChan(securityspy.EventAllEvents, c)
	go m.Spy.Events.Watch(5*time.Second, true)
	log.Println("[INFO] Event Stream Watcher Started")
	for event := range c {
		switch event.Type {
		case securityspy.EventKeepAlive:
			// ignore.
		case securityspy.EventMotionDetected:
			// v4 motion event.
			if strings.HasPrefix(m.Spy.Info.Version, "4") {
				m.handleCameraMotion(event)
			}
		case securityspy.EventTriggerAction:
			// v5 action event. (motion detected, actions enabled)
			m.handleCameraMotion(event)
		case securityspy.EventTriggerMotion:
			// ignore this for now.
		case securityspy.EventStreamConnect:
			log.Println("[INFO] SecuritySpy Event Stream Connected!")
		case securityspy.EventStreamDisconnect:
			log.Println("[ERROR] SecuritySpy Event Stream Disconnected")
		case securityspy.EventConfigChange:
			m.save()
			fallthrough
		default:
			camName := ""
			if event.Camera != nil {
				camName = "camera: " + event.Camera.Name
			}
			m.Debug.Println("[EVENT]", event.String(), camName, event.Msg)
		}
	}
	log.Println("[INFO] Event Stream Watcher Closed")
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
	path := filepath.Join(m.Config.Global.TempDir, fmt.Sprintf("imessage_relay_%s_%s.jpg", id, e.Camera.Name))
	if err := e.Camera.SaveJPEG(&securityspy.VidOps{}, path); err != nil {
		log.Printf("[ERROR] [%v] event.Camera.SaveJPEG: %v", id, err)
	}
	m.sendFileOrMsg(id, "", path, subs)
	for _, sub := range subs {
		// one per minute until we upgrade the subscribe module.
		_ = sub.Pause(e.Camera.Name, time.Minute)
	}
	m.Debug.Printf("[%v] Event '%v' triggered subscription messages. Subscribers: %v", id, e.Camera.Name, subCount)
}
