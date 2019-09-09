package main

import (
	"log"

	"github.com/davidnewhall/motifini/cli"
)

func main() {
	if err := cli.Start(); err != nil {
		log.Println("[ERROR]", err)
	}
}
