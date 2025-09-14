# syntax=docker/dockerfile:1
# ---------- Build stage ----------
FROM golang:1.25-bookworm AS build
WORKDIR /src

# Cache deps
COPY go.mod ./
# If you have go.sum, copy it too (optional)
# COPY go.sum ./
RUN go mod download

# Copy the whole repo
COPY . .

# Build static linux binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/events-api ./cmd/events-api

# ---------- Runtime stage ----------
FROM alpine:3.20
# busybox wget is available; used by HEALTHCHECK
RUN adduser -D -u 10001 appuser
WORKDIR /app

COPY --from=build /out/events-api /app/events-api
COPY api /app/api
COPY migrations /app/migrations

USER appuser
EXPOSE 8080

# Basic container health (uses /healthz)
HEALTHCHECK --interval=10s --timeout=3s --retries=5 CMD wget -qO- http://127.0.0.1:8080/healthz || exit 1

ENTRYPOINT ["/app/events-api"]
