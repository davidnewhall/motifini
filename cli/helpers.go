package cli

import (
	"log"
	"math/rand"
	"net/http"
)

func (m *Motifini) finishReq(w http.ResponseWriter, r *http.Request, id string, code int, reply string, cmd string) {
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
	m.finishReq(w, r, id, code, reply, "-")
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
