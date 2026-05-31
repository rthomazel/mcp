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
FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive

RUN pip3 install mcpo mcp-proxy --break-system-packages 2>/dev/null || \
    (apt-get update -qq && apt-get install -y -qq python3-pip && \
     pip3 install mcpo mcp-proxy --break-system-packages)

RUN groupadd -g 9999 keys && useradd -u 9999 -g 9999 -s /sbin/nologin -r keys

COPY --from=builder /build/keys /usr/local/bin/keys
COPY ./bin/keysmcphttp /usr/local/bin/keysmcphttp
RUN chmod +x /usr/local/bin/keysmcphttp

USER keys

EXPOSE 8001

CMD ["keysmcphttp"]
