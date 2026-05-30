# ── Stage 1: build ────────────────────────────────────────────────────────────
FROM golang:1.26.3-alpine AS builder

WORKDIR /build
COPY . .
RUN go mod download

ARG VERSION
RUN CGO_ENABLED=0 go build \
    # trim debug info and set version
    -ldflags="-s -w -X main.version=${VERSION:-local}" \
    -o bench-mcp .

# ── Stage 2: runtime ──────────────────────────────────────────────────────────
FROM ubuntu:24.04

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

ARG JJ_VERSION=v0.39.0
RUN ARCH=$(dpkg --print-architecture) && \
    JJ_ARCH=$([ "$ARCH" = "arm64" ] && echo "aarch64" || echo "x86_64") && \
    curl -fsSL "https://github.com/jj-vcs/jj/releases/download/${JJ_VERSION}/jj-${JJ_VERSION}-${JJ_ARCH}-unknown-linux-musl.tar.gz" \
    | tar -xz -C /usr/local/bin ./jj && \
    chmod +x /usr/local/bin/jj

ARG MISE_VERSION=v2026.3.6
RUN ARCH=$(dpkg --print-architecture) && \
    MISE_ARCH=$([ "$ARCH" = "arm64" ] && echo "arm64" || echo "x64") && \
    curl -fsSL "https://github.com/jdx/mise/releases/download/${MISE_VERSION}/mise-${MISE_VERSION}-linux-${MISE_ARCH}" \
    -o /usr/local/bin/mise && \
    chmod +x /usr/local/bin/mise

ENV MISE_DATA_DIR=/mise
ENV MISE_CONFIG_DIR=/mise
ENV PATH="/mise/shims:$PATH"

RUN pip install mcpo mcp-proxy --break-system-packages

COPY --from=builder /build/bench-mcp /usr/local/bin/bench-mcp
COPY ./bin/benchmcphttp /usr/local/bin/benchmcphttp
RUN chmod +x /usr/local/bin/benchmcphttp

EXPOSE 8001

CMD ["benchmcphttp"]
