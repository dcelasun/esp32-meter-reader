package main

import (
	"math"
	"testing"
)

func TestCheckReadingFilter(t *testing.T) {
	tests := []struct {
		name     string
		divided  float64
		prev     float64
		incrOnly bool
		maxIncr  float64
		wantOK   bool // true = accepted (empty reason)
	}{
		// --- First reading (prev is NaN) always accepted ---
		{
			name:     "first reading accepted with incr-only",
			divided:  100,
			prev:     math.NaN(),
			incrOnly: true,
			wantOK:   true,
		},
		{
			name:    "first reading accepted with max-incr",
			divided: 100,
			prev:    math.NaN(),
			maxIncr: 10,
			wantOK:  true,
		},

		// --- incr-only tests ---
		{
			name:     "incr-only accepts equal reading",
			divided:  100,
			prev:     100,
			incrOnly: true,
			wantOK:   true,
		},
		{
			name:     "incr-only accepts higher reading",
			divided:  101,
			prev:     100,
			incrOnly: true,
			wantOK:   true,
		},
		{
			name:     "incr-only rejects lower reading",
			divided:  99,
			prev:     100,
			incrOnly: true,
			wantOK:   false,
		},

		// --- max-incr tests ---
		{
			name:    "max-incr accepts increase within limit",
			divided: 105,
			prev:    100,
			maxIncr: 10,
			wantOK:  true,
		},
		{
			name:    "max-incr accepts increase exactly at limit",
			divided: 110,
			prev:    100,
			maxIncr: 10,
			wantOK:  true,
		},
		{
			name:    "max-incr rejects increase above limit",
			divided: 111,
			prev:    100,
			maxIncr: 10,
			wantOK:  false,
		},
		{
			name:    "max-incr accepts decrease (not its job)",
			divided: 90,
			prev:    100,
			maxIncr: 10,
			wantOK:  true,
		},
		{
			name:    "max-incr accepts equal reading",
			divided: 100,
			prev:    100,
			maxIncr: 10,
			wantOK:  true,
		},

		// --- both flags combined ---
		{
			name:     "both flags: normal increase accepted",
			divided:  105,
			prev:     100,
			incrOnly: true,
			maxIncr:  10,
			wantOK:   true,
		},
		{
			name:     "both flags: decrease rejected by incr-only",
			divided:  95,
			prev:     100,
			incrOnly: true,
			maxIncr:  10,
			wantOK:   false,
		},
		{
			name:     "both flags: large increase rejected by max-incr",
			divided:  200,
			prev:     100,
			incrOnly: true,
			maxIncr:  50,
			wantOK:   false,
		},

		// --- neither flag (should never reject) ---
		{
			name:    "no flags: everything accepted",
			divided: 50,
			prev:    100,
			wantOK:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reason := checkReadingFilter(tt.divided, tt.prev, tt.incrOnly, tt.maxIncr)
			gotOK := reason == ""
			if gotOK != tt.wantOK {
				if tt.wantOK {
					t.Errorf("expected reading to be accepted, but got rejected: %s", reason)
				} else {
					t.Errorf("expected reading to be rejected, but it was accepted")
				}
			}
		})
	}
}
