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

	// Mailbox: exact half (720) stream-copies; stay at 718.
	if got := heightForScale(1440, ScaleHalf); got != 718 {
		t.Fatalf("half 1440: got %d want 718", got)
	}

	if got := heightForScale(1440, ScaleQuarter); got != 360 {
		t.Fatalf("quarter 1440: got %d want 360", got)
	}

	// Pool 1728: half−1 → 862 (below 864 cliff).
	if got := heightForScale(1728, ScaleHalf); got != 862 {
		t.Fatalf("half 1728: got %d want 862", got)
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

func TestGetCameraClipSettingsClamps(t *testing.T) {
	t.Parallel()

	events := &subscribe.Events{Map: make(map[string]*subscribe.Rules)}
	data := &subscribe.Subscribe{Events: events}
	EnsureCameraSettings(data, "Office")
	key := CamSettingsKey("Office")
	events.RuleSetD(key, ruleLength, 60*time.Second)
	events.RuleSetI(key, ruleSize, 50*1024*1024)

	got := GetCameraClipSettings(data, "Office")
	if got.Length != time.Duration(MaxClipLengthSecs)*time.Second {
		t.Fatalf("length: got %v want %ds", got.Length, MaxClipLengthSecs)
	}
	if got.Size != MaxClipSizeBytes {
		t.Fatalf("size: got %d want %d", got.Size, MaxClipSizeBytes)
	}

	events.RuleSetD(key, ruleLength, time.Second)
	events.RuleSetI(key, ruleSize, 100)
	got = GetCameraClipSettings(data, "Office")
	if got.Length != time.Duration(MinClipLengthSecs)*time.Second {
		t.Fatalf("min length: got %v", got.Length)
	}
	if got.Size != MinClipSizeBytes {
		t.Fatalf("min size: got %d", got.Size)
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

	half := VideoClipOps(cam, ClipSettings{Scale: ScaleHalf})
	if half.Height != 718 {
		t.Fatalf("half height: got %d want 718", half.Height)
	}
	if half.Width != 1276 {
		t.Fatalf("half width: got %d want 1276", half.Width)
	}

	full := VideoClipOps(cam, ClipSettings{Scale: ScaleFull})
	if full.Height != 0 || full.Width != 0 {
		t.Fatalf("full should omit size: %+v", full)
	}
}
