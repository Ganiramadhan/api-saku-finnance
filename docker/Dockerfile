# syntax=docker/dockerfile:1.7

# ──────────────────────────────────────────────────────────────────────────────
# Build stage
# ──────────────────────────────────────────────────────────────────────────────
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates

ENV CGO_ENABLED=0 \
    GOFLAGS=-mod=readonly

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .
ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG VERSION=dev
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath \
      -ldflags="-s -w -X main.version=${VERSION}" \
      -o /out/api ./cmd/api

# ──────────────────────────────────────────────────────────────────────────────
# Runtime stage — distroless, no shell, runs as nonroot.
# ──────────────────────────────────────────────────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot AS runtime

WORKDIR /app

COPY --from=builder --chown=nonroot:nonroot /out/api /app/api
COPY --from=builder --chown=nonroot:nonroot /src/envs ./envs

USER nonroot:nonroot

EXPOSE 4000

ENTRYPOINT ["/app/api"]
