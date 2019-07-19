package cli

import (
	"encoding/xml"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"

	"code.golift.io/imessage"
	"github.com/pkg/errors"
)

// TODO switch to securityspy library. that's why we wrote it.....

// SecuritySpySystemInfo represents a very small portion of the ++systemInfo method.
type SecuritySpySystemInfo struct {
	Cameralist struct {
		Camera []struct {
			Number string `xml:"number"`
			Name   string `xml:"name"`
		} `xml:"camera"`
	} `xml:"cameralist"`
}

// GetCamNumbers asks SecuritySpy for number of every defined camera.
func (m *Motifini) GetCamNumbers() error {
	var sysInfo SecuritySpySystemInfo
	if m.SecuritySpy.URL == "" {
		return nil
	}
	uri := m.SecuritySpy.URL + url.PathEscape("++systemInfo")
	Debugf("xxxx", "Refreshing SecuritySpy Camera List from %v", uri)
	resp, err := http.Get(uri)
	if err != nil {
		return errors.Wrap(err, "http.Get(url)")
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if body, err := ioutil.ReadAll(resp.Body); err != nil {
		return errors.Wrap(err, "ioutil.ReadAll")
	} else if err := xml.Unmarshal(body, &sysInfo); err != nil {
		return errors.Wrap(err, "xml.Unmarshal")
	}
	m.Config.Lock()
	defer m.Config.Unlock()
CAMERAS:
	for name, camData := range m.Cameras {
		for _, spycams := range sysInfo.Cameralist.Camera {
			if spycams.Name == name {
				Debugf("xxxx", "Got camera number %v for %v", spycams.Number, name)
				camData.Number = spycams.Number
				m.Cameras[name] = camData
				continue CAMERAS
			}
		}
		return errors.Errorf("defined camera not found in SecuritySpy: %v", name)
	}
	return nil
}

// GetPicture makes SecuritySpy save a pic. TODO: Switch to API?
func (m *Motifini) GetPicture(id, cam, output string) []error {
	if m.SecuritySpy.URL == "" {
		arg := `tell application "SecuritySpy" to capture image camera name "` + cam +
			`" as "` + output + `" with overwrite`
		if ok, errs := m.RunAppleScript(id, []string{arg}, 2); !ok {
			return errs
		}
		return nil
	}
	url := m.SecuritySpy.URL + "++image?cameraNum=" + url.PathEscape(m.Cameras[cam].Number) + "/"
	resp, err := http.Get(url)
	if err != nil {
		return []error{errors.Wrap(err, "http.Get(url)")}
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	file, err := os.Create(output)
	if err != nil {
		return []error{errors.Wrap(err, "os.Create")}
	}
	defer func() {
		_ = file.Close()
	}()
	if _, err = io.Copy(file, resp.Body); err != nil {
		return []error{errors.Wrap(err, "io.Copy")}
	}
	return nil
}

// DebugLog turns on logging (syslog) for debug messages.
var DebugLog bool

// Debugf writes Debug log lines.
func Debugf(id, msg string, v ...interface{}) {
	if DebugLog {
		log.Printf("[DEBUG] ["+id+"] "+msg, v...)
	}
}

func (m *Motifini) finishReq(w http.ResponseWriter, r *http.Request, id string, code int, reply string, msg imessage.Outgoing, cmd string) {
	if msg.Text != "" {
		m.Send(msg)
	}
	log.Printf(`[REQST] [%v] %v %v "%v %v" %d %d "%v" "%v"`, id, r.RemoteAddr, r.Host, r.Method, r.URL.String(), code, len(reply), r.UserAgent(), cmd)
	w.WriteHeader(code)
	if _, err := w.Write([]byte(reply)); err != nil {
		log.Printf("[ERROR] [%v] Error Sending Reply: %v", id, err)
	}
}

// handle any unknown URIs.
func (m *Motifini) handleAll(w http.ResponseWriter, r *http.Request) {
	m.exports.httpVisits.Add(1)
	m.exports.defaultURL.Add(1)
	id, code, reply := ReqID(4), 405, "FAIL\n"
	m.finishReq(w, r, id, code, reply, imessage.Outgoing{}, "-")
}

// ReqID makes a random string to identify requests in the logs.
func ReqID(n int) string {
	l := []rune("abcdefghjkmnopqrstuvwxyzABCDEFGHJKMNPQRTUVWXYZ23456789")
	b := make([]rune, n)
	for i := range b {
		b[i] = l[rand.Intn(len(l))]
	}
	return string(b)
}

// check for a thing in a thing.
func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
