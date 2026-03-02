package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
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

type maskRegion struct {
	Rect  image.Rectangle
	Color color.RGBA
}

func parseMaskRegions(regions, colors string) []maskRegion {
	regions = strings.TrimSpace(regions)
	if regions == "" {
		return nil
	}

	coords := strings.Split(regions, ",")
	if len(coords)%4 != 0 {
		log.Fatalf("ocr-mask-regions: must have groups of 4 coordinates (x1,y1,x2,y2), got %d values", len(coords))
	}

	vals := make([]int, len(coords))
	for i, c := range coords {
		v, err := strconv.Atoi(strings.TrimSpace(c))
		if err != nil {
			log.Fatalf("ocr-mask-regions: invalid integer %q: %v", c, err)
		}
		vals[i] = v
	}

	numRects := len(vals) / 4
	rectColors := parseMaskColors(colors, numRects)

	masks := make([]maskRegion, numRects)
	for i := 0; i < numRects; i++ {
		x0, y0 := vals[i*4], vals[i*4+1]
		x1, y1 := vals[i*4+2], vals[i*4+3]
		if x0 >= x1 || y0 >= y1 {
			log.Fatalf("ocr-mask-regions: invalid rectangle #%d (x0,y0 must be < x1,y1): %d,%d,%d,%d", i+1, x0, y0, x1, y1)
		}
		masks[i] = maskRegion{
			Rect:  image.Rect(x0, y0, x1, y1),
			Color: rectColors[i],
		}
	}
	return masks
}

func parseMaskColors(s string, numRects int) []color.RGBA {
	s = strings.TrimSpace(s)
	defaultColor := color.RGBA{0, 0, 0, 255}

	if s == "" {
		colors := make([]color.RGBA, numRects)
		for i := range colors {
			colors[i] = defaultColor
		}
		return colors
	}

	parts := strings.Split(s, ",")
	parsed := make([]color.RGBA, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		parsed = append(parsed, parseHexColor(p))
	}

	if len(parsed) == 1 {
		colors := make([]color.RGBA, numRects)
		for i := range colors {
			colors[i] = parsed[0]
		}
		return colors
	}

	if len(parsed) != numRects {
		log.Fatalf("ocr-mask-colors: must specify 1 color or exactly %d (one per rectangle), got %d", numRects, len(parsed))
	}
	return parsed
}

func parseHexColor(s string) color.RGBA {
	s = strings.TrimPrefix(s, "#")
	if len(s) != 6 {
		log.Fatalf("ocr-mask-colors: invalid hex color %q (must be 6 hex digits like 0099CC)", s)
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		log.Fatalf("ocr-mask-colors: invalid hex color %q: %v", s, err)
	}
	return color.RGBA{R: b[0], G: b[1], B: b[2], A: 255}
}

func maskImage(data []byte, masks []maskRegion) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}

	bounds := img.Bounds()
	dst := image.NewRGBA(bounds)
	draw.Draw(dst, bounds, img, bounds.Min, draw.Src)

	for _, m := range masks {
		r := m.Rect.Intersect(bounds)
		if r.Empty() {
			continue
		}
		draw.Draw(dst, r, &image.Uniform{m.Color}, image.Point{}, draw.Src)
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 95}); err != nil {
		return nil, fmt.Errorf("encode masked image: %w", err)
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

func parseOCRFixRules(s string) []ocrFixRule {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	rules := make([]ocrFixRule, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		idx := strings.Index(part, "=")
		if idx < 0 {
			log.Fatalf("ocr-fix-regex: each rule must be pattern=replacement, got: %q", part)
		}
		pattern := part[:idx]
		replacement := part[idx+1:]
		re, err := regexp.Compile(pattern)
		if err != nil {
			log.Fatalf("ocr-fix-regex: invalid pattern %q: %v", pattern, err)
		}
		rules = append(rules, ocrFixRule{Pattern: re, Replacement: replacement})
	}
	return rules
}

func applyFixRules(s string, rules []ocrFixRule) string {
	for _, r := range rules {
		s = r.Pattern.ReplaceAllString(s, r.Replacement)
	}
	return s
}

func extractReading(texts []string, matchRe *regexp.Regexp, fixRules []ocrFixRule) string {
	// First pass: apply fix rules, then match.
	for _, t := range texts {
		t = applyFixRules(strings.TrimSpace(t), fixRules)
		if matchRe.MatchString(t) {
			return t
		}
	}
	// Fallback: longest all-digit string.
	digitsOnly := regexp.MustCompile(`^\d+$`)
	best := ""
	for _, t := range texts {
		t = applyFixRules(strings.TrimSpace(t), fixRules)
		if digitsOnly.MatchString(t) && len(t) > len(best) {
			best = t
		}
	}
	return best
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