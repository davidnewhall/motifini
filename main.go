// Package main is the Motifini entrypoint.
package main

import (
	"errors"
	"log"
	"net/http"

	"github.com/davidnewhall/motifini/pkg/motifini"
)

func main() {
	err := motifini.Start()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalln("[ERROR]", err)
	}
}
