package main

import (
	"expvar"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetMap(t *testing.T) {
	t.Parallel()
	var d expvar.String
	a := assert.New(t)
	testMap := GetMap("MyShinyMap")
	testMap.Set("AnotherShinyMap", &d)
	d.Set("FinalShinyMap")
	a.EqualValues(`"FinalShinyMap"`, testMap.Get("AnotherShinyMap").String())
	// make sure we get the same map back.
	testMap = GetMap("MyShinyMap")
	a.EqualValues(`"FinalShinyMap"`, testMap.Get("AnotherShinyMap").String())
}

func TestGetPublishedMap(t *testing.T) {
	t.Parallel()
	var d expvar.String
	a := assert.New(t)
	testMap := GetPublishedMap("MyOtherShinyMap")
	testMap.Set("AnotherShinyMap", &d)
	d.Set("MyLastShinyMap")
	a.EqualValues(`"MyLastShinyMap"`, testMap.Get("AnotherShinyMap").String())
	// make sure we get the same map back.
	testMap = GetPublishedMap("MyOtherShinyMap")
	a.EqualValues(`"MyLastShinyMap"`, testMap.Get("AnotherShinyMap").String())
}
