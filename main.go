package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"regexp"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	cli "github.com/urfave/cli/v3"
)

// Config — populated by urfave/cli before the action runs.
var (
	listenAddr  string
	ocrScript   string
	pythonBin   string
	storagePath string
	cropRect    *cropConfig
	ocrMatchRe  *regexp.Regexp
	ocrFixRules []ocrFixRule
	ocrMasks    []maskRegion

	mqttBroker             string
	mqttUser               string
	mqttPassword           string
	mqttTopicPrefix        string
	mqttDeviceID           string
	mqttDeviceManufacturer string
	mqttDeviceModel        string
	meterDivisor           float64
	ocrIncrOnly            bool
	ocrMergeTexts          bool
)

func main() {
	cmd := &cli.Command{
		Name:  "meter-reader",
		Usage: "Water meter OCR service with Prometheus metrics and MQTT/Home Assistant support",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "listen-addr",
				Value:   ":8080",
				Usage:   "Address and port for the HTTP server to listen on",
				Sources: cli.EnvVars("LISTEN_ADDR"),
			},
			&cli.StringFlag{
				Name:    "ocr-script",
				Value:   "ocr.py",
				Usage:   "Path to the PaddleOCR Python script",
				Sources: cli.EnvVars("OCR_SCRIPT"),
			},
			&cli.StringFlag{
				Name:    "python-bin",
				Value:   "/usr/bin/python3",
				Usage:   "Path to the Python binary used to run the OCR script",
				Sources: cli.EnvVars("PYTHON_BIN"),
			},
			&cli.StringFlag{
				Name:    "storage-path",
				Usage:   "Directory to store captured images and readings.csv (disabled if empty)",
				Sources: cli.EnvVars("STORAGE_PATH"),
			},
			&cli.StringFlag{
				Name:    "crop",
				Usage:   "Crop rectangle applied to images before OCR, as x0,y0,x1,y1 (disabled if empty)",
				Sources: cli.EnvVars("CROP"),
			},
			&cli.StringFlag{
				Name:    "ocr-match-regex",
				Value:   `^000\d+$`,
				Usage:   "Regex to identify the meter reading from OCR text results",
				Sources: cli.EnvVars("OCR_MATCH_REGEX"),
			},
			&cli.StringFlag{
				Name:    "ocr-fix-regex",
				Usage:   "Comma-separated list of regex substitutions applied to OCR text before matching, each as pattern=replacement (e.g. ^O=0,^030=000)",
				Sources: cli.EnvVars("OCR_FIX_REGEX"),
			},
			&cli.BoolFlag{
				Name:    "ocr-merge-texts",
				Usage:   "Concatenate all OCR text results into a single string before applying fix/match regexes (useful when readings are split across multiple detections)",
				Sources: cli.EnvVars("OCR_MERGE_TEXTS"),
			},
			&cli.StringFlag{
				Name:    "ocr-mask-regions",
				Usage:   "Comma-separated rectangle coordinates to mask before OCR, as x1,y1,x2,y2[,x3,y3,x4,y4,...] (applied after crop)",
				Sources: cli.EnvVars("OCR_MASK_REGIONS"),
			},
			&cli.StringFlag{
				Name:    "ocr-mask-colors",
				Usage:   "Comma-separated hex colors for mask regions (e.g. 0099CC). One color applies to all regions; otherwise must match the number of regions",
				Sources: cli.EnvVars("OCR_MASK_COLORS"),
			},
			&cli.StringFlag{
				Name:    "mqtt-broker",
				Usage:   "MQTT broker URL, e.g. tcp://192.168.1.100:1883 (disabled if empty)",
				Sources: cli.EnvVars("MQTT_BROKER"),
			},
			&cli.StringFlag{
				Name:    "mqtt-user",
				Usage:   "MQTT broker username",
				Sources: cli.EnvVars("MQTT_USER"),
			},
			&cli.StringFlag{
				Name:    "mqtt-password",
				Usage:   "MQTT broker password",
				Sources: cli.EnvVars("MQTT_PASSWORD"),
			},
			&cli.StringFlag{
				Name:    "mqtt-topic-prefix",
				Value:   "meter-reader",
				Usage:   "Prefix for MQTT topics",
				Sources: cli.EnvVars("MQTT_TOPIC_PREFIX"),
			},
			&cli.StringFlag{
				Name:    "mqtt-device-id",
				Value:   "water_meter",
				Usage:   "Device identifier used in MQTT topics and Home Assistant discovery",
				Sources: cli.EnvVars("MQTT_DEVICE_ID"),
			},
			&cli.StringFlag{
				Name:    "mqtt-device-manufacturer",
				Value:   "Generic",
				Usage:   "Device manufacturer reported in Home Assistant discovery",
				Sources: cli.EnvVars("MQTT_DEVICE_MANUFACTURER"),
			},
			&cli.StringFlag{
				Name:    "mqtt-device-model",
				Value:   "Generic",
				Usage:   "Device model reported in Home Assistant discovery",
				Sources: cli.EnvVars("MQTT_DEVICE_MODEL"),
			},
			&cli.FloatFlag{
				Name:    "meter-divisor",
				Value:   1000,
				Usage:   "Divisor to convert the raw OCR reading to m³ (e.g. 000354225 / 1000 = 354.225)",
				Sources: cli.EnvVars("METER_DIVISOR"),
			},
			&cli.BoolFlag{
				Name:    "ocr-incr-only",
				Usage:   "Only publish readings that are >= the previous reading (after dividing by meter-divisor), discarding likely OCR errors",
				Sources: cli.EnvVars("OCR_INCR_ONLY"),
			},
		},
		Action: run,
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}

