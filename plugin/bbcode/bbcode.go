package main

import "codeberg.org/tslocum/sriracha/plugin/bbcode/bbcode"

func Plugin() any {
	return bbcode.NewBBCode()
}

func main() {}
