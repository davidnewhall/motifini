package chat

import (
	"testing"
	"time"

	"golift.io/securityspy/v2"
	"golift.io/subscribe"
)

func TestFormatDuration(t *testing.T) {
	t.Parallel()

	cases := []struct {
		d    time.Duration
		want string
	}{
		{0, "0 seconds"},
		{500 * time.Millisecond, "0 seconds"},
		{time.Minute, "1 minute"},
		{2 * time.Minute, "2 minutes"},
		{30 * time.Second, "30 seconds"},
		{time.Hour, "1 hour"},
	}
	for _, tc := range cases {
		if got := formatDuration(tc.d); got != tc.want {
			t.Fatalf("%v: got %q want %q", tc.d, got, tc.want)
		}
	}
}

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
