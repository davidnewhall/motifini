package export

import (
	"expvar"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetMap(t *testing.T) {
	t.Parallel()

	var strVal expvar.String

	assertion := assert.New(t)
	testMap := GetMap("MyShinyMap")

	testMap.Set("AnotherShinyMap", &strVal)
	strVal.Set("FinalShinyMap")
	assertion.Equal(`"FinalShinyMap"`, testMap.Get("AnotherShinyMap").String())

	// make sure we get the same map back.
	testMap = GetMap("MyShinyMap")
	assertion.Equal(`"FinalShinyMap"`, testMap.Get("AnotherShinyMap").String())
}

func TestGetPublishedMap(t *testing.T) {
	t.Parallel()

	var strVal expvar.String

	assertion := assert.New(t)
	testMap := GetPublishedMap("MyOtherShinyMap")

	testMap.Set("AnotherShinyMap", &strVal)
	strVal.Set("MyLastShinyMap")
	assertion.Equal(`"MyLastShinyMap"`, testMap.Get("AnotherShinyMap").String())

	// make sure we get the same map back.
	testMap = GetPublishedMap("MyOtherShinyMap")
	assertion.Equal(`"MyLastShinyMap"`, testMap.Get("AnotherShinyMap").String())
}

func TestInit(t *testing.T) {
	t.Parallel()

	assertion := assert.New(t)

	assertion.Nil(Map, "the map must begin nil")
	Init("myCoolMapName")
	assertion.NotNil(Map, "the map var must not be nil after initialization")
	assertion.NotNil(Map.Map, "the map's var map struct must not be nil after initialization")
}
