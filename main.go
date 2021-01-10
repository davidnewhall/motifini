package main

import (
	"errors"
	"log"
	"net/http"

	"github.com/davidnewhall/motifini/pkg/motifini"
)

func main() {
	if err := motifini.Start(); err != nil &&
		!errors.Is(err, http.ErrServerClosed) {
		log.Fatalln("[ERROR]", err)
	}
}
