# Windows (WSL)

E-kiru is set up for Linux first. On Windows it runs in WSL 2, where WSLg carries the Windows microphone into Linux as a PulseAudio socket. Two things differ from the [Linux laptop test](laptop-test.md).

## Once

1. Windows 11 with WSL 2 and WSLg (default on current Windows 11). Check the mic reaches WSL: `ls /mnt/wslg/PulseServer` should exist.
2. Docker: Docker Desktop with the WSL 2 backend, or Docker Engine inside WSL.
3. Allow microphone access for desktop apps in Windows (Settings → Privacy & security → Microphone).

## Microphone

In `.env`, point E-kiru at WSLg's socket:

```sh
PULSE_SOCKET=/mnt/wslg/PulseServer
```

Only that socket is mounted into the container, not the rest of `/mnt/wslg`. After `wsl --shutdown` or a Windows restart, restart E-kiru too (`docker compose restart ekiru`).

## Integrated GPU

Use the WSL override instead of the Linux one. What WSL needs is listed at the top of the file.

```sh
docker compose -f docker-compose.yml -f docker-compose.igpu-wsl.yml up --build
```

Plain `docker compose up` does not use the GPU. To make it the default, add this line to `.env`:

```sh
COMPOSE_FILE=docker-compose.yml:docker-compose.igpu-wsl.yml
```

`docker compose config | grep -A3 dockerfile` should then show `WHISPER_GPU: vulkan`.

## Troubleshooting

- `miniaudio: Failed to open backend device`: check `/mnt/wslg/PulseServer` exists in WSL and that Windows allows mic access.
