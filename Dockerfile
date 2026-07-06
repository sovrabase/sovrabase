# Stage 1: Admin dashboard (React / Vite)
FROM node:22-alpine AS frontend

WORKDIR /app
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci --silent

COPY frontend/ ./
RUN npm run build

# Stage 2: Go binary (embeds internal/dashboard/dist)
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=frontend /app/dist ./internal/dashboard/dist

RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /sovrabase ./cmd/sovrabase

# Stage 3: Runtime
FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /sovrabase /sovrabase

EXPOSE 6070
VOLUME ["/data"]
ENV SOVRABASE_DATA_DIR=/data
ENV SOVRABASE_STORAGE_DIR=/data/storage

ENTRYPOINT ["/sovrabase"]
