# Hardware

> [!WARNING]
> No complete E-maki frame has been built yet. Everything on this page except "a laptop" is a plan, not a tested build. The parts are the ones Fugleramme uses, so the frame side should work as it does there, but nobody has checked. If you build one, please share how it went in [Discussions](https://github.com/Strandgaard96/E-maki/discussions).

An E-maki setup has three parts:

- **The brain** listens, transcribes, picks topics and makes the page. It needs real computing power.
- **A mic** where people talk. It can be plugged into the brain, or stream to it over Wi-Fi.
- **A screen** that shows the page. The brain serves it over HTTP using Fugleramme's panel protocol, so anything that can show a Fugleramme page can show E-maki: an e-ink frame, a TV, a browser.

The frame and the mic don't think. They can be small and sit anywhere on your network.

## Try it first: just a laptop

You don't need to buy anything to see what it does. A Linux laptop (or Windows with WSL) runs the brain, its built-in mic does the listening, and a browser is the screen. See [Setup](setup.md).

This is the only setup that has actually been run so far: an AMD laptop with an integrated GPU, in WSL.

## The brain

| Option | Notes |
| --- | --- |
| Laptop, mini PC or homelab box | Any x86-64 Linux machine that runs Docker. Tested on an AMD laptop with an integrated GPU (Vulkan) |
| Jetson Orin Nano Super 8 GB | The plan for a small, quiet, always-on brain. **Untested.** `configs/jetson.json` is a first sketch, and there is no compose file for it yet |

How much it needs, roughly, from the model sizes:

- **RAM:** the default topic model (`qwen3.5:4b`) takes about 3.4 GB and Whisper `small` about 0.5 GB, so 8 GB is tight and 16 GB is comfortable. The Danish and Norwegian models add about 1.6 GB. Local pictures (FLUX.2 klein) need several GB more.
- **Disk:** a few GB for the models, plus the saved pictures.
- **GPU:** optional. An integrated AMD or Intel GPU through Vulkan makes speech-to-text about 7x faster on the test laptop. No NVIDIA setup has been tried.
- **CPU only** works, but use Whisper `small` and expect topics to lag behind the talk.

The brain doesn't have to be next to the frame or the mic. Put it wherever it can run quietly.

## The mic

| Option | Notes |
| --- | --- |
| Built-in or USB mic on the brain | The simplest. Reached through PulseAudio or PipeWire. Put it where people talk, like the kitchen table |
| A Raspberry Pi (or any Linux box) with a USB mic | Streams to the brain with `arecord` and `nc`, no code needed. See [Network microphone](network-mic.md) |
| ESP32 with an INMP441 I2S mic | A cheap Wi-Fi mic. Sketch in [examples/esp32-mic](../examples/esp32-mic/esp32-mic.ino). **Untested on hardware** |

A room mic needs to catch speech from a few metres away, which is different from what Fugleramme's bird mics are chosen for. Any decent USB conference mic should do; none has been compared yet.

### ESP32 mic: parts and wiring

| Part | What works |
| --- | --- |
| Board | Any ESP32 with Wi-Fi supported by the ESP32 Arduino core 3.x (an ESP32 DevKit, for example) |
| Mic | INMP441 I2S breakout |
| Power | USB, 5 V |

Wiring, as set in the sketch:

| INMP441 | ESP32 |
| --- | --- |
| VDD | 3.3 V |
| GND | GND |
| SCK | GPIO 26 |
| WS | GPIO 25 |
| SD | GPIO 33 |
| L/R | GND (left channel) |

Other pins work too: change `PIN_BCLK`, `PIN_WS` and `PIN_DIN` at the top of the sketch.

### Flashing the ESP32 mic

1. Install the [Arduino IDE](https://www.arduino.cc/en/software) and the ESP32 board package (Espressif's, version 3.x) from the Boards Manager.
2. Open `examples/esp32-mic/esp32-mic.ino`.
3. Fill in `WIFI_SSID`, `WIFI_PASSWORD`, `EMAKI_HOST` (the brain's address) and `PANEL_TOKEN` (the page's token: yours from `.env`, or the one from `docker compose exec emaki cat /data/token`).
4. Pick your board and port, and upload.
5. Open the serial monitor at 115200 baud. It prints its IP, then `streaming to E-maki` once it reaches the brain.

On the brain, switch the network mic on and pick it on the settings page: see [Network microphone](network-mic.md). If speech comes in too quiet or clips, change `SHIFT` in the sketch (lower is louder).

## The frame

The default page is 1600x1200 for a 13.3" Spectra 6 colour e-ink panel. Smaller panels and portrait are a setting on the settings page.

### Raspberry Pi frame

The same frame as Fugleramme's "frame only" build: a Pi drives the panel and fetches the page from the brain.

| Part | What works |
| --- | --- |
| Board | Raspberry Pi 5 (1 GB is plenty), Pi 4, or a Pi Zero 2 W with the 40-pin header |
| Storage | microSD card, 16 GB or more |
| Panel | Pimoroni [Inky Impression 13.3"](https://shop.pimoroni.com/products/inky-impression) (Spectra 6, 1600x1200). The 7.3", 5.7" and 4" work too, at lower resolution |
| Power | The official supply for the board |
| Frame | IKEA RÖDALM 21x30 (A4) for the 13.3", 13x18 for the 7.3" |

Software is Fugleramme's Inky client, one command on the Pi: see [Setup](setup.md#5-put-it-on-a-frame). The Inky's four buttons pause listening, remove or pin the last picture, and put the next one up.

Fugleramme's [hardware guide](https://github.com/arnegiacomo/fugleramme/blob/main/docs/hardware.md) has more on the Pi and the panel, and its [mounting guide](https://github.com/arnegiacomo/fugleramme/blob/main/docs/mounting.md) covers fitting the panel in the frame, the passepartout, airflow and stand feet. It all applies here unchanged. The Pi only fetches a picture every few minutes, so it stays cool.

### Battery frame (ESP32)

A Seeed Studio XIAO ESP32-S3 board drives the panel and runs from a battery, with no wires.

| Part | What works |
| --- | --- |
| Board | [XIAO ePaper Display Board EE02](https://www.seeedstudio.com/XIAO-ePaper-Display-Board-ESP32-S3-EE02-p-6639.html) |
| Panel | [Seeed Studio 13.3" six-colour e-ink](https://www.seeedstudio.com/13-3inch-Six-Color-eInk-ePaper-Display-with-1200x1600-Pixels-p-6569.html) (Spectra 6, 1600x1200) |
| Power | Single-cell 3.7 V lithium battery with a JST 2.0 connector |
| Frame | IKEA RÖDALM 21x30 (A4) |

Seeed also sells the board and panel as a [kit](https://www.seeedstudio.com/XIAO-ePaper-DIY-Kit-EE02-for-13-3-Spectratm-6-E-Ink.html). The firmware is in [fugleramme/clients](https://github.com/fugleramme/clients): point it at the brain's address. It fetches `/frame.e6`, which E-maki serves for the 13.3". Its buttons work on the board itself and don't reach E-maki, so set Buttons to None on the settings page.

### No e-ink

The page is a plain PNG, so a TV, an old tablet or any browser can show it. Open `http://<your-brain>:8090/<token>/frame.png`. Other devices people have made work with Fugleramme's protocol are in [fugleramme/clients](https://github.com/fugleramme/clients).

## A small, quiet build (the plan)

What the finished thing is meant to look like, none of it tested together yet:

| Part | Plan |
| --- | --- |
| Brain | Jetson Orin Nano Super 8 GB, in a cupboard |
| Mic | ESP32 + INMP441 on the kitchen table |
| Frame | 13.3" Spectra 6 on a Pi or an ESP32 EE02, on the wall |
| Mode | Diary: one new page each morning |
