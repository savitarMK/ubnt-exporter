ARG APP_VERSION=2.0.0

FROM --platform=$BUILDPLATFORM golang:1.27.4-bookworm AS builder

ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG GIT_REVISION=unknown
ARG GIT_BRANCH=main

WORKDIR /build

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY VERSION ./
COPY cmd/ ./cmd/
COPY cli/ ./cli/
COPY internal/ ./internal/

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    BUILD_VERSION=$(tr -d '[:space:]' < VERSION) && \
    BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ") && \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
        -tags timetzdata \
        -ldflags="-s -w -X 'ubnt-exporter/internal/definitions.VERSION=${BUILD_VERSION}' -X 'ubnt-exporter/internal/definitions.Revision=${GIT_REVISION}' -X 'ubnt-exporter/internal/definitions.Branch=${GIT_BRANCH}' -X 'ubnt-exporter/internal/definitions.BuildDate=${BUILD_DATE}'" \
        -trimpath \
        -o /build/ubnt-exporter ./cmd/ubnt-exporter

# The runtime image has no shell: prepare writable directories here and copy them with ownership.
RUN mkdir -p /rootfs/app/logs /rootfs/conf

# Distroless static: no shell, no package manager, only CA certificates and /etc/passwd.
# Time zone data is embedded in the binary (-tags timetzdata), so TZ=... keeps working.
# Pinned by multi-arch index digest; Dependabot (docker ecosystem) keeps it current.
FROM gcr.io/distroless/static-debian13@sha256:448810357cc8e67b7208988abdba0f5ec3d3aebee0a5b13eb624979792cffc0f AS runner

ARG APP_VERSION=2.0.0

LABEL org.opencontainers.image.title="ubnt-exporter" \
      org.opencontainers.image.description="Prometheus exporter for Ubiquiti AirMAX (M/AC), LTU, and AirFiber devices" \
      org.opencontainers.image.version="${APP_VERSION}" \
      org.opencontainers.image.licenses="GPL-2.0-only" \
      org.opencontainers.image.base.name="gcr.io/distroless/static-debian13"

ENV GOMEMLIMIT=128MiB \
    GODEBUG=x509negativeserial=1 \
    HOME=/app

COPY --from=builder --chown=1000:1000 /rootfs/ /
COPY --from=builder /build/ubnt-exporter /usr/local/bin/ubnt-exporter

WORKDIR /app

EXPOSE 8000

# Same numeric UID/GID as previous Debian-based images, so mounted log volumes keep working.
USER 1000:1000

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD ["ubnt-exporter", "health"]

STOPSIGNAL SIGTERM

ENTRYPOINT ["ubnt-exporter"]
CMD ["run"]
