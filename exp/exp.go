package exp

import (
	"expvar"
	"sync"
)

// We hold a list of map pointers here, so we can retain data through reload.
var maps Maps

// Maps holds a list of reusable expvar maps.
type Maps struct {
	list map[string]*expvar.Map
	sync.Mutex
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
