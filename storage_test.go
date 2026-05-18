package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLastStoredReading(t *testing.T) {
	tests := []struct {
		name    string
		csv     string // file contents; empty string = don't create file
		noFile  bool
		divisor float64
		wantVal float64
		wantOK  bool
	}{
		{
			name:   "file does not exist",
			noFile: true,
			wantOK: false,
		},
		{
			name:   "empty file",
			csv:    "",
			wantOK: false,
		},
		{
			name:    "single valid row 4-column",
			csv:     "img.jpg,00036596,2026-05-07T14:50:45Z,1\n",
			divisor: 1000,
			wantVal: 36.596,
			wantOK:  true,
		},
		{
			name:    "multiple rows returns last valid",
			csv:     "a.jpg,10000,2026-01-01T00:00:00Z,1\nb.jpg,20000,2026-01-02T00:00:00Z,1\n",
			divisor: 1000,
			wantVal: 20,
			wantOK:  true,
		},
		{
			name:    "last row invalid skipped",
			csv:     "a.jpg,10000,2026-01-01T00:00:00Z,1\nb.jpg,20000,2026-01-02T00:00:00Z,0\n",
			divisor: 1000,
			wantVal: 10,
			wantOK:  true,
		},
		{
			name:    "all rows invalid",
			csv:     "a.jpg,10000,2026-01-01T00:00:00Z,0\nb.jpg,20000,2026-01-02T00:00:00Z,0\n",
			divisor: 1000,
			wantOK:  false,
		},
		{
			name:    "old 3-column format assumed valid",
			csv:     "img.jpg,00036596,2026-05-07T14:50:45Z\n",
			divisor: 1000,
			wantVal: 36.596,
			wantOK:  true,
		},
		{
			name:    "mixed old and new format",
			csv:     "a.jpg,10000,2026-01-01T00:00:00Z\nb.jpg,20000,2026-01-02T00:00:00Z,1\nc.jpg,30000,2026-01-03T00:00:00Z,0\n",
			divisor: 1000,
			wantVal: 20,
			wantOK:  true,
		},
		{
			name:    "unparseable reading skipped",
			csv:     "a.jpg,10000,2026-01-01T00:00:00Z,1\nb.jpg,notanumber,2026-01-02T00:00:00Z,1\n",
			divisor: 1000,
			wantVal: 10,
			wantOK:  true,
		},
		{
			name:    "divisor applied",
			csv:     "img.jpg,5000,2026-01-01T00:00:00Z,1\n",
			divisor: 100,
			wantVal: 50,
			wantOK:  true,
		},
		{
			name:    "row with too few columns skipped",
			csv:     "onlyonefield\na.jpg,10000,2026-01-01T00:00:00Z,1\n",
			divisor: 1000,
			wantVal: 10,
			wantOK:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			csvPath := filepath.Join(dir, "readings.csv")

			if !tt.noFile {
				if err := os.WriteFile(csvPath, []byte(tt.csv), 0644); err != nil {
					t.Fatal(err)
				}
			}

			divisor := tt.divisor
			if divisor == 0 {
				divisor = 1
			}

			gotVal, gotOK := lastStoredReading(csvPath, divisor)
			if gotOK != tt.wantOK {
				t.Fatalf("ok = %v, want %v", gotOK, tt.wantOK)
			}
			if gotOK && gotVal != tt.wantVal {
				t.Errorf("val = %f, want %f", gotVal, tt.wantVal)
			}
		})
	}
}
