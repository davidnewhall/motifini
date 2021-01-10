// Package export is a standalone package to deal with expvar data. Other packages
// can import this one to expose debug details on the expvar interface.
// More exports can be easily added.
package export

import (
	"expvar"
	"sync"
	"time"
)

// We hold a list of map pointers here, so we can retain data through reload.
var maps mapsList

// mapsList holds a list of reusable expvar maps.
type mapsList struct {
	list map[string]*expvar.Map
	sync.Mutex
}

// Data contains our expvar exports.
type Data struct {
	*expvar.Map
	StartAt    expvar.String
	Version    expvar.String
	ConfigFile expvar.String
	ListenPort expvar.Int
	HTTPVisits expvar.Int
	DefaultURL expvar.Int
	Files      expvar.Int
	Sent       expvar.Int
	Recv       expvar.Int
	Errors     expvar.Int
}

var Map *Data

// Init needs to be called before using the Map.
func Init(name string) {
	Map = &Data{Map: GetPublishedMap(name)}
	// If you add more above, make sure to add them to the map here.
	Map.Set("app_started", &Map.StartAt)
	Map.Set("app_version", &Map.Version)
	Map.Set("config_file", &Map.ConfigFile)
	Map.Set("listen_port", &Map.ListenPort)
	Map.Set("http_visits", &Map.HTTPVisits)
	Map.Set("default_url", &Map.DefaultURL)
	Map.Set("files_sent", &Map.Files)
	Map.Set("messge_sent", &Map.Sent)
	Map.Set("messge_recv", &Map.Recv)
	Map.Set("error_count", &Map.Errors)
	Map.StartAt.Set(time.Now().String())
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
