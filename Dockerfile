ARG APP_VERSION=1.1.1

FROM golang:1.27.2-bookworm AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY VERSION ./
COPY cmd/ ./cmd/
COPY internal/ ./internal/

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    BUILD_VERSION=$(tr -d '[:space:]' < VERSION) && \
    BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ") && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
        -ldflags="-s -w -X 'ubnt-exporter/internal/definitions.VERSION=${BUILD_VERSION}' -X 'ubnt-exporter/internal/definitions.BuildDate=${BUILD_DATE}'" \
        -trimpath \
        -o /build/ubnt-exporter ./cmd/ubnt-exporter

FROM debian:13.7-slim AS runner

ARG APP_VERSION=1.1.1

LABEL org.opencontainers.image.title="ubnt-exporter" \
      org.opencontainers.image.description="Prometheus exporter for Ubiquiti AirMAX (M/AC), LTU, and AirFiber devices" \
      org.opencontainers.image.version="${APP_VERSION}" \
      org.opencontainers.image.licenses="GPL-2.0-only" \
      org.opencontainers.image.base.name="debian:13.7-slim"

ENV GOMEMLIMIT=128MiB

WORKDIR /app

RUN apt-get update && \
    DEBIAN_FRONTEND=noninteractive apt-get upgrade -y && \
    DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
        ca-certificates \
        curl \
        tzdata && \
    rm -rf /var/lib/apt/lists/* && \
    groupadd -g 1000 appgroup && \
    useradd -u 1000 -g appgroup -s /usr/sbin/nologin -M appuser && \
    mkdir -p /app/logs /conf && \
    chown -R appuser:appgroup /app /conf

COPY --from=builder /build/ubnt-exporter /usr/local/bin/ubnt-exporter

EXPOSE 8000

USER appuser

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:8000/metrics || exit 1

STOPSIGNAL SIGTERM

ENTRYPOINT ["ubnt-exporter"]
CMD ["run"]