# ── Stage 1: build ──────────────────────────────────────────────────────────────────────────
FROM golang:1.27.4-alpine AS builder

WORKDIR /build
COPY . .
RUN go mod download

ARG VERSION
RUN CGO_ENABLED=0 go build \
    -ldflags="-s -w -X main.version=${VERSION:-local}" \
    -o postgres-mcp .

# ── Stage 2: runtime ────────────────────────────────────────────────────────────────────────────
FROM debian:trixie-slim

ENV DEBIAN_FRONTEND=noninteractive

# ca-certificates: TLS for database connections
# python3 + pip: mcpo and mcp-proxy transports
RUN apt-get update && apt-get install -y \
    ca-certificates \
    python3 python3-pip \
    && rm -rf /var/lib/apt/lists/*

RUN pip install mcpo mcp-proxy==0.15.0 "mcp<2.0.0" --break-system-packages

COPY --from=builder /build/postgres-mcp /usr/local/bin/postgres-mcp
COPY ./bin/postgresmcphttp /usr/local/bin/postgresmcphttp
RUN chmod +x /usr/local/bin/postgresmcphttp

# image metadata (OCI); last so a new version only invalidates this layer
ARG VERSION=local
ARG REVISION=unknown
LABEL org.opencontainers.image.title="postgres-mcp" \
      org.opencontainers.image.source="https://github.com/rthomazel/mcp" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${REVISION}"

EXPOSE 8001

CMD ["postgresmcphttp"]
