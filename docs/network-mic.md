# Network microphone

A mic in another room streams raw audio to E-maki over TCP: 16 kHz, mono, 16-bit little-endian PCM, the plain output of `arecord`.

## Turn it on

```bash
NET_MIC_BIND=0.0.0.0
```

Run `docker compose up -d` and pick Network microphone under Listening > Microphone. `NET_MIC_PORT` changes the port (10700). Without Docker, set `NET_MIC_LISTEN=:10700`.

One sender at a time. A new connection replaces the old one.

## Senders

Linux box with a mic:

```bash
arecord -q -f S16_LE -r 16000 -c 1 -t raw | nc <your-server> 10700
```

With `PANEL_TOKEN`, send it first:

```bash
(echo "$PANEL_TOKEN"; arecord -q -f S16_LE -r 16000 -c 1 -t raw) | nc <your-server> 10700
```

Run it as a systemd service with `Restart=always`.

ESP32 with an INMP441: see [Hardware](hardware.md#esp32-mic).

## Privacy

Audio crosses your network unencrypted. Set `PANEL_TOKEN`, keep `NET_MIC_BIND` on your local network, and don't forward the port.
