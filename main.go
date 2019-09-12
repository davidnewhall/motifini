package main

import (
	"log"
	"net/http"

	"github.com/davidnewhall/motifini/cli"
)

func main() {
	if err := cli.Start(); err != nil && err != http.ErrServerClosed {
		log.Fatalln("[ERROR]", err)
	}
}
