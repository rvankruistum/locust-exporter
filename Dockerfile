# Multi-Stage Dockerfile (Modern Approach)
# No external dependencies required - just `docker build`
# Self-contained, reproducible, and includes version injection

# Stage 1: Build
# Use Debian-based image to avoid Alpine ARM64 emulation issues
FROM golang:1.24-bookworm AS builder

# Install git and ca-certificates (needed for version info and HTTPS)
RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /build

# Copy go module files first (better layer caching)
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build static binary with version info injected
# TARGETARCH is automatically set by Docker buildx for multi-platform builds
ARG TARGETOS=linux
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -a -tags netgo \
    -ldflags="-s -w \
      -X github.com/prometheus/common/version.Version=$(git describe --tags --always --dirty 2>/dev/null || echo 'dev') \
      -X github.com/prometheus/common/version.Revision=$(git rev-parse HEAD 2>/dev/null || echo 'unknown') \
      -X github.com/prometheus/common/version.Branch=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo 'unknown') \
      -X github.com/prometheus/common/version.BuildUser=$(whoami)@$(hostname) \
      -X github.com/prometheus/common/version.BuildDate=$(date -u +'%Y%m%d-%H:%M:%S')" \
    -o locust_exporter .

# Verify the binary works
RUN /build/locust_exporter --version

# Stage 2: Runtime
FROM scratch

# Copy SSL certificates (needed for HTTPS requests to Locust)
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the binary
COPY --from=builder /build/locust_exporter /bin/locust_exporter

EXPOSE 9646
USER 1000

ENTRYPOINT ["/bin/locust_exporter"]
