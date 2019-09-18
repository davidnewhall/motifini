package main

import (
	"log"
	"net/http"

	"github.com/davidnewhall/motifini/pkg/motifini"
)

func main() {
	if err := motifini.Start(); err != nil && err != http.ErrServerClosed {
		log.Fatalln("[ERROR]", err)
	}
}
