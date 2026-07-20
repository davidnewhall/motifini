package chat

import (
	"testing"
	"time"

	"golift.io/securityspy/v2"
	"golift.io/subscribe"
)

func TestCameraSubKey(t *testing.T) {
	t.Parallel()

	if got := CameraSubKey("Office", ClassAny); got != "Office" {
		t.Fatalf("any: got %q", got)
	}

	if got := CameraSubKey("Office", ClassHuman); got != "Office:human" {
		t.Fatalf("human: got %q", got)
	}
}

func TestParseCameraSubKey(t *testing.T) {
	t.Parallel()

	cam, class := ParseCameraSubKey("Office")
	if cam != "Office" || class != ClassAny {
		t.Fatalf("legacy: %q %q", cam, class)
	}

	cam, class = ParseCameraSubKey("Office:vehicle")
	if cam != "Office" || class != ClassVehicle {
		t.Fatalf("classed: %q %q", cam, class)
	}
}

func TestNotifyKeys(t *testing.T) {
	t.Parallel()

	keys := NotifyKeys("Office", []securityspy.TriggerEvent{
		securityspy.TriggerByMotion,
		securityspy.TriggerByHumanDetection,
	})

	want := map[string]bool{
		"Office":        true,
		"Office:motion": true,
		"Office:human":  true,
	}
	if len(keys) != len(want) {
		t.Fatalf("len=%d keys=%v", len(keys), keys)
	}
	for _, k := range keys {
		if !want[k] {
			t.Fatalf("unexpected key %q in %v", k, keys)
		}
	}
}

func TestCommandName(t *testing.T) {
	t.Parallel()

	if got := commandName("/sub@MyBot"); got != "sub" {
		t.Fatalf("got %q", got)
	}
}

func TestActiveKeysAmong(t *testing.T) {
	t.Parallel()

	if ActiveKeysAmong(nil, []string{"Office:human"}) != nil {
		t.Fatal("nil sub")
	}

	sub := &subscribe.Subscriber{Events: &subscribe.Events{Map: make(map[string]*subscribe.Rules)}}
	_ = sub.Subscribe("Office:human")

	got := ActiveKeysAmong(sub, []string{"Office", "Office:human", "Office:motion"})
	if len(got) != 1 || got[0] != "Office:human" {
		t.Fatalf("got %v", got)
	}

	_ = sub.Events.Pause("Office:human", time.Hour)
	if got = ActiveKeysAmong(sub, []string{"Office:human"}); len(got) != 0 {
		t.Fatalf("paused should be excluded: %v", got)
	}
}
