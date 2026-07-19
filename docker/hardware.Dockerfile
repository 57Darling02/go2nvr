# syntax=docker/dockerfile:labs

# 0. Prepare images
# only debian 13 (trixie) has latest ffmpeg
# https://packages.debian.org/trixie/ffmpeg
ARG DEBIAN_VERSION="trixie-slim"
ARG GO_VERSION="1.25-bookworm"
ARG NODE_VERSION="22-bookworm-slim"


# 1. Build the embedded Web UI
FROM --platform=$BUILDPLATFORM node:${NODE_VERSION} AS webui

WORKDIR /build

COPY webui/package.json webui/pnpm-lock.yaml webui/pnpm-workspace.yaml ./webui/
RUN npm install --global corepack@0.33.0 \
    && corepack enable \
    && corepack install --global pnpm@10.33.0 \
    && pnpm --dir webui install --frozen-lockfile

COPY webui ./webui
COPY scripts/build-webui.sh ./scripts/build-webui.sh
RUN ./scripts/build-webui.sh


# 2. Build go2nvr binary
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION} AS build
ARG APP_VERSION
ARG TARGETPLATFORM
ARG TARGETOS
ARG TARGETARCH

ENV GOOS=${TARGETOS}
ENV GOARCH=${TARGETARCH}

WORKDIR /build

# Cache dependencies
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/root/.cache/go-build go mod download

COPY . .
COPY --from=webui /build/internal/nvrui/dist ./internal/nvrui/dist
RUN --mount=type=cache,target=/root/.cache/go-build \
    ldflags="-s -w"; \
    if [ -n "${APP_VERSION:-}" ]; then ldflags="$ldflags -X github.com/AlexxIT/go2rtc/internal/app.VersionOverride=${APP_VERSION#v}"; fi; \
    CGO_ENABLED=0 GO2NVR_SKIP_WEBUI=1 ./scripts/build-go2nvr.sh -ldflags "$ldflags" -trimpath -o go2nvr


# 3. Final image
FROM debian:${DEBIAN_VERSION}

# Prepare apt for buildkit cache
RUN rm -f /etc/apt/apt.conf.d/docker-clean \
  && echo 'Binary::apt::APT::Keep-Downloaded-Packages "true";' >/etc/apt/apt.conf.d/keep-cache

# Install ffmpeg, tini (for signal handling),
# and other common tools for the echo source.
# non-free for Intel QSV support (not used by go2rtc, just for tests)
# mesa-va-drivers for AMD APU
# libasound2-plugins for ALSA support
RUN --mount=type=cache,target=/var/cache/apt,sharing=locked --mount=type=cache,target=/var/lib/apt,sharing=locked \
    echo 'deb http://deb.debian.org/debian trixie non-free' > /etc/apt/sources.list.d/debian-non-free.list && \
    apt-get -y update && apt-get -y install ffmpeg tini \
        python3 curl jq \
        intel-media-va-driver-non-free \
        mesa-va-drivers \
        libasound2-plugins && \
    apt-get clean && rm -rf /var/lib/apt/lists/*

COPY --from=build /build/go2nvr /usr/local/bin/

EXPOSE 1984 8554 8555 8555/udp
ENTRYPOINT ["/usr/bin/tini", "--"]
VOLUME /config
WORKDIR /config
# https://github.com/NVIDIA/nvidia-docker/wiki/Installation-(Native-GPU-Support)
ENV NVIDIA_VISIBLE_DEVICES all
ENV NVIDIA_DRIVER_CAPABILITIES compute,video,utility

CMD ["go2nvr", "-config", "/config/go2nvr.yaml"]
