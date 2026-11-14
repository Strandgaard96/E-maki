#!/bin/sh
# Two jobs, like whisper, so the server that draws needs no internet:
#   /entrypoint.sh download   fetch the model files into /models, then exit
#   /entrypoint.sh            serve them on port 1234
# The default is FLUX.2 klein 4B (Apache 2.0) in 4-bit, with its Qwen3 4B
# text encoder and VAE: about 5.3 GB on disk. SD_MODEL_URL,
# SD_TEXT_ENCODER_URL and SD_VAE_URL point at other files. Set those two to
# "none" for a model that is one checkpoint file (SD-Turbo, SDXL-Turbo). Only use sources you trust: the files are parsed by
# stable-diffusion.cpp.
set -e
MODEL_URL="${SD_MODEL_URL:-https://huggingface.co/unsloth/FLUX.2-klein-4B-GGUF/resolve/main/flux-2-klein-4b-Q4_0.gguf}"
TEXT_URL="${SD_TEXT_ENCODER_URL:-https://huggingface.co/unsloth/Qwen3-4B-GGUF/resolve/main/Qwen3-4B-Q4_K_M.gguf}"
VAE_URL="${SD_VAE_URL:-https://huggingface.co/Comfy-Org/flux2-klein-4B/resolve/main/split_files/vae/flux2-vae.safetensors}"
[ "$TEXT_URL" = none ] && TEXT_URL=
[ "$VAE_URL" = none ] && VAE_URL=

file() { if [ -n "$1" ]; then echo "/models/$(basename "${1%%\?*}")"; fi; }
MODEL="$(file "$MODEL_URL")"
TEXT="$(file "$TEXT_URL")"
VAE="$(file "$VAE_URL")"

if [ "$1" = download ]; then
  for url in "$MODEL_URL" "$TEXT_URL" "$VAE_URL"; do
    [ -z "$url" ] && continue
    f="$(file "$url")"
    [ -s "$f" ] && continue
    echo "downloading $url"
    curl -fL --retry 3 -o "$f.part" "$url"
    # A real model file is tens of MB or more; anything tiny is an error page.
    if [ "$(wc -c < "$f.part")" -lt 10000000 ]; then
      echo "download of $url is too small to be a model" >&2
      rm -f "$f.part"; exit 1
    fi
    mv "$f.part" "$f"
  done
  exit 0
fi

for f in "$MODEL" "$TEXT" "$VAE"; do
  if [ -n "$f" ] && [ ! -s "$f" ]; then
    echo "no model at $f: run the sd-model service first (docker compose up does)" >&2
    exit 1
  fi
done
if [ -n "$TEXT" ]; then
  set -- --diffusion-model "$MODEL" --llm "$TEXT"
else
  set -- --model "$MODEL"
fi
if [ -n "$VAE" ]; then set -- "$@" --vae "$VAE"; fi
# Distilled models: few steps and no CFG. SD_ARGS adds anything else, e.g.
# "--offload-to-cpu" on a small GPU.
# shellcheck disable=SC2086
exec sd-server "$@" -l 0.0.0.0 --listen-port 1234 --threads "${SD_THREADS:-$(nproc)}" \
  --steps "${SD_STEPS:-4}" --cfg-scale 1.0 $SD_ARGS
