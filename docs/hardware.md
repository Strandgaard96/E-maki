# Hardware

An E-maki setup has three parts:

- The server: a Linux computer with Docker. It listens, transcribes, picks topics and makes the picture.
- A mic, on the server or on the network.
- An e-ink frame that fetches the picture from the server.

## The server

Any x86-64 Linux machine that runs Docker: a laptop, a mini PC or a homelab server.

- RAM: 16 GB. The default topic model takes about 3.4 GB and Whisper `small` about 0.5 GB. Danish and Norwegian add about 1.6 GB, local pictures several GB more.
- Disk: a few GB for the models and the saved pictures.
- GPU: optional. An integrated AMD or Intel GPU (Vulkan) makes speech-to-text about 7x faster.

## The mic

| Mic | Notes |
| --- | --- |
| USB or built-in mic on the server | Through PulseAudio or PipeWire |
| Raspberry Pi or other Linux box with a USB mic | Streams with `arecord` and `nc`. See [Network microphone](network-mic.md) |
| ESP32 with an INMP441 | Wi-Fi mic. Sketch in [examples/esp32-mic](../examples/esp32-mic/esp32-mic.ino) |

### ESP32 mic

Any ESP32 with Wi-Fi (Arduino core 3.x), an INMP441 I2S breakout and 5 V USB power.

| INMP441 | ESP32 |
| --- | --- |
| VDD | 3.3 V |
| GND | GND |
| SCK | GPIO 26 |
| WS | GPIO 25 |
| SD | GPIO 33 |
| L/R | GND |

Change `PIN_BCLK`, `PIN_WS` and `PIN_DIN` in the sketch for other pins.

1. Install the [Arduino IDE](https://www.arduino.cc/en/software) and Espressif's ESP32 board package 3.x.
2. Open `examples/esp32-mic/esp32-mic.ino`.
3. Set `WIFI_SSID`, `WIFI_PASSWORD`, `EMAKI_HOST` and `PANEL_TOKEN` if you use one.
4. Upload, and open the serial monitor at 115200 baud. It prints `streaming to E-maki` when it connects.

Then turn on the network mic: see [Network microphone](network-mic.md). If speech is too quiet or clips, change `SHIFT` in the sketch (lower is louder).

## The frame

The picture is 1600x1200 for a 13.3" Spectra 6 panel. Other sizes, portrait and fewer colours (4 colours, 4 greys, black and white) are set under Frame > Panel. Pictures are dithered with Atkinson dithering.

### Raspberry Pi

| Part | What to use |
| --- | --- |
| Board | Raspberry Pi 5, Pi 4 or Pi Zero 2 W |
| Storage | microSD, 16 GB or more |
| Panel | Pimoroni [Inky Impression 13.3"](https://shop.pimoroni.com/products/inky-impression). The 7.3", 5.7" and 4" work too |
| Frame | IKEA RÖDALM 21x30 for the 13.3", 13x18 for the 7.3" |

The Pi runs Fugleramme's Inky client: see [Setup](setup.md#4-put-it-on-a-frame). For mounting, see Fugleramme's [hardware](https://github.com/arnegiacomo/fugleramme/blob/main/docs/hardware.md) and [mounting](https://github.com/arnegiacomo/fugleramme/blob/main/docs/mounting.md) guides.

### XIAO EE02 (battery)

| Part | What to use |
| --- | --- |
| Board | [XIAO ePaper Display Board EE02](https://www.seeedstudio.com/XIAO-ePaper-Display-Board-ESP32-S3-EE02-p-6639.html) |
| Panel | [Seeed 13.3" six-colour e-ink](https://www.seeedstudio.com/13-3inch-Six-Color-eInk-ePaper-Display-with-1200x1600-Pixels-p-6569.html) |
| Power | 3.7 V lithium battery, JST 2.0 |
| Frame | IKEA RÖDALM 21x30 |

Seeed sells board and panel as a [kit](https://www.seeedstudio.com/XIAO-ePaper-DIY-Kit-EE02-for-13-3-Spectratm-6-E-Ink.html). Flash the [xiao-arduino](https://github.com/fugleramme/clients/tree/main/xiao-arduino) firmware:

1. Connect the board over USB-C. Hold boot, press reset, release boot.
2. Open the [web flasher](https://fugleramme.github.io/clients/) in Chrome or Edge, pick EE02 and connect.
3. Join the board's Wi-Fi from the QR code. Set your Wi-Fi, the server's address as host and `8090` as port. Leave the password empty.

Hold KEY2 for 5 seconds to change the settings, KEY1 for 10 seconds to wipe them.

The buttons send A (KEY3), B (KEY2) and C (KEY1). Turn on Board buttons under Frame > Panel to use them.

### Other clients

More boards are in [fugleramme/clients](https://github.com/fugleramme/clients). A client fetches one of these:

| Fetch | Format |
| --- | --- |
| `GET /frame.png` | PNG at the panel size, in its colours |
| `GET /frame.e6` | Packed for a 13.3" or 7.3" Spectra 6 panel. The header holds the sleep time |

Both send an `ETag` and answer `304` until the picture changes. With `PANEL_TOKEN` set, prefix the path with `/<token>`.

Buttons send a letter from `A` to `H`:

```bash
curl -X POST -d button=A http://<your-server>:8090/press
```

| Client | Buttons |
| --- | --- |
| Inky client on a Pi | A to D |
| xiao-arduino (EE02, EE04) | A to C |
| MicroPython (EE02, EE04) | None |
