// Package main demonstrates the zero-config bootstrap mode of the Helix framework.
// A single helix.Run() call is sufficient to start the application.
package main

import (
	"log"

	helix "github.com/enokdev/helix"
)

func main() {
	if err := helix.Run(); err != nil {
		log.Fatal(err)
	}
}
