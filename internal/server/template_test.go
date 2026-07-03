package server

import (
	"testing"
)

func TestTemplate(t *testing.T) {
	s, err := newTestServer()
	if err != nil {
		t.Fatal(err)
	}

	err = s.validateTemplates(s)
	if err != nil {
		t.Fatal(err)
	}
}

// BenchmarkTemplate benchmarks rendering templates.
func BenchmarkTemplate(b *testing.B) {
	s, err := newTestServer()
	if err != nil {
		b.Fatal(err)
	}

	// Warm caches.
	err = s.validateTemplates(s)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err = s.validateTemplates(s)
		if err != nil {
			b.Fatal(err)
		}
	}
}
