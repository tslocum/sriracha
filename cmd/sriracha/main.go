package main

import (
	"log"

	"codeberg.org/tslocum/sriracha"
)

func main() {
	s := sriracha.NewServer()

	err := s.Run()
	if err != nil {
		log.Fatal(err)
	}
}
