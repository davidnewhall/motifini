package chat

import (
	"strings"

	"golift.io/securityspy/v2"
	"golift.io/subscribe"
)

// Camera subscription classifications (Telegram wizard + text /sub).
const (
	ClassAny     = "any"
	ClassMotion  = "motion"
	ClassHuman   = "human"
	ClassVehicle = "vehicle"
	ClassAnimal  = "animal"
)

const classSep = ":"

// CameraSubKey builds the subscribe event key for a camera + classification.
// ClassAny (or empty) keeps the bare camera name for backward compatibility.
func CameraSubKey(cameraName, class string) string {
	class = normalizeClass(class)
	if class == ClassAny || class == "" {
		return cameraName
	}

	return cameraName + classSep + class
}

// ParseCameraSubKey splits a subscription key into camera name and class.
// Bare names (legacy) are ClassAny.
func ParseCameraSubKey(key string) (string, string) {
	camera, class, ok := strings.Cut(key, classSep)
	if !ok || class == "" {
		return key, ClassAny
	}

	return camera, normalizeClass(class)
}

func normalizeClass(class string) string {
	switch strings.ToLower(strings.TrimSpace(class)) {
	case "", ClassAny, "*", "all":
		return ClassAny
	case ClassMotion, "m":
		return ClassMotion
	case ClassHuman, "h", "person", "people":
		return ClassHuman
	case ClassVehicle, "v", "car":
		return ClassVehicle
	case ClassAnimal, "a":
		return ClassAnimal
	default:
		return strings.ToLower(strings.TrimSpace(class))
	}
}

func classShort(class string) string {
	switch normalizeClass(class) {
	case ClassMotion:
		return "m"
	case ClassHuman:
		return "h"
	case ClassVehicle:
		return "v"
	case ClassAnimal:
		return "a"
	default:
		return "*"
	}
}

func classFromShort(short string) string {
	switch short {
	case "m":
		return ClassMotion
	case "h":
		return ClassHuman
	case "v":
		return ClassVehicle
	case "a":
		return ClassAnimal
	default:
		return ClassAny
	}
}

func classLabel(class string) string {
	switch normalizeClass(class) {
	case ClassMotion:
		return "Motion"
	case ClassHuman:
		return "Human"
	case ClassVehicle:
		return "Vehicle"
	case ClassAnimal:
		return "Animal"
	default:
		return "Any"
	}
}

// cameraSubBadges returns compact [M][H][V][A] markers for a camera's active class subs.
// Legacy bare-camera subscriptions show as [*].
func cameraSubBadges(sub *subscribe.Subscriber, camName string) string {
	if sub == nil || sub.Events == nil {
		return ""
	}

	var badge strings.Builder
	for _, class := range []string{ClassMotion, ClassHuman, ClassVehicle, ClassAnimal} {
		if sub.Events.Name(CameraSubKey(camName, class)) != "" {
			badge.WriteByte('[')
			badge.WriteString(strings.ToUpper(classShort(class)))
			badge.WriteByte(']')
		}
	}

	if sub.Events.Name(camName) != "" {
		badge.WriteString("[*]")
	}

	return badge.String()
}

// ClassesFromReasons maps SecuritySpy trigger reason bits to subscription classes.
func ClassesFromReasons(reasons []securityspy.TriggerEvent) []string {
	seen := map[string]bool{}
	var out []string

	add := func(class string) {
		if seen[class] {
			return
		}
		seen[class] = true
		out = append(out, class)
	}

	for _, reason := range reasons {
		switch reason {
		case securityspy.TriggerByHumanDetection,
			securityspy.TriggerByHumanArrival,
			securityspy.TriggerByHumanDeparture:
			add(ClassHuman)
		case securityspy.TriggerByVehicleDetection,
			securityspy.TriggerByVehicleArrival,
			securityspy.TriggerByVehicleDeparture:
			add(ClassVehicle)
		case securityspy.TriggerByAnimalDetection,
			securityspy.TriggerByAnimalArrival,
			securityspy.TriggerByAnimalDeparture:
			add(ClassAnimal)
		case securityspy.TriggerByMotion,
			securityspy.TriggerByAudio,
			securityspy.TriggerByCameraEvent,
			securityspy.TriggerByOtherCamera,
			securityspy.TriggerByManual,
			securityspy.TriggerByWebServer,
			securityspy.TriggerByScript,
			securityspy.TriggerByHomeKitEvent:
			add(ClassMotion)
		}
	}

	if len(out) == 0 {
		add(ClassMotion)
	}

	return out
}

// NotifyKeys returns subscription keys that should fire for a camera event.
// Bare camera names (legacy ClassAny) are still included so old subscriptions keep working.
func NotifyKeys(cameraName string, reasons []securityspy.TriggerEvent) []string {
	classes := ClassesFromReasons(reasons)
	keys := make([]string, 0, 1+len(classes))
	keys = append(keys, cameraName) // legacy "any" subs

	for _, class := range classes {
		keys = append(keys, CameraSubKey(cameraName, class))
	}

	return keys
}

// Media caption kinds for on-demand grabs (not SecuritySpy classifications).
const (
	CaptionPhoto = "photo"
	CaptionVideo = "video"
)

// CameraCaption formats a Telegram media caption: "Pool (human)".
func CameraCaption(name, kind string) string {
	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind == "" {
		return name
	}

	return name + " (" + kind + ")"
}

// EventCaption builds a media caption from SecuritySpy trigger reasons.
// Prefers detection classes (human/vehicle/animal) over bare motion when both fire.
func EventCaption(name string, reasons []securityspy.TriggerEvent) string {
	return CameraCaption(name, eventClassKind(reasons))
}

func eventClassKind(reasons []securityspy.TriggerEvent) string {
	classes := ClassesFromReasons(reasons)

	var specific []string
	for _, c := range classes {
		if c != ClassMotion {
			specific = append(specific, c)
		}
	}

	if len(specific) > 0 {
		return strings.Join(specific, ", ")
	}

	return ClassMotion
}
