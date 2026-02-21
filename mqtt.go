package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

var mqttClient mqtt.Client

type haDiscoveryConfig struct {
	Name              string   `json:"name"`
	UniqueID          string   `json:"unique_id"`
	StateTopic        string   `json:"state_topic"`
	ValueTemplate     string   `json:"value_template"`
	AvailabilityTopic string   `json:"availability_topic"`
	Device            haDevice `json:"device"`
	DeviceClass       string   `json:"device_class,omitempty"`
	StateClass        string   `json:"state_class,omitempty"`
	UnitOfMeasurement string   `json:"unit_of_measurement,omitempty"`
	Icon              string   `json:"icon,omitempty"`
}

type haDevice struct {
	Identifiers  []string `json:"identifiers"`
	Name         string   `json:"name"`
	Manufacturer string   `json:"manufacturer,omitempty"`
	Model        string   `json:"model,omitempty"`
}

type statePayload struct {
	Reading    float64 `json:"reading"`
	BatLevel   int     `json:"bat_level,omitempty"`
	BatVoltage int     `json:"bat_voltage,omitempty"`
}

func stateTopic() string {
	return fmt.Sprintf("%s/%s/state", mqttTopicPrefix, mqttDeviceID)
}

func availabilityTopic() string {
	return fmt.Sprintf("%s/%s/availability", mqttTopicPrefix, mqttDeviceID)
}

func initMQTT() {
	opts := mqtt.NewClientOptions().
		AddBroker(mqttBroker).
		SetClientID(fmt.Sprintf("meter-reader-%s", mqttDeviceID)).
		SetKeepAlive(60*time.Second).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetConnectRetryInterval(10*time.Second).
		SetWill(availabilityTopic(), "offline", 1, true).
		SetOnConnectHandler(onMQTTConnect)

	if mqttUser != "" {
		opts.SetUsername(mqttUser)
	}
	if mqttPassword != "" {
		opts.SetPassword(mqttPassword)
	}

	mqttClient = mqtt.NewClient(opts)
	if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
		log.Fatalf("MQTT connect error: %v", token.Error())
	}
}

func onMQTTConnect(c mqtt.Client) {
	log.Printf("MQTT connected, publishing discovery configs")
	publishDiscovery()
	publish(availabilityTopic(), "online", true)
}

func publishDiscovery() {
	device := haDevice{
		Identifiers:  []string{mqttDeviceID},
		Name:         "Water Meter",
		Manufacturer: mqttDeviceManufacturer,
		Model:        mqttDeviceModel,
	}

	sensors := []haDiscoveryConfig{
		{
			Name:              "Water Meter Reading",
			UniqueID:          mqttDeviceID + "_reading",
			StateTopic:        stateTopic(),
			ValueTemplate:     "{{ value_json.reading }}",
			AvailabilityTopic: availabilityTopic(),
			Device:            device,
			DeviceClass:       "water",
			StateClass:        "total_increasing",
			UnitOfMeasurement: "m³",
			Icon:              "mdi:water",
		},
		{
			Name:              "Water Meter Battery",
			UniqueID:          mqttDeviceID + "_battery",
			StateTopic:        stateTopic(),
			ValueTemplate:     "{{ value_json.bat_level }}",
			AvailabilityTopic: availabilityTopic(),
			Device:            device,
			DeviceClass:       "battery",
			StateClass:        "measurement",
			UnitOfMeasurement: "%",
		},
		{
			Name:              "Water Meter Battery Voltage",
			UniqueID:          mqttDeviceID + "_battery_voltage",
			StateTopic:        stateTopic(),
			ValueTemplate:     "{{ value_json.bat_voltage }}",
			AvailabilityTopic: availabilityTopic(),
			Device:            device,
			DeviceClass:       "voltage",
			StateClass:        "measurement",
			UnitOfMeasurement: "mV",
		},
	}

	for _, s := range sensors {
		topic := fmt.Sprintf("homeassistant/sensor/%s/%s/config", mqttDeviceID, s.UniqueID)
		payload, err := json.Marshal(s)
		if err != nil {
			log.Printf("MQTT discovery marshal error: %v", err)
			continue
		}
		publish(topic, string(payload), true)
	}
}

func publishReading(reading float64, batLevel, batVoltage int) {
	payload := statePayload{
		Reading:    reading,
		BatLevel:   batLevel,
		BatVoltage: batVoltage,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("MQTT state marshal error: %v", err)
		return
	}

	publish(stateTopic(), string(data), true)
}

func publish(topic, payload string, retain bool) {
	if mqttClient == nil || !mqttClient.IsConnected() {
		log.Printf("MQTT not connected, skipping publish to %s", topic)
		return
	}
	token := mqttClient.Publish(topic, 1, retain, payload)
	token.Wait()
	if token.Error() != nil {
		log.Printf("MQTT publish error (%s): %v", topic, token.Error())
	}
}