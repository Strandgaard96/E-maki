# Windows (WSL)

E-maki is set up for Linux first. On Windows it runs in WSL 2, where WSLg carries the Windows microphone into Linux as a PulseAudio socket. Two things differ from the [Linux laptop test](laptop-test.md).

## Once

1. Windows 11 with WSL 2 and WSLg (default on current Windows 11). Check the mic reaches WSL: `ls /mnt/wslg/PulseServer` should exist.
2. Docker: Docker Desktop with the WSL 2 backend, or Docker Engine inside WSL.
3. Allow microphone access for desktop apps in Windows (Settings → Privacy & security → Microphone).

## Microphone

In `.env`, point E-maki at WSLg's socket:

```sh
PULSE_SOCKET=/mnt/wslg/PulseServer
```

Only that socket is mounted into the container, not the rest of `/mnt/wslg`. After `wsl --shutdown` or a Windows restart, restart E-maki too (`docker compose restart emaki`).

## Integrated GPU

Set this in `.env`, then `docker compose up -d --build`. What WSL needs is listed at the end of [docker-compose.yml](../docker-compose.yml).

```sh
GPU=vulkan-wsl
```

`docker compose config | grep WHISPER_GPU` should then show `vulkan`.

## Troubleshooting

- `miniaudio: Failed to open backend device`: check `/mnt/wslg/PulseServer` exists in WSL and that Windows allows mic access.
