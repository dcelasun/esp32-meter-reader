/**
 * @file esp32cam_watermeter_upload.ino
 * @brief ESP32-CAM (AI-Thinker) capture and HTTP POST every 10 seconds
 * @Hardware: AI-Thinker ESP32-CAM with OV2640 camera
 * @Board: "AI Thinker ESP32-CAM" in Arduino IDE
 * @Dependent libraries: WiFi (built-in), ArduinoHttpClient
 */

#include "esp_camera.h"
#include <WiFi.h>
#include <ArduinoHttpClient.h>

// ===== Camera pin definitions for AI-Thinker ESP32-CAM =====
#define PWDN_GPIO_NUM     32
#define RESET_GPIO_NUM    -1
#define XCLK_GPIO_NUM      0
#define SIOD_GPIO_NUM     26
#define SIOC_GPIO_NUM     27
#define Y9_GPIO_NUM       35
#define Y8_GPIO_NUM       34
#define Y7_GPIO_NUM       39
#define Y6_GPIO_NUM       36
#define Y5_GPIO_NUM       21
#define Y4_GPIO_NUM       19
#define Y3_GPIO_NUM       18
#define Y2_GPIO_NUM        5
#define VSYNC_GPIO_NUM    25
#define HREF_GPIO_NUM     23
#define PCLK_GPIO_NUM     22

// ===== Flash LED (on-board) =====
#define FLASH_LED_PIN      4

// ===== WiFi credentials =====
#define _WIFI_SSID "My SSID"
#define _WIFI_PASS "MyPassword"

// ===== HTTP server =====
#define _SERVER_HOST "127.0.0.1"
#define _SERVER_PORT 80

// ===== Interval between captures (milliseconds) =====
#define CAPTURE_INTERVAL_MS 14400000  

// ===== Forward declarations =====
bool initCamera();
WiFiClient connectToWiFi();
bool sendImage(WiFiClient wifi);

void setup() {
    Serial.begin(115200);
    Serial.println("Booting...");

    // Flash briefly to indicate start
    pinMode(FLASH_LED_PIN, OUTPUT);
    digitalWrite(FLASH_LED_PIN, HIGH);
    delay(1000);
    digitalWrite(FLASH_LED_PIN, LOW);

    if (!initCamera()) {
        Serial.println("Camera configuration failed, halting");
        while (true) delay(1000);   // stop here
    }

    // Connect to WiFi once at startup
    WiFiClient wifi = connectToWiFi();
    if (WiFi.status() != WL_CONNECTED) {
        Serial.println("WiFi connection failed, halting");
        while (true) delay(1000);
    }

    Serial.println("Setup complete. Entering main loop.");
}

bool initCamera() {
    camera_config_t config;
    config.ledc_channel = LEDC_CHANNEL_0;
    config.ledc_timer   = LEDC_TIMER_0;
    config.pin_d0       = Y2_GPIO_NUM;
    config.pin_d1       = Y3_GPIO_NUM;
    config.pin_d2       = Y4_GPIO_NUM;
    config.pin_d3       = Y5_GPIO_NUM;
    config.pin_d4       = Y6_GPIO_NUM;
    config.pin_d5       = Y7_GPIO_NUM;
    config.pin_d6       = Y8_GPIO_NUM;
    config.pin_d7       = Y9_GPIO_NUM;
    config.pin_xclk     = XCLK_GPIO_NUM;
    config.pin_pclk     = PCLK_GPIO_NUM;
    config.pin_vsync    = VSYNC_GPIO_NUM;
    config.pin_href     = HREF_GPIO_NUM;
    config.pin_sscb_sda = SIOD_GPIO_NUM;
    config.pin_sscb_scl = SIOC_GPIO_NUM;
    config.pin_pwdn     = PWDN_GPIO_NUM;
    config.pin_reset    = RESET_GPIO_NUM;
    config.xclk_freq_hz = 20000000;
    config.pixel_format = PIXFORMAT_JPEG;

    config.frame_size   = FRAMESIZE_SVGA;   // 800x600
    config.jpeg_quality = 12;
    config.fb_count     = 1;

    esp_err_t err = esp_camera_init(&config);
    if (err != ESP_OK) {
        Serial.printf("Camera init failed with error 0x%x\n", err);
        return false;
    }

    sensor_t *s = esp_camera_sensor_get();
    // s->set_vflip(s, 1);
    // s->set_hmirror(s, 0);

    Serial.println("Camera init success");
    return true;
}

WiFiClient connectToWiFi() {
    WiFiClient wifi;
    WiFi.mode(WIFI_STA);
    WiFi.begin(_WIFI_SSID, _WIFI_PASS);
    WiFi.setSleep(false);
    Serial.println();
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

    Serial.println();
    Serial.print("Connected to ");
    Serial.println(_WIFI_SSID);
    Serial.print("IP address: ");
    Serial.println(WiFi.localIP());
    return wifi;
}

bool sendImage(WiFiClient wifi) {
    digitalWrite(FLASH_LED_PIN, HIGH);
    delay(300);

    camera_fb_t *fb = esp_camera_fb_get();
    if (!fb) {
        digitalWrite(FLASH_LED_PIN, LOW);
        Serial.println("Could not get camera frame");
        return false;
    }

    digitalWrite(FLASH_LED_PIN, LOW);

    Serial.printf("Captured image: %u bytes\n", fb->len);

    HttpClient client = HttpClient(wifi, _SERVER_HOST, _SERVER_PORT);
    Serial.println("Making POST request");

    String path = "/ocr";
    String contentType = "image/jpeg";

    client.post(path.c_str(), contentType.c_str(), fb->len, fb->buf);

    int statusCode = client.responseStatusCode();
    String response = client.responseBody();

    Serial.print("Status code: ");
    Serial.println(statusCode);
    Serial.print("Response: ");
    Serial.println(response);

    esp_camera_fb_return(fb);

    if (statusCode != 202) {
        Serial.println("Upload failed");
        return false;
    }

    return true;
}

void loop() {
    // 1. Check WiFi connection, reconnect if lost
    if (WiFi.status() != WL_CONNECTED) {
        Serial.println("WiFi lost, reconnecting...");
        connectToWiFi();
        if (WiFi.status() != WL_CONNECTED) {
            Serial.println("Reconnect failed, retrying in 10 s");
            delay(CAPTURE_INTERVAL_MS);
            return;
        }
    }

    // 2. Create a fresh WiFiClient (the connection is managed by the HTTPClient)
    WiFiClient wifi;

    // 3. Capture and upload
    Serial.println("Starting capture cycle...");
    if (sendImage(wifi)) {
        Serial.println("Upload successful");
    } else {
        Serial.println("Upload failed");
    }

    // 4. Wait for the next cycle
    Serial.printf("Waiting %d ms until next capture...\n", CAPTURE_INTERVAL_MS);
    delay(CAPTURE_INTERVAL_MS);
}
