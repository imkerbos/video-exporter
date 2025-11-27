# MIT License
#
# Copyright (c) 2025 kk
#
# Permission is hereby granted, free of charge, to any person obtaining a copy
# of this software and associated documentation files (the "Software"), to deal
# in the Software without restriction, including without limitation the rights
# to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
# copies of the Software, and to permit persons to whom the Software is
# furnished to do so, subject to the following conditions:
#
# The above copyright notice and this permission notice shall be included in all
# copies or substantial portions of the Software.
#
# THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
# IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
# FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
# AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
# LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
# OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
# SOFTWARE.

# --- Builder stage ---
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install build tools
RUN apk add --no-cache git build-base

# Cache modules
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build static binary (CGO disabled for portability)
ENV CGO_ENABLED=0
RUN go build -o /bin/video-exporter ./

# --- Runtime stage ---
FROM alpine:3.20

WORKDIR /app

# Minimal runtime deps: CA certs and ffmpeg (per project README)
RUN apk add --no-cache ca-certificates ffmpeg

# Copy binary
COPY --from=builder /bin/video-exporter /usr/local/bin/video-exporter

# Non-root user
RUN addgroup -S app && adduser -S app -G app
USER app

# Default config path inside container
ENV CONFIG_FILE=/app/config.yml

EXPOSE 8080

# Healthcheck for the metrics endpoint
HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8080/metrics >/dev/null 2>&1 || exit 1

# Use sh -c so we can pass a custom config path if needed
ENTRYPOINT ["/usr/local/bin/video-exporter"]
CMD []


