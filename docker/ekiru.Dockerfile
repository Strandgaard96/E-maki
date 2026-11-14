FROM golang:1.27-trixie AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -o /ekiru ./cmd/ekiru

FROM debian:trixie-slim
# libpulse0: the microphone is reached through PulseAudio (or PipeWire's PulseAudio socket).
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates libpulse0 \
 && rm -rf /var/lib/apt/lists/*
COPY --from=build /ekiru /usr/local/bin/ekiru
COPY configs /configs
# A new named volume on /data takes this owner, so the settings page can save.
RUN mkdir /data && chown 1000:1000 /data
# Not root. docker compose overrides this with your own user id, which the
# PulseAudio/PipeWire socket expects.
USER 1000:1000
ENTRYPOINT ["ekiru"]
CMD ["-config", "/configs/docker.json", "-mic"]
