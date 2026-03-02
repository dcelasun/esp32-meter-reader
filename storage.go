package main

import (
	"encoding/csv"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var csvMu sync.Mutex

func appendCSV(csvPath, imagePath, reading string) error {
	csvMu.Lock()
	defer csvMu.Unlock()

	f, err := os.OpenFile(csvPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if err := w.Write([]string{imagePath, reading, time.Now().UTC().Format(time.RFC3339)}); err != nil {
		return err
	}
	w.Flush()
	return w.Error()
}

// storeImages saves the original image and, if processing was applied, the
// processed image (cropped, masked, or both) to disk.
// Returns the relative path of the original image, or empty string if storage is disabled.
func storeImages(imageData, processedData []byte, cropped, masked bool) string {
	if storagePath == "" {
		return ""
	}

	now := time.Now().UTC()
	relDir := filepath.Join(
		now.Format("2006"),
		now.Format("01"),
		now.Format("02"),
	)
	baseName := now.Format("15-04-05")
	relPath := filepath.Join(relDir, baseName+".jpg")
	fullPath := filepath.Join(storagePath, relPath)

	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		log.Printf("storage mkdir error: %v", err)
		return ""
	}

	if err := os.WriteFile(fullPath, imageData, 0644); err != nil {
		log.Printf("storage write error: %v", err)
		return ""
	}

	if cropped || masked {
		var suffix string
		switch {
		case cropped && masked:
			suffix = "_cropped_masked"
		case cropped:
			suffix = "_cropped"
		case masked:
			suffix = "_masked"
		}
		procPath := filepath.Join(storagePath, relDir, baseName+suffix+".jpg")
		if err := os.WriteFile(procPath, processedData, 0644); err != nil {
			log.Printf("storage write %s error: %v", suffix[1:], err)
		}
	}

	return relPath
}

// storeReading appends a row to readings.csv.
func storeReading(imagePath, reading string) {
	if storagePath == "" || imagePath == "" || reading == "" {
		return
	}

	csvPath := filepath.Join(storagePath, "readings.csv")
	if err := appendCSV(csvPath, imagePath, reading); err != nil {
		log.Printf("csv append error: %v", err)
	}
}