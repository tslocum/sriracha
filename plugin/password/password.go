package main

import "codeberg.org/tslocum/sriracha/plugin/password/password"

func Plugin() any {
	return password.NewPassword()
}

func main() {}
