# syntax=docker/dockerfile:1
# Copyright 2024-2025 KrakLabs
#
# Licensed under the GNU Affero General Public License, Version 3.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# SPDX-License-Identifier: AGPL-3.0-or-later


# ==============================================================================
# CIE - Code Intelligence Engine
# Multi-stage Dockerfile for optimized production image
# ==============================================================================

# ==============================================================================
# Build Stage
# ==============================================================================
FROM golang:1.24-bookworm AS builder

# Install build dependencies for CGO (CozoDB, tree-sitter)
RUN apt-get update && apt-get install -y --no-install-recommends \
    gcc \
    g++ \
    libc6-dev \
    wget \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Download CozoDB C library (glibc version)
ARG COZO_VERSION=0.7.6
RUN ARCH=$(uname -m) && \
    if [ "$ARCH" = "x86_64" ]; then \
        COZO_PLATFORM="x86_64-unknown-linux-gnu"; \
    elif [ "$ARCH" = "aarch64" ]; then \
        COZO_PLATFORM="aarch64-unknown-linux-gnu"; \
    else \
        echo "Unsupported architecture: $ARCH" && exit 1; \
    fi && \
    wget -q "https://github.com/cozodb/cozo/releases/download/v${COZO_VERSION}/libcozo_c-${COZO_VERSION}-${COZO_PLATFORM}.a.gz" && \
    gunzip libcozo_c-${COZO_VERSION}-${COZO_PLATFORM}.a.gz && \
    mv libcozo_c-${COZO_VERSION}-${COZO_PLATFORM}.a /usr/local/lib/libcozo_c.a && \
    ldconfig

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build arguments
ARG VERSION=dev
ARG COMMIT=unknown
ARG DATE=unknown

# Build with CGO enabled, strip debug symbols
RUN CGO_ENABLED=1 go build \
    -ldflags "-X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE} -s -w" \
    -o /app/cie \
    ./cmd/cie

# ==============================================================================
# Runtime Stage
# ==============================================================================
FROM gcr.io/distroless/cc-debian12:nonroot

# Labels
LABEL org.opencontainers.image.title="CIE - Code Intelligence Engine"
LABEL org.opencontainers.image.description="Code intelligence through semantic search and call graph analysis"
LABEL org.opencontainers.image.url="https://github.com/kraklabs/cie"
LABEL org.opencontainers.image.source="https://github.com/kraklabs/cie"
LABEL org.opencontainers.image.vendor="KrakLabs"
LABEL org.opencontainers.image.licenses="AGPL-3.0-or-later"

# Copy binary from builder
COPY --from=builder /app/cie /usr/local/bin/cie

# Distroless already runs as nonroot user (uid=65532)
# Distroless already has ca-certificates and tzdata
WORKDIR /repo

# Expose no ports by default (CIE is CLI-based)
# MCP mode uses stdio, not network ports

ENTRYPOINT ["cie"]
CMD ["--help"]
