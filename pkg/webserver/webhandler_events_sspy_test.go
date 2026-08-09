package webserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/davidnewhall/motifini/pkg/messenger"
	"golift.io/securityspy/v2"
	"golift.io/securityspy/v2/server"
)

// fakeSystemInfo is a minimal ++systemInfo reply: Garage (3) is online,
// Porch (1) is offline.
const fakeSystemInfo = `<?xml version='1.0' encoding='utf-8'?>
<system>
  <camera-list>
    <camera><number>3</number><name>Garage</name><connected>true</connected>
      <video-width>2560</video-width><video-height>1440</video-height></camera>
    <camera><number>1</number><name>Porch</name><connected>false</connected>
      <video-width>3072</video-width><video-height>2048</video-height></camera>
  </camera-list>
  <server><name>SecuritySpy</name><version>6.20</version><uuid>FAKEUUID</uuid></server>
</system>`

// fakeSSpy is a minimal SecuritySpy stand-in: systemInfo for Refresh, ++image
// for SaveJPEG. It records ++image camera numbers so tests can tell whether a
// capture was actually attempted.
type fakeSSpy struct {
	mu        sync.Mutex
	imageCams []string
}

func (f *fakeSSpy) handler(resp http.ResponseWriter, req *http.Request) {
	switch req.URL.Path {
	case "/++systemInfo":
		resp.Header().Set("Content-Type", "application/xml")
		_, _ = resp.Write([]byte(fakeSystemInfo))
	case "/++image":
		f.mu.Lock()
		f.imageCams = append(f.imageCams, req.URL.Query().Get("cameraNum"))
		f.mu.Unlock()
		resp.Header().Set("Content-Type", "image/jpeg")
		_, _ = resp.Write([]byte("\xff\xd8\xff\xd9")) // minimal JPEG SOI/EOI
	default:
		http.NotFound(resp, req)
	}
}

func (f *fakeSSpy) imageRequests() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]string(nil), f.imageCams...)
}

func testConfigWithSSpy(t *testing.T) (*Config, *fakeSSpy) {
	t.Helper()

	fake := &fakeSSpy{}
	httpServer := httptest.NewServer(http.HandlerFunc(fake.handler))
	t.Cleanup(httpServer.Close)

	cfg, _ := testConfig(t)
	cfg.SSpy = securityspy.NewMust(&server.Config{
		Username: "user",
		Password: "pass",
		URL:      httpServer.URL + "/",
	})

	err := cfg.SSpy.Refresh()
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	return cfg, fake
}

func TestCameraByNameOrNum(t *testing.T) {
	t.Parallel()

	cfg, _ := testConfigWithSSpy(t)

	for _, want := range []string{"Garage", "garage", "3", "Porch", "1"} {
		if cam := cfg.cameraByNameOrNum(want); cam == nil {
			t.Fatalf("cameraByNameOrNum(%q): got nil, want camera", want)
		}
	}

	for _, miss := range []string{"Nope", "99", "", "3.5"} {
		if cam := cfg.cameraByNameOrNum(miss); cam != nil {
			t.Fatalf("cameraByNameOrNum(%q): got %s, want nil", miss, cam.Name)
		}
	}
}

func TestNotifyPhotoWithFakeSSpy(t *testing.T) {
	t.Parallel()

	cfg, fake := testConfigWithSSpy(t)

	sub := cfg.Subs.CreateSubWithID(1234, "someone", messenger.APITelegram, false, false)

	err := sub.Subscribe("garage_event")
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	// By name, with a caption.
	rec := doRequest(cfg.eventsHandler, "POST",
		"/api/v1.0/event/notify/garage_event?camera=Garage&msg=door+open", "", "",
		map[string]string{"cmd": "notify", "event": "garage_event"})
	if rec.Code != http.StatusOK {
		t.Fatalf("notify by name: code=%d body=%q", rec.Code, rec.Body.String())
	}

	// By SecuritySpy number, no caption (media defaults to photo).
	rec = doRequest(cfg.eventsHandler, "POST",
		"/api/v1.0/event/notify/garage_event?camera=3", "", "",
		map[string]string{"cmd": "notify", "event": "garage_event"})
	if rec.Code != http.StatusOK {
		t.Fatalf("notify by number: code=%d body=%q", rec.Code, rec.Body.String())
	}

	if got := fake.imageRequests(); len(got) != 2 || got[0] != "3" || got[1] != "3" {
		t.Fatalf("image requests: got %v want [3 3]", got)
	}

	// Unknown camera is a 400 and hits SecuritySpy not at all.
	rec = doRequest(cfg.eventsHandler, "POST",
		"/api/v1.0/event/notify/garage_event?camera=Nope&media=photo", "", "",
		map[string]string{"cmd": "notify", "event": "garage_event"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown camera: code=%d want 400 body=%q", rec.Code, rec.Body.String())
	}

	if got := fake.imageRequests(); len(got) != 2 {
		t.Fatalf("image requests after 400: got %v want 2 total", got)
	}
}

func TestCaptureNotifyMediaOffline(t *testing.T) {
	t.Parallel()

	cfg, fake := testConfigWithSSpy(t)
	req := &notifyRequest{
		event: "porch_event", msg: "motion at the porch",
		media: mediaPhoto, cam: cfg.cameraByNameOrNum("Porch"),
	}

	msg, path, code, _ := cfg.captureNotifyMedia("reqid", req, 1)

	if code != http.StatusOK || path != "" {
		t.Fatalf("offline: code=%d path=%q, want 200 and no file", code, path)
	}

	if !strings.Contains(msg, "motion at the porch") || !strings.Contains(msg, "Porch is offline") {
		t.Fatalf("offline message: %q", msg)
	}

	// No message: the note becomes the message.
	req.msg = ""

	msg, _, code, _ = cfg.captureNotifyMedia("reqid", req, 1)
	if code != http.StatusOK || !strings.Contains(msg, "Porch is offline") {
		t.Fatalf("offline note-only message: code=%d msg=%q", code, msg)
	}

	if got := fake.imageRequests(); len(got) != 0 {
		t.Fatalf("offline camera must not be captured: %v", got)
	}
}

func TestCaptureNotifyMediaVideoFailure(t *testing.T) {
	t.Parallel()

	cfg, _ := testConfigWithSSpy(t)
	req := &notifyRequest{
		event: "garage_event", msg: "check this",
		media: mediaVideo, cam: cfg.cameraByNameOrNum("Garage"),
	}

	// The fake speaks HTTP, not RTSP, so the clip capture fails fast.
	msg, path, code, _ := cfg.captureNotifyMedia("reqid", req, 1)

	if code != http.StatusInternalServerError || path != "" {
		t.Fatalf("capture failure: code=%d path=%q, want 500 and no file", code, path)
	}

	if !strings.Contains(msg, "check this") || !strings.Contains(msg, "Couldn't capture video from Garage") {
		t.Fatalf("capture failure message: %q", msg)
	}
}
