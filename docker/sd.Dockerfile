# stable-diffusion.cpp's sd-server, built from source for this machine's CPU,
# for pictures made on this machine (see docs/local-images.md).
# SD_GPU=vulkan builds it for a Vulkan GPU instead, e.g. integrated AMD or
# Intel graphics (GPU in .env, see docker/options.yml).
FROM debian:trixie-slim AS build
ARG SD_GPU=
RUN apt-get update && apt-get install -y --no-install-recommends build-essential cmake git ca-certificates \
    $([ "$SD_GPU" = vulkan ] && echo libvulkan-dev glslc spirv-headers) \
 && rm -rf /var/lib/apt/lists/*
# stable-diffusion.cpp has no numbered releases; these tags are its builds.
ARG SD_VERSION=master-1091-b9d7176
RUN git clone --depth 1 --branch ${SD_VERSION} https://github.com/leejet/stable-diffusion.cpp /src \
 && git -C /src submodule update --init --depth 1 ggml
WORKDIR /src
# No web UI (E-maki has its own page) and no WebP/WebM, so no Node and fewer
# submodules.
RUN cmake -B build -DCMAKE_BUILD_TYPE=Release -DSD_SERVER_BUILD_FRONTEND=OFF -DSD_WEBP=OFF -DSD_WEBM=OFF \
    $([ "$SD_GPU" = vulkan ] && echo -DSD_VULKAN=ON) \
 && cmake --build build -j"$(nproc)" --target sd-server

FROM debian:trixie-slim
ARG SD_GPU=
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl libgomp1 libstdc++6 \
    $([ "$SD_GPU" = vulkan ] && echo libvulkan1 mesa-vulkan-drivers) \
 && rm -rf /var/lib/apt/lists/*
COPY --from=build /src/build/bin/sd-server /usr/local/bin/sd-server
COPY docker/sd-entrypoint.sh /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
