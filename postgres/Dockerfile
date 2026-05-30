# ── Stage 1: build ──────────────────────────────────────────────────────────────────────────
FROM golang:1.26.3-alpine AS builder

WORKDIR /build
COPY . .
RUN go mod download

ARG VERSION
RUN CGO_ENABLED=0 go build \
    -ldflags="-s -w -X main.version=${VERSION:-local}" \
    -o postgres-mcp .

# ── Stage 2: runtime ─────────────────────────────────────────────────────────────────────────────
FROM debian:bookworm-slim

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y \
    ca-certificates \
    python3 python3-pip \
    && rm -rf /var/lib/apt/lists/*

RUN pip install mcpo mcp-proxy --break-system-packages

COPY --from=builder /build/postgres-mcp /usr/local/bin/postgres-mcp
COPY ./bin/postgresmcphttp /usr/local/bin/postgresmcphttp
RUN chmod +x /usr/local/bin/postgresmcphttp

EXPOSE 8001

CMD ["postgresmcphttp"]
