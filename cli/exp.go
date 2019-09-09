package cli

import (
	"expvar"
	"sync"
	"time"
)

// Contains our expvar exports.
type exportData struct {
	*expvar.Map
	startAt    expvar.String
	version    expvar.String
	configFile expvar.String
	listenPort expvar.Int
	httpVisits expvar.Int
	defaultURL expvar.Int
	videos     expvar.Int
	pics       expvar.Int
	texts      expvar.Int
	errors     expvar.Int
}

// We hold a list of map pointers here, so we can retain data through reload.
var maps mapsList

// mapsList holds a list of reusable expvar maps.
type mapsList struct {
	list map[string]*expvar.Map
	sync.Mutex
}

// exportData makes all the expvar data available. Only needs to run once.
func (m *Motifini) exportData() {
	m.exports.Map = GetPublishedMap("iMessageRelay")
	m.exports.Set("app_started", &m.exports.startAt)
	m.exports.Set("app_version", &m.exports.version)
	m.exports.Set("config_file", &m.exports.configFile)
	m.exports.Set("listen_port", &m.exports.listenPort)
	m.exports.Set("http_visits", &m.exports.httpVisits)
	m.exports.Set("default_url", &m.exports.defaultURL)
	m.exports.Set("videos_sent", &m.exports.videos)
	m.exports.Set("photos_sent", &m.exports.pics)
	m.exports.Set("messge_sent", &m.exports.texts)
	m.exports.Set("error_count", &m.exports.errors)
	// Set static data now.
	m.exports.startAt.Set(time.Now().String())
	m.exports.version.Set(Version)
	m.exports.configFile.Set(m.Flags.ConfigFile)
	m.exports.listenPort.Set(int64(m.Config.Global.Port))
}

// GetMap returns an unpublished map if one exists, or returns a new one.
func GetMap(name string) *expvar.Map {
	maps.Lock()
	defer maps.Unlock()
	if maps.list == nil {
		maps.list = make(map[string]*expvar.Map)
	}
	if m, mapExists := maps.list[name]; mapExists {
		return m
	}
	maps.list[name] = new(expvar.Map).Init()
	return maps.list[name]
}

// GetPublishedMap returns a published map if one exists, or returns a new one.
func GetPublishedMap(name string) *expvar.Map {
	if p := expvar.Get(name); p != nil {
		return p.(*expvar.Map)
	}
	p := GetMap(name)
	expvar.Publish(name, p)
	return p
}
