# syntax=docker/dockerfile:labs

# 0. Prepare images
ARG PYTHON_VERSION="3.13"
ARG GO_VERSION="1.25"
ARG NODE_VERSION="22-alpine"


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
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS build
ARG APP_VERSION
ARG TARGETPLATFORM
ARG TARGETOS
ARG TARGETARCH

ENV GOOS=${TARGETOS}
ENV GOARCH=${TARGETARCH}

WORKDIR /build

RUN apk add git

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
FROM python:${PYTHON_VERSION}-alpine AS base

# Install ffmpeg, tini (for signal handling),
# and other common tools for the echo source.
# alsa-plugins-pulse for ALSA support (+0MB)
# font-droid for FFmpeg drawtext filter (+2MB)
RUN apk add --no-cache tini ffmpeg ffplay bash curl jq alsa-plugins-pulse font-droid

# Hardware Acceleration for Intel CPU (+50MB)
ARG TARGETARCH

RUN if [ "${TARGETARCH}" = "amd64" ]; then apk add --no-cache libva-intel-driver intel-media-driver; fi

# Hardware: AMD and NVidia VAAPI (not sure about this)
# RUN libva-glx mesa-va-gallium
# Hardware: AMD and NVidia VDPAU (not sure about this)
# RUN libva-vdpau-driver mesa-vdpau-gallium (+150MB total)

COPY --from=build /build/go2nvr /usr/local/bin/

EXPOSE 1984 8554 8555 8555/udp
ENTRYPOINT ["/sbin/tini", "--"]
VOLUME /config
WORKDIR /config

CMD ["go2nvr", "-config", "/config/go2nvr.yaml"]
