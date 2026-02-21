/**
 * @file m5stack_timer_camera_x.ino
 * @author Can Celasun
 * @brief TimerCamera-X deep sleep & HTTP Post
 * @version 0.1
 * @date 2026-02-20
 *
 *
 * @Hardwares: TimerCamera-X
 * @Platform Version: Arduino M5Stack Board Manager v2.0.9
 * @Dependent Library:
 * TimerCam-arduino: https://github.com/m5stack/TimerCam-arduino
 * ArduinoHttpClient: https://github.com/arduino-libraries/ArduinoHttpClient
 * Based on: https://github.com/m5stack/TimerCam-arduino/blob/ca24786e3cecdad060d86d6eb1cbc299685591b9/examples/wakeup/wakeup.ino
 * Based on: https://github.com/m5stack/TimerCam-arduino/blob/ca24786e3cecdad060d86d6eb1cbc299685591b9/examples/http_post/http_post.ino
 * Based on: https://github.com/m5stack/M5Unit-FlashLight/blob/008853f9e9832db20ba95c8b58ec68f327f61e36/SetFlashTimeAndBrightness/SetFlashTimeAndBrightness.ino
 */

#include "M5TimerCAM.h"
#include <WiFi.h>
#include <ArduinoHttpClient.h>

#define FLASH_EN_PIN 4 // SDA port is G4 on the Timer Camera X

#define _WIFI_SSID "My SSID"
#define _WIFI_PASS "MyPassword"

#define _SERVER_HOST "127.0.0.1"
#define _SERVER_PORT 80

#define SLEEP_INTERVAL_SECS 14400

void unit_flash_set_brightness(uint8_t brightness);

void led_breathe(int ms) {
    for (int16_t i = 0; i < 255; i++) {
        TimerCAM.Power.setLed(i);
        vTaskDelay(pdMS_TO_TICKS(ms));
    }

    for (int16_t i = 255; i >= 0; i--) {
        TimerCAM.Power.setLed(i);
        vTaskDelay(pdMS_TO_TICKS(ms));
    }
}

// See https://github.com/m5stack/TimerCam-arduino/issues/16
void setup() {
    TimerCAM.begin(true);
    Serial.println("Waking up");
    led_breathe(10);

    int batVoltage = TimerCAM.Power.getBatteryVoltage();
    int batLevel   = TimerCAM.Power.getBatteryLevel();

    Serial.printf("Bat Voltage: %dmv\r\n", batVoltage);
    Serial.printf("Bat Level: %d%%\r\n", batLevel);

    pinMode(FLASH_EN_PIN, OUTPUT);

    if (!configureCamera()) {
        Serial.println("Camera configuration failed, halting");
        return;
    }

    WiFiClient wifi = connectToWiFi();
    if (WiFi.status() != WL_CONNECTED) {
        Serial.println("WiFi connection failed, halting");
        return;
    }

    if (!sendImage(wifi, batLevel, batVoltage)) {
        Serial.println("Image upload failed, halting");
        return;
    }

    Serial.println("Going to sleep now");
    TimerCAM.Power.timerSleep(SLEEP_INTERVAL_SECS);
}

bool configureCamera() {
    if (!TimerCAM.Camera.begin()) {
        Serial.println("Camera Init Fail");
        return false;
    }
    Serial.println("Camera init success");

    TimerCAM.Camera.sensor->set_pixformat(TimerCAM.Camera.sensor, PIXFORMAT_JPEG);
    // 2MP Sensor
    //TimerCAM.Camera.sensor->set_framesize(TimerCAM.Camera.sensor, FRAMESIZE_UXGA);
    // 3MP Sensor
    TimerCAM.Camera.sensor->set_framesize(TimerCAM.Camera.sensor, FRAMESIZE_QXGA);

    TimerCAM.Camera.sensor->set_vflip(TimerCAM.Camera.sensor, 1);
    TimerCAM.Camera.sensor->set_hmirror(TimerCAM.Camera.sensor, 0);

    TimerCAM.Camera.free();
    return true;
}

WiFiClient connectToWiFi() {
    WiFiClient wifi;
    WiFi.mode(WIFI_STA);
    WiFi.begin(_WIFI_SSID, _WIFI_PASS);
    WiFi.setSleep(false);
    Serial.println("");
    Serial.print("Connecting to ");
    Serial.println(_WIFI_SSID);

    int attempts = 0;
    while (WiFi.status() != WL_CONNECTED) {
        if (attempts >= 20) {
            Serial.println("\nFailed to connect after 20 attempts");
            return WiFiClient();
        }
        delay(1000);
        Serial.print(".");
        attempts++;
    }

    Serial.println("");
    Serial.print("Connected to ");
    Serial.println(_WIFI_SSID);
    Serial.print("IP address: ");
    Serial.println(WiFi.localIP());
    return wifi;
}

bool sendImage(WiFiClient wifi, int batLevel, int batVoltage) {
    unit_flash_set_brightness(9);
    delay(200);
    if (!TimerCAM.Camera.get()) {
        unit_flash_set_brightness(0);
        Serial.println("Could not get camera");
        return false;
    }
    unit_flash_set_brightness(0);

    HttpClient client = HttpClient(wifi, _SERVER_HOST, _SERVER_PORT);

    Serial.println("making POST request");

    String path = "/ocr?bat_level=" + String(batLevel) + "&bat_voltage=" + String(batVoltage);
    String contentType = "image/jpeg";
    client.post(path.c_str(), contentType.c_str(), TimerCAM.Camera.fb->len, TimerCAM.Camera.fb->buf);

    int statusCode  = client.responseStatusCode();
    String response = client.responseBody();

    Serial.print("Status code: ");
    Serial.println(statusCode);
    Serial.print("Response: ");
    Serial.println(response);

    TimerCAM.Camera.free();

    if (statusCode != 202) {
        Serial.println("Upload failed");
        return false;
    }

    return true;
}

// 0: Flashlight off
// 1: 100% brightness + 220ms
// 2: 90% brightness + 220ms
// 3: 80% brightness + 220ms
// 4: 70% brightness + 220ms
// 5: 60% brightness + 220ms
// 6: 50% brightness + 220ms
// 7: 40% brightness + 220ms
// 8: 30% brightness + 220ms
// 9: 100% brightness + 1.3s
// 10: 90% brightness + 1.3s
// 11: 80% brightness + 1.3s
// 12: 70% brightness + 1.3s
// 13: 60% brightness + 1.3s
// 14: 50% brightness + 1.3s
// 15: 40% brightness + 1.3s
// 16: 30% brightness + 1.3s
void unit_flash_set_brightness(uint8_t brightness) {
    if ((brightness >= 1) && (brightness <= 16)) {
        for (int i = 0; i < brightness; i++) {
            digitalWrite(FLASH_EN_PIN, LOW);
            delayMicroseconds(4);
            digitalWrite(FLASH_EN_PIN, HIGH);
            delayMicroseconds(4);
        }
    } else {
        digitalWrite(FLASH_EN_PIN, LOW);
    }
}

void loop() {

}