package main

import "codeberg.org/tslocum/sriracha/plugin/wordfilter/wordfilter"

func Plugin() any {
	return wordfilter.NewWordfilter()
}

func main() {}
