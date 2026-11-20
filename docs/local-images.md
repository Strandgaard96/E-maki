# Pictures made on this machine

With a cloud image API, the English picture description is the one thing that leaves the house. With local pictures not even that does.

Local pictures are one more container on the same machine, started by the same `docker compose`:

| Container | Does |
| --- | --- |
| `emaki` | the app |
| `whisper` | speech to text |
| `ollama` | topics |
| `sd` (optional) | draws the pictures with [stable-diffusion.cpp](https://github.com/leejet/stable-diffusion.cpp) |

`sd` runs on this machine next to whisper and Ollama. It is off by default because its model is a big download. Nothing has to run anywhere else.

## Start it

Set this in `.env` and run `docker compose up -d --build`:

```sh
LOCAL_IMAGES=1
```

The first start builds sd-server from source and downloads the model. Once it is up, the settings page lists "On this machine" under System > Image provider. With "Automatic" it is picked over any API key in `.env`.

With `GPU=vulkan` or `GPU=vulkan-wsl` (see [Setup](setup.md#integrated-gpu)) it draws on the integrated GPU as well.

## The model

The default is [FLUX.2 klein 4B](https://huggingface.co/black-forest-labs/FLUX.2-klein-4B) (Apache 2.0), quantised to 4 bits, with its Qwen3 4B text encoder and VAE. It draws a picture in 4 steps.

| | |
| --- | --- |
| Download | about 5.3 GB (model 2.5 GB, text encoder 2.5 GB, VAE 0.3 GB) |
| Memory while drawing | plan for about 8 GB, on top of whisper and the topic LLM |
| Time per picture | seconds on a GPU; on a laptop CPU expect minutes |

Rough numbers: they have not been measured on E-maki's own hardware yet. Listening and topic detection go on while a picture is drawn. New topics wait in line for their picture, a few at a time. On a slow CPU, use a smaller picture (`LOCAL_IMAGE_SIZE=768x576`) or a GPU.

That is too much for an 8 GB Jetson that also runs Whisper and the topic LLM. There, use a cloud API, or a smaller single-file model such as [SD-Turbo](https://huggingface.co/stabilityai/sd-turbo), which is much lighter but looks worse. In `.env`, with the download link of its `.safetensors` file:

```sh
SD_MODEL_URL=<link to sd_turbo.safetensors>
SD_TEXT_ENCODER_URL=none
SD_VAE_URL=none
SD_STEPS=1
LOCAL_IMAGE_SIZE=512x384
LOCAL_IMAGE_MODEL=SD-Turbo
```

Any model stable-diffusion.cpp supports works the same way: point the three URLs at its files in `.env` (or "none" for parts it does not have), and set its step count. Files are kept by name in the `sd-models` volume, so old ones stay until you remove it. Only use sources you trust.

## Without docker

On a Jetson or any machine where you run the sidecars yourself, build `sd-server` from stable-diffusion.cpp (with `-DSD_CUDA=ON` on a Jetson), start it with the model files, and point E-maki at it:

```sh
sd-server --diffusion-model flux-2-klein-4b-Q4_0.gguf --llm Qwen3-4B-Q4_K_M.gguf \
  --vae flux2-vae.safetensors --steps 4 --cfg-scale 1.0 -l 127.0.0.1 --listen-port 1234
LOCAL_IMAGE_URL=http://localhost:1234 ./emaki -config configs/jetson.json
```

E-maki talks to it over its OpenAI-style endpoint, `POST /v1/images/generations`, so any other local server with that endpoint works too.
