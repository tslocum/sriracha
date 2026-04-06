package main

import "codeberg.org/tslocum/sriracha/plugin/robot9000/robot9000"

func Plugin() any {
	return robot9000.NewRobot9000()
}

func main() {}
