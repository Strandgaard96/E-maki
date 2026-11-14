// E-maki network microphone: an ESP32 with an I2S microphone (INMP441 or
// similar) that streams 16 kHz mono 16-bit PCM to E-maki over TCP.
// See docs/network-mic.md. Not yet tested on hardware.
//
// Needs the ESP32 Arduino core 3.x (ESP_I2S.h). Fill in the settings below.

#include <WiFi.h>
#include <ESP_I2S.h>

const char *WIFI_SSID = "your-wifi";
const char *WIFI_PASSWORD = "your-password";
const char *EMAKI_HOST = "192.168.1.10";  // the machine running E-maki
const uint16_t EMAKI_PORT = 10700;        // NET_MIC_PORT
const char *PANEL_TOKEN = "";             // the same as in E-maki's .env, or "" for none

// I2S pins: INMP441 SCK, WS and SD. L/R to GND (left channel).
const int PIN_BCLK = 26;
const int PIN_WS = 25;
const int PIN_DIN = 33;
// The INMP441 sends 24 bits in a 32-bit slot; shifting by less than 16
// adds gain. 14 is a good start for a room.
const int SHIFT = 14;

I2SClass i2s;
WiFiClient client;
int32_t raw[512];
int16_t pcm[512];

void setup() {
  Serial.begin(115200);
  WiFi.begin(WIFI_SSID, WIFI_PASSWORD);
  while (WiFi.status() != WL_CONNECTED) delay(500);
  Serial.printf("Wi-Fi: %s\n", WiFi.localIP().toString().c_str());
  i2s.setPins(PIN_BCLK, PIN_WS, -1, PIN_DIN);
  if (!i2s.begin(I2S_MODE_STD, 16000, I2S_DATA_BIT_WIDTH_32BIT, I2S_SLOT_MODE_MONO, I2S_STD_SLOT_LEFT)) {
    Serial.println("I2S did not start: check the pins");
    while (true) delay(1000);
  }
}

void loop() {
  if (!client.connected()) {
    if (!client.connect(EMAKI_HOST, EMAKI_PORT)) {
      delay(5000);  // E-maki not up yet: try again
      return;
    }
    client.setNoDelay(true);
    if (PANEL_TOKEN[0]) client.printf("%s\n", PANEL_TOKEN);
    Serial.println("streaming to E-maki");
  }
  size_t n = i2s.readBytes((char *)raw, sizeof(raw)) / sizeof(int32_t);
  for (size_t i = 0; i < n; i++) {
    int32_t s = raw[i] >> SHIFT;
    pcm[i] = s > 32767 ? 32767 : s < -32768 ? -32768 : s;
  }
  client.write((const uint8_t *)pcm, n * sizeof(int16_t));
}
