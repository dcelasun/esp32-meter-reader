package main

import (
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
			got := extractReading(tt.texts, tt.matchRe, tt.fixRules)
			if got != tt.want {
				t.Errorf("extractReading() = %q, want %q", got, tt.want)
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