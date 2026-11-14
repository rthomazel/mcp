# ── Stage 1: build ──────────────────────────────────────────────────────────────────────────────
FROM golang:1.26.5-alpine AS builder

WORKDIR /build
COPY . .
RUN go mod download

ARG VERSION
RUN CGO_ENABLED=0 go build \
    -ldflags="-s -w -X main.version=${VERSION:-local}" \
    -o investments-mcp ./cmd/investments

# ── Stage 2: runtime ────────────────────────────────────────────────────────────────────────────
FROM debian:trixie-slim

ENV DEBIAN_FRONTEND=noninteractive

# ca-certificates: TLS for database and provider connections
# python3 + pip: mcp-proxy transport
RUN apt-get update && apt-get install -y \
    ca-certificates \
    python3 python3-pip \
    && rm -rf /var/lib/apt/lists/*

RUN pip install mcp-proxy "mcp<2.0.0" --break-system-packages

RUN groupadd -g 9999 investments && useradd -u 9999 -g 9999 -s /sbin/nologin -r investments

COPY --from=builder /build/investments-mcp /usr/local/bin/investments-mcp
COPY ./bin/investmentsmcphttp /usr/local/bin/investmentsmcphttp
RUN chmod +x /usr/local/bin/investmentsmcphttp

USER investments

EXPOSE 8001

CMD ["investmentsmcphttp"]
