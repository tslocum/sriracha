//go:build trace

package server

import (
	"fmt"
	"log"
	"time"
)

const trace = true

func traceLog(message string, duration time.Duration) {
	var label string
	if duration >= time.Millisecond {
		label = fmt.Sprintf("%s", duration.Round(time.Millisecond))
	} else {
		label = fmt.Sprintf("%.1fms", float64(duration)/float64(time.Millisecond))
	}
	log.Printf("%s took %s", message, label)
}
