package main

import (
	"regexp"
	"testing"
)

func TestExtractReading(t *testing.T) {
	defaultMatchRe := regexp.MustCompile(`^000\d+$`)

	tests := []struct {
		name    string
		texts   []string
		matchRe *regexp.Regexp
		fixRule *ocrFixRule
		want    string
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
			name:    "mixed text ignored in first pass",
			texts:   []string{"m3/h", "000354225", "reading"},
			matchRe: defaultMatchRe,
			want:    "000354225",
		},
		{
			name:    "fix rule corrects 300 prefix to 000",
			texts:   []string{"300354225"},
			matchRe: defaultMatchRe,
			fixRule: &ocrFixRule{
				Pattern:     regexp.MustCompile(`^300`),
				Replacement: "000",
			},
			want: "000354225",
		},
		{
			name:    "fix rule does not affect already correct text",
			texts:   []string{"000354225"},
			matchRe: defaultMatchRe,
			fixRule: &ocrFixRule{
				Pattern:     regexp.MustCompile(`^300`),
				Replacement: "000",
			},
			want: "000354225",
		},
		{
			name:    "fix rule applied in fallback path too",
			texts:   []string{"abc", "30099"},
			matchRe: defaultMatchRe,
			fixRule: &ocrFixRule{
				Pattern:     regexp.MustCompile(`^300`),
				Replacement: "000",
			},
			want: "00099",
		},
		{
			name:    "custom match regex",
			texts:   []string{"WM-12345", "67890"},
			matchRe: regexp.MustCompile(`^WM-\d+$`),
			want:    "WM-12345",
		},
		{
			name:    "fix rule with capture group",
			texts:   []string{"O00354225"},
			matchRe: defaultMatchRe,
			fixRule: &ocrFixRule{
				Pattern:     regexp.MustCompile(`^O`),
				Replacement: "0",
			},
			want: "000354225",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractReading(tt.texts, tt.matchRe, tt.fixRule)
			if got != tt.want {
				t.Errorf("extractReading() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseOCRFixRegex(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantNil bool
		text    string
		want    string
	}{
		{
			name:    "empty returns nil",
			input:   "",
			wantNil: true,
		},
		{
			name:  "simple prefix replacement",
			input: "^300=000",
			text:  "300354225",
			want:  "000354225",
		},
		{
			name:  "replacement can be empty",
			input: "^X=",
			text:  "X12345",
			want:  "12345",
		},
		{
			name:  "equals sign in replacement",
			input: "^foo=bar=baz",
			text:  "fooQux",
			want:  "bar=bazQux",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := parseOCRFixRegex(tt.input)
			if tt.wantNil {
				if rule != nil {
					t.Fatalf("expected nil, got %+v", rule)
				}
				return
			}
			if rule == nil {
				t.Fatal("expected non-nil rule")
			}
			got := rule.Pattern.ReplaceAllString(tt.text, rule.Replacement)
			if got != tt.want {
				t.Errorf("ReplaceAllString(%q) = %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}