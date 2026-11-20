#!/bin/bash
# Downloads the speech models into /models on the first start, then serves
# them. Stock Whisper answers on port 8081. With WHISPER_DA_NO=1 the Danish
# (Edda) and Norwegian (NB-Whisper) models run next to it on 8082 and 8083.
# A *_URL overrides a download. Only use a URL you trust: the file is parsed
# by whisper.cpp. A *_SHA256, when set, must match the file.
set -e
VAD="/models/ggml-silero-v6.2.0.bin"

# fetch NAME URL SHA256: download /models/NAME.bin once.
fetch() {
  local file="/models/$1.bin" url="$2" sum="$3"
  [ -s "$file" ] && return
  echo "downloading $url"
  curl -fL --retry 3 -o "$file.part" "$url"
  # A real model is hundreds of MB; anything tiny is an error page.
  if [ "$(wc -c < "$file.part")" -lt 10000000 ]; then
    echo "download of $url is too small to be a model" >&2
    rm -f "$file.part"; exit 1
  fi
  if [ -n "$sum" ] && ! echo "$sum  $file.part" | sha256sum -c - >/dev/null; then
    echo "download of $url does not match its SHA-256" >&2
    rm -f "$file.part"; exit 1
  fi
  mv "$file.part" "$file"
}

# serve NAME PORT: run whisper-server for /models/NAME.bin in the background.
# -nf: no temperature fallback. On unclear audio whisper otherwise decodes
# the segment up to six times, which took 70-120 s per 15 s segment.
# --vad: Silero voice detection skips the parts without speech instead of
# inventing text there.
serve() {
  whisper-server -m "/models/$1.bin" --host 0.0.0.0 --port "$2" \
    -t "${WHISPER_THREADS:-$(nproc)}" -nf --vad -vm "$VAD" &
}

MODEL="${WHISPER_MODEL:-small}"
fetch "$MODEL" "${WHISPER_MODEL_URL:-https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-${MODEL}.bin}" "$WHISPER_MODEL_SHA256"
if [ ! -s "$VAD" ]; then
  curl -fL --retry 3 -o "$VAD.part" https://huggingface.co/ggml-org/whisper-vad/resolve/main/ggml-silero-v6.2.0.bin
  mv "$VAD.part" "$VAD"
fi

# docker stop: stop the servers too.
trap 'jobs -p | xargs -r kill 2>/dev/null; exit 0' TERM INT
if [ -n "$WHISPER_DA_NO" ]; then
  # Edda's makers publish no ggml file, so its URL has no default.
  if [ -z "$WHISPER_DA_MODEL_URL" ]; then
    echo "WHISPER_DA_MODEL_URL is not set in .env, see docs/setup.md" >&2; exit 1
  fi
  DA="${WHISPER_DA_MODEL:-edda-v0.2}"
  NO="${WHISPER_NO_MODEL:-nb-whisper-medium-q5_0}"
  fetch "$DA" "$WHISPER_DA_MODEL_URL" "$WHISPER_DA_MODEL_SHA256"
  fetch "$NO" "${WHISPER_NO_MODEL_URL:-https://huggingface.co/NbAiLab/nb-whisper-medium/resolve/main/ggml-model-q5_0.bin}" "$WHISPER_NO_MODEL_SHA256"
  serve "$DA" 8082
  serve "$NO" 8083
fi
serve "$MODEL" 8081

# If any server stops, stop the others and the container, so docker
# restarts it.
wait -n || true
jobs -p | xargs -r kill 2>/dev/null
exit 1
