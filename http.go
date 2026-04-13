package main

import (
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

var (
	lastReadingMu sync.Mutex
	lastReading   = math.NaN()
)

func handleOCR(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}

	query := r.URL.Query()
	var batLevel, batVoltage int
	if v := query.Get("bat_level"); v != "" {
		if level, err := strconv.Atoi(v); err == nil && level >= 0 && level <= 100 {
			batLevel = level
			metricBatteryLevel.Set(float64(level))
		}
	}
	if v := query.Get("bat_voltage"); v != "" {
		if voltage, err := strconv.Atoi(v); err == nil && voltage > 0 {
			batVoltage = voltage
			metricBatteryVoltage.Set(float64(voltage))
		}
	}

	imageData, err := io.ReadAll(io.LimitReader(r.Body, 10<<20))
	if err != nil {
		http.Error(w, "failed to read body: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	log.Printf("OCR request: image_bytes=%d bat_level=%d bat_voltage=%d", len(imageData), batLevel, batVoltage)

	if len(imageData) == 0 {
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}

	// Respond immediately so the ESP32 can go back to sleep.
	w.WriteHeader(http.StatusAccepted)

	// Process OCR in the background.
	go processOCR(imageData, batLevel, batVoltage)
}

func processOCR(imageData []byte, batLevel, batVoltage int) {
	var cropped, masked bool
	ocrData := imageData
	if cropRect != nil {
		data, err := cropImage(imageData, cropRect)
		if err != nil {
			log.Printf("crop error: %v, using original image", err)
		} else {
			ocrData = data
			cropped = true
		}
	}

	if len(ocrMasks) > 0 {
		data, err := maskImage(ocrData, ocrMasks)
		if err != nil {
			log.Printf("mask error: %v, using unmasked image", err)
		} else {
			ocrData = data
			masked = true
		}
	}

	// Store images to disk before OCR so we have them even if OCR fails.
	imagePath := storeImages(imageData, ocrData, cropped, masked)

	tmpDir, err := os.MkdirTemp("", "ocr-")
	if err != nil {
		log.Printf("failed to create temp dir: %v", err)
		metricOCRErrors.Inc()
		return
	}
	defer os.RemoveAll(tmpDir)

	tmpFile := filepath.Join(tmpDir, "image.jpg")
	if err := os.WriteFile(tmpFile, ocrData, 0644); err != nil {
		log.Printf("failed to write temp file: %v", err)
		metricOCRErrors.Inc()
		return
	}

	start := time.Now()
	ocrOut, err := runOCR(tmpFile)
	elapsed := time.Since(start)
	metricOCRDuration.Observe(elapsed.Seconds())

	if err != nil {
		metricOCRErrors.Inc()
		log.Printf("ocr error: %v", err)
		return
	}

	reading := extractReading(ocrOut.Texts, ocrMatchRe, ocrFixRules, ocrMergeTexts)

	// Append reading to CSV unconditionally so discarded values are still on disk.
	storeReading(imagePath, reading)

	if reading == "" {
		log.Printf("OCR completed in %s: no reading found, texts=%v", elapsed, ocrOut.Texts)
		return
	}

	val, err := strconv.ParseFloat(reading, 64)
	if err != nil {
		log.Printf("OCR completed in %s: invalid reading %q, texts=%v", elapsed, reading, ocrOut.Texts)
		return
	}

	divided := val / meterDivisor

	if ocrIncrOnly || ocrMaxIncr > 0 {
		lastReadingMu.Lock()
		prev := lastReading
		if !math.IsNaN(prev) {
			if ocrIncrOnly && divided < prev {
				lastReadingMu.Unlock()
				log.Printf("OCR incr-only: discarding reading %.3f < previous %.3f", divided, prev)
				return
			}
			if ocrMaxIncr > 0 && divided-prev > ocrMaxIncr {
				lastReadingMu.Unlock()
				log.Printf("OCR max-incr: discarding reading %.3f, increase %.3f > max %.3f (previous %.3f)", divided, divided-prev, ocrMaxIncr, prev)
				return
			}
		}
		lastReading = divided
		lastReadingMu.Unlock()
	}

	metricMeterReading.Set(val)

	if mqttBroker != "" {
		publishReading(divided, batLevel, batVoltage)
	}

	log.Printf("OCR completed in %s: reading=%s (%.3f m³) texts=%v", elapsed, reading, divided, ocrOut.Texts)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}