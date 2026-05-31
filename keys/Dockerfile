# ── Stage 1: build ──────────────────────────────────────────────────────────────────────────────
FROM golang:1.26.3-alpine AS builder

WORKDIR /build
COPY . .
RUN go mod download

ARG VERSION
RUN CGO_ENABLED=0 go build \
    -ldflags="-s -w -X main.version=${VERSION:-local}" \
    -o keys .

# ── Stage 2: runtime ────────────────────────────────────────────────────────────────────────────
FROM debian:trixie-slim

ENV DEBIAN_FRONTEND=noninteractive

# NOTE: rootless Docker consideration — uid 9999 inside the container may not
# have access to Docker Secret files depending on the host uid-mapping config.
# Verify secret file permissions match the effective uid in your environment.
RUN apt-get update -qq && apt-get install -y -qq python3-pip --no-install-recommends \
    && pip3 install mcpo mcp-proxy --break-system-packages \
    && rm -rf /var/lib/apt/lists/*

RUN groupadd -g 9999 keys && useradd -u 9999 -g 9999 -s /sbin/nologin -r keys

COPY --from=builder /build/keys /usr/local/bin/keys
COPY ./bin/keysmcphttp /usr/local/bin/keysmcphttp
RUN chmod +x /usr/local/bin/keysmcphttp

USER keys

EXPOSE 8001

CMD ["keysmcphttp"]
