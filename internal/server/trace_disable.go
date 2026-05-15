//go:build !trace

package server

import "time"

const trace = false

func traceLog(message string, duration time.Duration) {
	// Tracing disabled.
}
