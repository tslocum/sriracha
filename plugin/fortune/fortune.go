package main

import "codeberg.org/tslocum/sriracha/plugin/fortune/fortune"

func Plugin() any {
	return fortune.NewFortune()
}

func main() {}
