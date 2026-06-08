FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w -X main.version=$(git describe --tags --always 2>/dev/null || echo 'dev')" \
    -o /build/server ./cmd/server

FROM alpine:3.19 AS runner

RUN apk add --no-cache ca-certificates tzdata

RUN addgroup -S filestore && adduser -S filestore -G filestore

WORKDIR /app

COPY --from=builder /build/server .
COPY --from=builder /build/src/infrastructure/database/migrations ./migrations

RUN mkdir -p /data/storage /tmp/filestore && \
    chown -R filestore:filestore /data /tmp/filestore /app

USER filestore

EXPOSE 8080

ENV STORAGE_DATA_DIR=/data/storage
ENV STORAGE_TEMP_DIR=/tmp/filestore

HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
    CMD wget -qO- http://localhost:8080/health || exit 1

ENTRYPOINT ["./server"]
