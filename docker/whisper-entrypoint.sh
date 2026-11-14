#!/bin/sh
# Two jobs, so the server that hears the audio needs no internet:
#   /entrypoint.sh download   fetch the model into /models, then exit
#   /entrypoint.sh            serve it
# WHISPER_MODEL_URL overrides the default download, e.g. for a Danish or
# Norwegian fine-tune in ggml format. Only use a URL you trust: the file is
# parsed by whisper.cpp. WHISPER_MODEL_SHA256, when set, must match it.
set -e
MODEL="${WHISPER_MODEL:-small}"
FILE="/models/${MODEL}.bin"
# Silero voice detection (under 1 MB): whisper skips the parts of a segment
# without speech instead of inventing text there, which is slow too.
VAD="/models/ggml-silero-v6.2.0.bin"

if [ "$1" = download ]; then
  URL="${WHISPER_MODEL_URL:-https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-${MODEL}.bin}"
  if [ ! -s "$FILE" ]; then
    echo "downloading $URL"
    curl -fL --retry 3 -o "$FILE.part" "$URL"
    # A real model is hundreds of MB; anything tiny is an error page.
    if [ "$(wc -c < "$FILE.part")" -lt 10000000 ]; then
      echo "download of $URL is too small to be a model" >&2
      rm -f "$FILE.part"; exit 1
    fi
    if [ -n "$WHISPER_MODEL_SHA256" ] && ! echo "$WHISPER_MODEL_SHA256  $FILE.part" | sha256sum -c - >/dev/null; then
      echo "download of $URL does not match WHISPER_MODEL_SHA256" >&2
      rm -f "$FILE.part"; exit 1
    fi
    mv "$FILE.part" "$FILE"
  fi
  if [ ! -s "$VAD" ]; then
    curl -fL --retry 3 -o "$VAD.part" https://huggingface.co/ggml-org/whisper-vad/resolve/main/ggml-silero-v6.2.0.bin
    mv "$VAD.part" "$VAD"
  fi
  exit 0
fi

if [ ! -s "$FILE" ] || [ ! -s "$VAD" ]; then
  echo "no model at $FILE: run the whisper-model service first (docker compose up does)" >&2
  exit 1
fi
# -nf: no temperature fallback. On unclear audio whisper otherwise decodes
# the segment up to six times, which took 70-120 s per 15 s segment.
exec whisper-server -m "$FILE" --host 0.0.0.0 --port 8081 -t "${WHISPER_THREADS:-$(nproc)}" \
  -nf --vad -vm "$VAD"
