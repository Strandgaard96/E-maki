# whisper.cpp's whisper-server, built from source for this machine's CPU.
# WHISPER_GPU=vulkan builds it for a Vulkan GPU instead, e.g. integrated
# AMD or Intel graphics (GPU in .env, see docker/gpu.yml).
FROM debian:trixie-slim AS build
ARG WHISPER_GPU=
RUN apt-get update && apt-get install -y --no-install-recommends build-essential cmake git ca-certificates \
    $([ "$WHISPER_GPU" = vulkan ] && echo libvulkan-dev glslc spirv-headers) \
 && rm -rf /var/lib/apt/lists/*
ARG WHISPER_VERSION=v1.9.5
RUN git clone --depth 1 --branch ${WHISPER_VERSION} https://github.com/ggml-org/whisper.cpp /src
WORKDIR /src
RUN cmake -B build -DCMAKE_BUILD_TYPE=Release -DBUILD_SHARED_LIBS=OFF -DWHISPER_BUILD_TESTS=OFF \
    $([ "$WHISPER_GPU" = vulkan ] && echo -DGGML_VULKAN=ON) \
 && cmake --build build -j --target whisper-server

FROM debian:trixie-slim
ARG WHISPER_GPU=
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl libgomp1 libstdc++6 \
    $([ "$WHISPER_GPU" = vulkan ] && echo libvulkan1 mesa-vulkan-drivers) \
 && rm -rf /var/lib/apt/lists/*
COPY --from=build /src/build/bin/whisper-server /usr/local/bin/whisper-server
COPY docker/whisper-entrypoint.sh /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
