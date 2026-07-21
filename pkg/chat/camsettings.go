package chat

import (
	"fmt"
	"strings"
	"time"

	"golift.io/securityspy/v2"
	"golift.io/subscribe"
)

// Global catalog keys for admin per-camera clip settings (not subscribeable).
const (
	camSettingsPrefix = "__cam:"

	ruleScale  = "scale"
	ruleLength = "length"
	ruleSize   = "size"

	ScaleFull    = "full"
	ScaleHalf    = "half"
	ScaleQuarter = "quarter"

	DefaultClipScale  = ScaleHalf
	DefaultClipLength = 6 * time.Second
	DefaultClipSize   = 1572864 // 1.5 MiB

	MinClipLengthSecs = 2
	MaxClipLengthSecs = 15
	MinClipSizeBytes  = 500 * 1024
	MaxClipSizeBytes  = 3 * 1024 * 1024
)

// ClipSettings is the admin-global capture profile for one camera.
type ClipSettings struct {
	Scale  string
	Length time.Duration
	Size   int // max file size in bytes
}

// CamSettingsKey returns the reserved catalog event name for a camera.
func CamSettingsKey(camName string) string {
	return camSettingsPrefix + camName
}

// IsCamSettingsKey reports whether name is a reserved camera-settings catalog entry.
func IsCamSettingsKey(name string) bool {
	return strings.HasPrefix(name, camSettingsPrefix)
}

// CatalogEventNames lists subscribable global events (excludes __cam: settings keys).
func CatalogEventNames(events *subscribe.Events) []string {
	if events == nil {
		return nil
	}

	names := events.Names()
	out := make([]string, 0, len(names))

	for _, name := range names {
		if IsCamSettingsKey(name) {
			continue
		}

		out = append(out, name)
	}

	return out
}

// EnsureCameraSettings creates a catalog entry with defaults if missing.
func EnsureCameraSettings(data *subscribe.Subscribe, camName string) {
	if data == nil || data.Events == nil || camName == "" {
		return
	}

	key := CamSettingsKey(camName)
	if data.Events.Exists(key) {
		return
	}

	_ = data.Events.New(key, &subscribe.Rules{
		S: map[string]string{ruleScale: DefaultClipScale},
		D: map[string]time.Duration{ruleLength: DefaultClipLength},
		I: map[string]int{ruleSize: DefaultClipSize},
	})
}

// GetCameraClipSettings returns stored settings or defaults.
func GetCameraClipSettings(data *subscribe.Subscribe, camName string) ClipSettings {
	settings := ClipSettings{
		Scale:  DefaultClipScale,
		Length: DefaultClipLength,
		Size:   DefaultClipSize,
	}

	if data == nil || data.Events == nil || camName == "" {
		return settings
	}

	key := CamSettingsKey(camName)
	if scale, ok := data.Events.RuleGetS(key, ruleScale); ok && validScale(scale) {
		settings.Scale = scale
	}

	if length, ok := data.Events.RuleGetD(key, ruleLength); ok && length > 0 {
		settings.Length = length
	}

	if size, ok := data.Events.RuleGetI(key, ruleSize); ok && size > 0 {
		settings.Size = size
	}

	return settings
}

func validScale(scale string) bool {
	switch scale {
	case ScaleFull, ScaleHalf, ScaleQuarter:
		return true
	default:
		return false
	}
}

func allowedClipLengthSecs(secs int) bool {
	return secs >= MinClipLengthSecs && secs <= MaxClipLengthSecs
}

func allowedClipSizeBytes(size int) bool {
	return size >= MinClipSizeBytes && size <= MaxClipSizeBytes
}

// heightForScale maps full/half/quarter to a request height.
// Zero means omit height/width (native / full-size stream).
func heightForScale(nativeHeight int, scale string) int {
	if nativeHeight < 2 {
		return 0
	}

	switch scale {
	case ScaleFull:
		return 0
	case ScaleQuarter:
		return evenPixels(nativeHeight / 4)
	default:
		return evenPixels(nativeHeight / 2)
	}
}

func evenPixels(value int) int {
	if value < 2 {
		return 0
	}

	if value%2 != 0 {
		value--
	}

	if value < 2 {
		return 0
	}

	return value
}

// VideoClipOps builds RTSP remux options from admin clip settings.
// Full scale omits width/height so SecuritySpy can stream-copy native resolution.
// Half / quarter request even dimensions derived from the camera aspect ratio.
func VideoClipOps(cam *securityspy.Camera, settings ClipSettings) *securityspy.VidOps {
	ops := &securityspy.VidOps{ACodec: "aac"}
	if cam == nil {
		return ops
	}

	ops.VCodec = cam.PreferredVCodec()

	height := heightForScale(cam.Height, settings.Scale)
	ops.Height = height

	if cam.Width > 0 && cam.Height > 0 && height > 0 {
		width := cam.Width * height / cam.Height
		ops.Width = evenPixels(width)
	}

	return ops
}

// FormatClipSettings summarizes settings for Telegram button labels.
func FormatClipSettings(settings ClipSettings) string {
	return fmt.Sprintf("%s · %s · %s",
		scaleLabel(settings.Scale),
		formatClipSecs(settings.Length),
		formatByteSize(settings.Size))
}

func scaleLabel(scale string) string {
	switch scale {
	case ScaleFull:
		return "full"
	case ScaleQuarter:
		return "¼"
	default:
		return "½"
	}
}

func formatClipSecs(dur time.Duration) string {
	secs := max(1, int(dur.Round(time.Second)/time.Second))

	return fmt.Sprintf("%ds", secs)
}

func formatByteSize(bytes int) string {
	const (
		bytesPerKB = 1024
		bytesPerMB = 1024 * 1024
	)

	if bytes >= bytesPerMB {
		whole := bytes / bytesPerMB
		frac := bytes % bytesPerMB
		if frac == 0 {
			return fmt.Sprintf("%dMB", whole)
		}

		// One decimal place (1.2MB, 1.5MB, 2.5MB).
		tenths := (bytes*10 + bytesPerMB/2) / bytesPerMB

		return fmt.Sprintf("%d.%dMB", tenths/10, tenths%10)
	}

	if bytes%bytesPerKB == 0 {
		return fmt.Sprintf("%dk", bytes/bytesPerKB)
	}

	return fmt.Sprintf("%dB", bytes)
}
