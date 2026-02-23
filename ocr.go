package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"log"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

type cropConfig struct {
	X0, Y0, X1, Y1 int
}

func parseCropRect(s string) *cropConfig {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	if len(parts) != 4 {
		log.Fatalf("crop must be x0,y0,x1,y1 but got: %q", s)
	}
	vals := make([]int, 4)
	for i, p := range parts {
		v, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			log.Fatalf("crop: invalid integer %q: %v", p, err)
		}
		vals[i] = v
	}
	c := &cropConfig{X0: vals[0], Y0: vals[1], X1: vals[2], Y1: vals[3]}
	if c.X0 >= c.X1 || c.Y0 >= c.Y1 {
		log.Fatalf("crop: invalid rectangle (x0,y0 must be < x1,y1): %v", c)
	}
	return c
}

func cropImage(data []byte, c *cropConfig) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}

	bounds := img.Bounds()
	x0 := max(c.X0, bounds.Min.X)
	y0 := max(c.Y0, bounds.Min.Y)
	x1 := min(c.X1, bounds.Max.X)
	y1 := min(c.Y1, bounds.Max.Y)

	if x0 >= x1 || y0 >= y1 {
		return nil, fmt.Errorf("crop rect outside image bounds")
	}

	type subImager interface {
		SubImage(r image.Rectangle) image.Image
	}

	si, ok := img.(subImager)
	if !ok {
		return nil, fmt.Errorf("image type does not support cropping")
	}

	cropped := si.SubImage(image.Rect(x0, y0, x1, y1))

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, cropped, &jpeg.Options{Quality: 95}); err != nil {
		return nil, fmt.Errorf("encode cropped image: %w", err)
	}

	return buf.Bytes(), nil
}

type ocrOutput struct {
	Texts  []string  `json:"texts"`
	Scores []float64 `json:"scores"`
}

type ocrFixRule struct {
	Pattern     *regexp.Regexp
	Replacement string
}

func parseOCRFixRegex(s string) *ocrFixRule {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	idx := strings.Index(s, "=")
	if idx < 0 {
		log.Fatalf("ocr-fix-regex must be pattern=replacement, got: %q", s)
	}
	pattern := s[:idx]
	replacement := s[idx+1:]
	re, err := regexp.Compile(pattern)
	if err != nil {
		log.Fatalf("ocr-fix-regex: invalid pattern %q: %v", pattern, err)
	}
	return &ocrFixRule{Pattern: re, Replacement: replacement}
}

func extractReading(texts []string, matchRe *regexp.Regexp, fixRule *ocrFixRule) string {
	for _, t := range texts {
		t = strings.TrimSpace(t)
		if fixRule != nil {
			t = fixRule.Pattern.ReplaceAllString(t, fixRule.Replacement)
		}
		if matchRe.MatchString(t) {
			return t
		}
	}
	return ""
}

func runOCR(imagePath string) (*ocrOutput, error) {
	cmd := exec.Command(pythonBin, ocrScript, imagePath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, stderr.String())
	}

	var out ocrOutput
	if err := json.Unmarshal(output, &out); err != nil {
		return nil, fmt.Errorf("parse output: %w (raw: %s)", err, string(output))
	}

	return &out, nil
}