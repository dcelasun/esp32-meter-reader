package main

import "github.com/prometheus/client_golang/prometheus"

var (
	metricMeterReading = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "meter_reading",
		Help: "Last detected water meter reading (raw integer).",
	})
	metricBatteryLevel = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "meter_battery_level_percent",
		Help: "ESP32 battery level (0-100).",
	})
	metricBatteryVoltage = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "meter_battery_voltage_millivolts",
		Help: "ESP32 battery voltage in millivolts.",
	})
	metricOCRDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "meter_ocr_duration_seconds",
		Help:    "Time taken for OCR processing.",
		Buckets: prometheus.DefBuckets,
	})
	metricOCRErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "meter_ocr_errors_total",
		Help: "Total OCR errors.",
	})
)

func init() {
	prometheus.MustRegister(
		metricMeterReading,
		metricBatteryLevel,
		metricBatteryVoltage,
		metricOCRDuration,
		metricOCRErrors,
	)
}