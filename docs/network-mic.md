# Network microphone

E-maki can listen to a microphone in another room, or on a small board next to the frame, instead of one plugged into the brain. The sender streams raw audio over TCP: 16 kHz, mono, 16-bit little-endian PCM. That is the plain output of `arecord`, so most senders need no code at all.

It is off until you set it up, and it is hardware-dependent: please report how it goes.

## Turn it on

With docker compose, set this in `.env` and run `docker compose up -d`:

```bash
NET_MIC_BIND=0.0.0.0
```

`NET_MIC_PORT` changes the port (10700). Without docker, set `net_mic_listen` in your profile (for example `":10700"`) or `NET_MIC_LISTEN`. Then open the settings page, go to E-maki → Microphone and pick **Network microphone**. The level meter moves once a sender is connected.

One sender at a time. A new connection replaces the old one, so a sender that restarts is heard again at once.

## Senders

**A Raspberry Pi (or any Linux box) with a USB or HAT microphone:**

```bash
arecord -q -f S16_LE -r 16000 -c 1 -t raw | nc <your-brain> 10700
```

E-maki only hears a sender that first sends the page's token on its own line. That is your `PANEL_TOKEN`, or the one E-maki made (`docker compose exec emaki cat /data/token`):

```bash
(echo "$PANEL_TOKEN"; arecord -q -f S16_LE -r 16000 -c 1 -t raw) | nc <your-brain> 10700
```

Wrap it in a systemd service with `Restart=always` to keep it running.

**An ESP32 with an I2S microphone (INMP441):** [examples/esp32-mic](../examples/esp32-mic/esp32-mic.ino) is an Arduino sketch. Fill in Wi-Fi, E-maki's address and the token, and flash it. Not yet tested on hardware.

## Privacy

Audio crosses your local network unencrypted. E-maki still keeps it in memory only and never writes it to disk. Only senders that know the token are heard (unless you set `PANEL_OPEN=1`). Keep `NET_MIC_BIND` to your local network, and never forward the port to the internet.
