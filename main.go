package main

import (
	"log"

	"github.com/oneslash/upwind-cli/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
