# Local pictures

The `imagegen` container draws pictures on the brain with [stable-diffusion.cpp](https://github.com/leejet/stable-diffusion.cpp). Nothing leaves the house.

## Turn it on

```sh
LOCAL_IMAGES=1
```

Run `docker compose up -d --build`, then pick "On this machine" under System > Image provider. With `GPU=vulkan` ([Setup](setup.md#gpu)) it uses the GPU.

## Default model

[FLUX.2 klein 4B](https://huggingface.co/black-forest-labs/FLUX.2-klein-4B) (Apache 2.0), 4-bit, 4 steps.

| | |
| --- | --- |
| Download | about 5.3 GB |
| RAM | about 8 GB on top of whisper and Ollama |
| Time | seconds on a GPU, minutes on a CPU |

On a slow CPU, set `LOCAL_IMAGE_SIZE=768x576`.

## Other models

Point the URLs at the model's files, or `none` for parts it doesn't have. A lighter example, [SD-Turbo](https://huggingface.co/stabilityai/sd-turbo):

```sh
SD_MODEL_URL=<link to sd_turbo.safetensors>
SD_TEXT_ENCODER_URL=none
SD_VAE_URL=none
SD_STEPS=1
LOCAL_IMAGE_SIZE=512x384
LOCAL_IMAGE_MODEL=SD-Turbo
```

Files are kept in the `sd-models` volume. Only use sources you trust.

## Without Docker

Run `sd-server` yourself and point E-maki at it:

```sh
sd-server --diffusion-model flux-2-klein-4b-Q4_0.gguf --llm Qwen3-4B-Q4_K_M.gguf \
  --vae flux2-vae.safetensors --steps 4 --cfg-scale 1.0 -l 127.0.0.1 --listen-port 1234
LOCAL_IMAGE_URL=http://localhost:1234 ./emaki -config configs/laptop.json
```

Any server with an OpenAI-style `POST /v1/images/generations` works.
