package main

import (
	"log"

	"codeberg.org/tslocum/sriracha/server"
)

func main() {
	s := server.New()

	err := s.Run()
	if err != nil {
		log.Fatal(err)
	}
}
