package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type ocrResult struct {
	RawTexts []string  `json:"raw_texts"`
	Scores   []float64 `json:"scores"`
	Reading  string    `json:"reading"`
}

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

	if len(imageData) == 0 {
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}

	var croppedData []byte
	ocrData := imageData
	if cropRect != nil {
		cropped, err := cropImage(imageData, cropRect)
		if err != nil {
			log.Printf("crop error: %v, using original image", err)
		} else {
			croppedData = cropped
			ocrData = cropped
		}
	}

	tmpDir, err := os.MkdirTemp("", "ocr-")
	if err != nil {
		http.Error(w, "failed to create temp dir: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tmpDir)

	tmpFile := filepath.Join(tmpDir, "image.jpg")
	if err := os.WriteFile(tmpFile, ocrData, 0644); err != nil {
		http.Error(w, "failed to write temp file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	start := time.Now()
	ocrOut, err := runOCR(tmpFile)
	elapsed := time.Since(start)
	metricOCRDuration.Observe(elapsed.Seconds())

	if err != nil {
		metricOCRErrors.Inc()
		log.Printf("ocr error: %v", err)
		http.Error(w, fmt.Sprintf("OCR failed: %v", err), http.StatusInternalServerError)
		return
	}

	reading := extractReading(ocrOut.Texts)

	if reading != "" {
		if val, err := strconv.ParseFloat(reading, 64); err == nil {
			metricMeterReading.Set(val)
		}
	}

	storeReading(imageData, croppedData, reading)

	if mqttBroker != "" && reading != "" {
		if val, err := strconv.ParseFloat(reading, 64); err == nil {
			publishReading(val/meterDivisor, batLevel, batVoltage)
		}
	}

	result := ocrResult{
		RawTexts: ocrOut.Texts,
		Scores:   ocrOut.Scores,
		Reading:  reading,
	}

	log.Printf("OCR completed in %s: reading=%s texts=%v", elapsed, reading, ocrOut.Texts)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}