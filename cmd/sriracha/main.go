package main

import (
	"log"

	"codeberg.org/tslocum/sriracha/internal/server"
)

func main() {
	s := server.NewServer()

	err := s.Run()
	if err != nil {
		log.Fatal(err)
	}
}
