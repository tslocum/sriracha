package main

import "codeberg.org/tslocum/sriracha/plugin/statistics/statistics"

func Plugin() any {
	return statistics.NewStatistics()
}

func main() {}
