package main

import "codeberg.org/tslocum/sriracha/plugin/irc/irc"

func Plugin() any {
	return irc.NewIRC()
}

func main() {}
