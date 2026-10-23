# syntax=docker/dockerfile:1
FROM golang:1.22-bookworm AS builder

ARG VERSION=dev
ARG COMMIT=none

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
      -ldflags="-s -w -X mercado/internal/version.Version=${VERSION} -X mercado/internal/version.Commit=${COMMIT}" \
      -o /out/api ./cmd/api \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath \
      -ldflags="-s -w -X mercado/internal/version.Version=${VERSION} -X mercado/internal/version.Commit=${COMMIT}" \
      -o /out/worker ./cmd/worker \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath \
      -ldflags="-s -w -X mercado/internal/version.Version=${VERSION} -X mercado/internal/version.Commit=${COMMIT}" \
      -o /out/migrate ./cmd/migrate

FROM alpine:3.20

RUN addgroup -S mercado && adduser -S -G mercado mercado \
    && apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=builder /out/api /app/api
COPY --from=builder /out/worker /app/worker
COPY --from=builder /out/migrate /app/migrate

USER mercado
EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8080/healthz >/dev/null || exit 1

ENTRYPOINT ["/app/api"]
