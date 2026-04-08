package main

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"regexp"
	"testing"
)

func TestExtractReading(t *testing.T) {
	defaultMatchRe := regexp.MustCompile(`^000\d+$`)

	tests := []struct {
		name     string
		texts    []string
		matchRe  *regexp.Regexp
		fixRules []ocrFixRule
		want     string
	}{
		{
			name:    "exact match with 000 prefix",
			texts:   []string{"000354225"},
			matchRe: defaultMatchRe,
			want:    "000354225",
		},
		{
			name:    "match with leading whitespace",
			texts:   []string{"  000354225  "},
			matchRe: defaultMatchRe,
			want:    "000354225",
		},
		{
			name:    "prefers 000-prefixed over longer digit string",
			texts:   []string{"9876543210", "000354225"},
			matchRe: defaultMatchRe,
			want:    "000354225",
		},
		{
			name:    "fallback to longest digit string when no match",
			texts:   []string{"12", "12345", "abc"},
			matchRe: defaultMatchRe,
			want:    "12345",
		},
		{
			name:    "no digits at all returns empty",
			texts:   []string{"abc", "def"},
			matchRe: defaultMatchRe,
			want:    "",
		},
		{
			name:    "empty input returns empty",
			texts:   []string{},
			matchRe: defaultMatchRe,
			want:    "",
		},
		{
			name:    "nil fix rules behaves like no fix",
			texts:   []string{"000354225"},
			matchRe: defaultMatchRe,
			want:    "000354225",
		},
		{
			name:    "single fix rule corrects 300 prefix to 000",
			texts:   []string{"300354225"},
			matchRe: defaultMatchRe,
			fixRules: []ocrFixRule{
				{Pattern: regexp.MustCompile(`^300`), Replacement: "000"},
			},
			want: "000354225",
		},
		{
			name:    "fix rule does not affect already correct text",
			texts:   []string{"000354225"},
			matchRe: defaultMatchRe,
			fixRules: []ocrFixRule{
				{Pattern: regexp.MustCompile(`^300`), Replacement: "000"},
			},
			want: "000354225",
		},
		{
			name:    "fix rule applied in fallback path too",
			texts:   []string{"abc", "30099"},
			matchRe: defaultMatchRe,
			fixRules: []ocrFixRule{
				{Pattern: regexp.MustCompile(`^300`), Replacement: "000"},
			},
			want: "00099",
		},
		{
			name:    "multiple fix rules applied in order",
			texts:   []string{"O30354225"},
			matchRe: defaultMatchRe,
			fixRules: []ocrFixRule{
				{Pattern: regexp.MustCompile(`^O`), Replacement: "0"},
				{Pattern: regexp.MustCompile(`^030`), Replacement: "000"},
			},
			want: "000354225",
		},
		{
			name:    "later rule depends on earlier rule output",
			texts:   []string{"X3Y354225"},
			matchRe: defaultMatchRe,
			fixRules: []ocrFixRule{
				{Pattern: regexp.MustCompile(`X`), Replacement: "0"},
				{Pattern: regexp.MustCompile(`Y`), Replacement: "0"},
			},
			want: "030354225",
			// After rules: "030354225" — doesn't match ^000\d+$, falls through to fallback as longest digit-only? No, it has no non-digits after fix.
			// Actually 030354225 is all digits so it's the fallback best.
		},
		{
			name:    "custom match regex",
			texts:   []string{"WM-12345", "67890"},
			matchRe: regexp.MustCompile(`^WM-\d+$`),
			want:    "WM-12345",
		},
		{
			name:     "empty fix rules slice same as nil",
			texts:    []string{"000354225"},
			matchRe:  defaultMatchRe,
			fixRules: []ocrFixRule{},
			want:     "000354225",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractReading(tt.texts, tt.matchRe, tt.fixRules, false)
			if got != tt.want {
				t.Errorf("extractReading() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractReadingMerge(t *testing.T) {
	defaultMatchRe := regexp.MustCompile(`^000\d+$`)

	tests := []struct {
		name     string
		texts    []string
		matchRe  *regexp.Regexp
		fixRules []ocrFixRule
		want     string
	}{
		{
			name:    "merge splits into single reading",
			texts:   []string{"00036", "128"},
			matchRe: regexp.MustCompile(`^\d+$`),
			want:    "00036128",
		},
		{
			name:    "merge with match regex",
			texts:   []string{"000", "354225"},
			matchRe: defaultMatchRe,
			want:    "000354225",
		},
		{
			name:    "merge with fix rules",
			texts:   []string{"O30", "354225"},
			matchRe: defaultMatchRe,
			fixRules: []ocrFixRule{
				{Pattern: regexp.MustCompile(`^O`), Replacement: "0"},
				{Pattern: regexp.MustCompile(`^030`), Replacement: "000"},
			},
			want: "000354225",
		},
		{
			name:    "merge single element unchanged",
			texts:   []string{"000354225"},
			matchRe: defaultMatchRe,
			want:    "000354225",
		},
		{
			name:    "merge trims whitespace from each part",
			texts:   []string{" 000 ", " 354225 "},
			matchRe: defaultMatchRe,
			want:    "000354225",
		},
		{
			name:    "merge empty input returns empty",
			texts:   []string{},
			matchRe: defaultMatchRe,
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractReading(tt.texts, tt.matchRe, tt.fixRules, true)
			if got != tt.want {
				t.Errorf("extractReading(merge=true) = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseOCRFixRules(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantNil   bool
		wantCount int
		text      string
		want      string
	}{
		{
			name:    "empty returns nil",
			input:   "",
			wantNil: true,
		},
		{
			name:      "single rule",
			input:     "^300=000",
			wantCount: 1,
			text:      "300354225",
			want:      "000354225",
		},
		{
			name:      "two comma-separated rules applied in order",
			input:     "^O=0,^030=000",
			wantCount: 2,
			text:      "O30354225",
			want:      "000354225",
		},
		{
			name:      "replacement can be empty",
			input:     "^X=",
			wantCount: 1,
			text:      "X12345",
			want:      "12345",
		},
		{
			name:      "whitespace around rules is trimmed",
			input:     " ^300=000 , ^O=0 ",
			wantCount: 2,
			text:      "300354225",
			want:      "000354225",
		},
		{
			name:      "trailing comma ignored",
			input:     "^300=000,",
			wantCount: 1,
			text:      "300354225",
			want:      "000354225",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules := parseOCRFixRules(tt.input)
			if tt.wantNil {
				if rules != nil {
					t.Fatalf("expected nil, got %+v", rules)
				}
				return
			}
			if len(rules) != tt.wantCount {
				t.Fatalf("expected %d rules, got %d", tt.wantCount, len(rules))
			}
			got := applyFixRules(tt.text, rules)
			if got != tt.want {
				t.Errorf("applyFixRules(%q) = %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}

func TestParseMaskRegions(t *testing.T) {
	tests := []struct {
		name      string
		regions   string
		colors    string
		wantCount int
		wantNil   bool
	}{
		{
			name:    "empty returns nil",
			regions: "",
			wantNil: true,
		},
		{
			name:      "single region default color",
			regions:   "10,20,30,40",
			wantCount: 1,
		},
		{
			name:      "two regions default color",
			regions:   "10,20,30,40,50,60,70,80",
			wantCount: 2,
		},
		{
			name:      "single color applies to all regions",
			regions:   "10,20,30,40,50,60,70,80",
			colors:    "FF0000",
			wantCount: 2,
		},
		{
			name:      "one color per region",
			regions:   "10,20,30,40,50,60,70,80",
			colors:    "FF0000,00FF00",
			wantCount: 2,
		},
		{
			name:      "whitespace trimmed",
			regions:   " 10 , 20 , 30 , 40 ",
			colors:    " 0099CC ",
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			masks := parseMaskRegions(tt.regions, tt.colors)
			if tt.wantNil {
				if masks != nil {
					t.Fatalf("expected nil, got %+v", masks)
				}
				return
			}
			if len(masks) != tt.wantCount {
				t.Fatalf("expected %d masks, got %d", tt.wantCount, len(masks))
			}
		})
	}
}

func TestParseMaskColors(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		numRects int
		wantR    []uint8
	}{
		{
			name:     "empty defaults to black",
			input:    "",
			numRects: 2,
			wantR:    []uint8{0, 0},
		},
		{
			name:     "single color broadcast to all",
			input:    "FF0000",
			numRects: 3,
			wantR:    []uint8{255, 255, 255},
		},
		{
			name:     "per-region colors",
			input:    "FF0000,00FF00",
			numRects: 2,
			wantR:    []uint8{255, 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			colors := parseMaskColors(tt.input, tt.numRects)
			if len(colors) != tt.numRects {
				t.Fatalf("expected %d colors, got %d", tt.numRects, len(colors))
			}
			for i, c := range colors {
				if c.R != tt.wantR[i] {
					t.Errorf("color[%d].R = %d, want %d", i, c.R, tt.wantR[i])
				}
			}
		})
	}
}

func TestParseHexColor(t *testing.T) {
	tests := []struct {
		input string
		want  color.RGBA
	}{
		{"000000", color.RGBA{0, 0, 0, 255}},
		{"FFFFFF", color.RGBA{255, 255, 255, 255}},
		{"0099CC", color.RGBA{0, 153, 204, 255}},
		{"#FF8800", color.RGBA{255, 136, 0, 255}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseHexColor(tt.input)
			if got != tt.want {
				t.Errorf("parseHexColor(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestMaskImage(t *testing.T) {
	// Create a 100x100 white JPEG.
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.RGBA{255, 255, 255, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 100}); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()

	masks := []maskRegion{
		{Rect: image.Rect(10, 10, 20, 20), Color: color.RGBA{255, 0, 0, 255}},
		{Rect: image.Rect(50, 50, 60, 60), Color: color.RGBA{0, 0, 255, 255}},
	}

	result, err := maskImage(data, masks)
	if err != nil {
		t.Fatal(err)
	}

	masked, _, err := image.Decode(bytes.NewReader(result))
	if err != nil {
		t.Fatal(err)
	}

	// Check that the masked region at (15,15) is approximately red.
	r, _, _, _ := masked.At(15, 15).RGBA()
	if r>>8 < 200 {
		t.Errorf("expected red at (15,15), got R=%d", r>>8)
	}

	// Check that the masked region at (55,55) is approximately blue.
	_, _, b, _ := masked.At(55, 55).RGBA()
	if b>>8 < 200 {
		t.Errorf("expected blue at (55,55), got B=%d", b>>8)
	}

	// Check that an unmasked region at (0,0) is still approximately white.
	ur, ug, ub, _ := masked.At(0, 0).RGBA()
	if ur>>8 < 200 || ug>>8 < 200 || ub>>8 < 200 {
		t.Errorf("expected white at (0,0), got R=%d G=%d B=%d", ur>>8, ug>>8, ub>>8)
	}
}