# ── Stage 1: build ────────────────────────────────────────────────────────────
FROM golang:1.27.4-alpine AS builder

WORKDIR /build
COPY . .
RUN go mod download

ARG VERSION
RUN CGO_ENABLED=0 go build \
    # trim debug info and set version
    -ldflags="-s -w -X main.version=${VERSION:-local}" \
    -o bench-mcp .

# ── Stage 2: mise ─────────────────────────────────────────────────────────────
# official release image, a single static binary; a FROM line so Dependabot bumps the version
FROM ghcr.io/jdx/mise:2026.11.8 AS mise

# ── Stage 3: runtime ──────────────────────────────────────────────────────────
FROM ubuntu:26.04

# skip prompts from apt
ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y \
    # shells
    bash \
    # core utils
    curl wget git make jq \
    # build tools
    gcc g++ build-essential pkg-config \
    # scripting
    python3 python3-pip \
    # file tools
    zip unzip tar \
    # text / search
    ripgrep vim nano less \
    # network
    dnsutils iputils-ping netcat-openbsd \
    && rm -rf /var/lib/apt/lists/*

COPY --from=mise /usr/local/bin/mise /usr/local/bin/mise

ENV MISE_DATA_DIR=/mise
ENV MISE_CONFIG_DIR=/mise
ENV PATH="/mise/shims:$PATH"

# pin mcp: mcpo/mcp-proxy declare mcp>=1.17.0 with no upper bound, and mcp 2.0.0 (released
# 2026-07-28) removed/renamed lowlevel server internals (e.g. request_ctx) that mcp-proxy 0.12.0
# still imports directly, so an unpinned install silently resolves a broken combo.
# TODO: mcp-proxy 0.12.0 (latest as of 2026-08-12, https://pypi.org/project/mcp-proxy/) has not
# been updated for mcp>=2.0 yet -- this pin is temporary, not us capping mcp-proxy itself. Once
# upstream (https://github.com/sparfenyuk/mcp-proxy) ships a release compatible with mcp 2.x,
# drop this pin and let both float again.
RUN pip install mcpo mcp-proxy==0.15.0 "mcp<2.0.0" --break-system-packages

COPY --from=builder /build/bench-mcp /usr/local/bin/bench-mcp
COPY ./bin/benchmcphttp /usr/local/bin/benchmcphttp
RUN chmod +x /usr/local/bin/benchmcphttp

# image metadata (OCI); last so a new version only invalidates this layer
ARG VERSION=local
ARG REVISION=unknown
LABEL org.opencontainers.image.title="bench-mcp" \
      org.opencontainers.image.source="https://github.com/rthomazel/mcp" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${REVISION}"

EXPOSE 8001

CMD ["benchmcphttp"]
