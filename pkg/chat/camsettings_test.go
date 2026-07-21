package chat

import (
	"testing"
	"time"

	"golift.io/securityspy/v2"
	"golift.io/subscribe"
)

func TestHeightForScale(t *testing.T) {
	t.Parallel()

	if got := heightForScale(1440, ScaleFull); got != 0 {
		t.Fatalf("full: got %d want 0", got)
	}

	if got := heightForScale(1440, ScaleHalf); got != 720 {
		t.Fatalf("half 1440: got %d want 720", got)
	}

	if got := heightForScale(1440, ScaleQuarter); got != 360 {
		t.Fatalf("quarter 1440: got %d want 360", got)
	}

	if got := heightForScale(1728, ScaleHalf); got != 864 {
		t.Fatalf("half 1728: got %d want 864", got)
	}

	if got := heightForScale(1081, ScaleHalf); got%2 != 0 {
		t.Fatalf("half odd native: got odd height %d", got)
	}
}

func TestGetCameraClipSettingsDefaults(t *testing.T) {
	t.Parallel()

	got := GetCameraClipSettings(nil, "Office")
	if got.Scale != DefaultClipScale || got.Length != DefaultClipLength || got.Size != DefaultClipSize {
		t.Fatalf("defaults: %+v", got)
	}
}

func TestCameraClipSettingsRoundTrip(t *testing.T) {
	t.Parallel()

	events := &subscribe.Events{Map: make(map[string]*subscribe.Rules)}
	data := &subscribe.Subscribe{Events: events}

	EnsureCameraSettings(data, "Mailbox")
	key := CamSettingsKey("Mailbox")
	events.RuleSetS(key, ruleScale, ScaleQuarter)
	events.RuleSetD(key, ruleLength, 10*time.Second)
	events.RuleSetI(key, ruleSize, 800*1024)

	got := GetCameraClipSettings(data, "Mailbox")
	if got.Scale != ScaleQuarter || got.Length != 10*time.Second || got.Size != 800*1024 {
		t.Fatalf("got %+v", got)
	}

	_ = events.New("Door Bell", nil)

	names := CatalogEventNames(events)
	foundDoor := false
	for _, name := range names {
		if IsCamSettingsKey(name) {
			t.Fatalf("catalog leaked settings key %q", name)
		}
		if name == "Door Bell" {
			foundDoor = true
		}
	}
	if !foundDoor {
		t.Fatal("expected Door Bell in catalog names")
	}
}

func TestVideoClipOpsScale(t *testing.T) {
	t.Parallel()

	cam := &securityspy.Camera{Name: "Office", Width: 2560, Height: 1440, VideoFormat: "H.265"}
	ops := VideoClipOps(cam, ClipSettings{Scale: ScaleQuarter})
	if ops.Height != 360 {
		t.Fatalf("height: got %d want 360", ops.Height)
	}
	if ops.Width != 640 {
		t.Fatalf("width: got %d want 640", ops.Width)
	}
	if ops.VCodec != "h265" {
		t.Fatalf("vcodec: got %q", ops.VCodec)
	}

	full := VideoClipOps(cam, ClipSettings{Scale: ScaleFull})
	if full.Height != 0 || full.Width != 0 {
		t.Fatalf("full should omit size: %+v", full)
	}
}