func run(_ context.Context, cmd *cli.Command) error {
	listenAddr = cmd.String("listen-addr")
	ocrScript = cmd.String("ocr-script")
	pythonBin = cmd.String("python-bin")
	storagePath = cmd.String("storage-path")
	cropRect = parseCropRect(cmd.String("crop"))
	ocrMatchRe = regexp.MustCompile(cmd.String("ocr-match-regex"))
	ocrFixRules = parseOCRFixRules(cmd.String("ocr-fix-regex"))
	ocrMasks = parseMaskRegions(cmd.String("ocr-mask-regions"), cmd.String("ocr-mask-colors"))

	mqttBroker = cmd.String("mqtt-broker")
	mqttUser = cmd.String("mqtt-user")
	mqttPassword = cmd.String("mqtt-password")
	mqttTopicPrefix = cmd.String("mqtt-topic-prefix")
	mqttDeviceID = cmd.String("mqtt-device-id")
	mqttDeviceManufacturer = cmd.String("mqtt-device-manufacturer")
	mqttDeviceModel = cmd.String("mqtt-device-model")
	meterDivisor = cmd.Float("meter-divisor")
	ocrIncrOnly = cmd.Bool("ocr-incr-only")
	ocrMergeTexts = cmd.Bool("ocr-merge-texts")

	if mqttBroker != "" {
		initMQTT()
	}

	http.HandleFunc("/ocr", handleOCR)
	http.HandleFunc("/health", handleHealth)
	http.Handle("/metrics", promhttp.Handler())

	log.Printf("meter-reader listening on %s", listenAddr)
	if storagePath != "" {
		log.Printf("storage enabled: %s", storagePath)
	}
	if cropRect != nil {
		log.Printf("crop enabled: (%d,%d)-(%d,%d)", cropRect.X0, cropRect.Y0, cropRect.X1, cropRect.Y1)
	}
	if mqttBroker != "" {
		log.Printf("MQTT enabled: %s (device: %s, divisor: %.0f)", mqttBroker, mqttDeviceID, meterDivisor)
	}

	return http.ListenAndServe(listenAddr, nil)
}