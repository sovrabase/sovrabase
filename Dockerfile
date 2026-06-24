# Stage 1: Build
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /sovrabase ./cmd/sovrabase

# Stage 2: Runtime
FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /sovrabase /sovrabase

EXPOSE 6070
VOLUME ["/data"]
ENV SOVRABASE_DATA_DIR=/data
ENV SOVRABASE_STORAGE_DIR=/data/storage

ENTRYPOINT ["/sovrabase"]
