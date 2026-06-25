# Sovrabase

**The sovereign European BaaS — a lightweight, high-performance alternative to Supabase.**

A single Go binary that provides:
- 📦 **JSON Document Database** (Pebble LSM engine)
- 🔐 **Authentication** (JWT, OAuth with Google & GitHub)
- 📁 **File Storage** (local driver, S3-ready interface)

All in ~40 MB, consuming ~30 MB RAM at idle.

---

## Quick Start

### Docker (recommended)

```bash
# Pull the multi-arch image (amd64 + arm64)
docker pull ghcr.io/ketsuna-org/sovrabase:latest

# Run with a persistent data volume
docker run -d \
  --name sovrabase \
  -p 6070:6070 \
  -v sovrabase-data:/data \
  -e SOVRABASE_JWT_SECRET="$(openssl rand -hex 32)" \
  -e SOVRABASE_ADMIN_EMAIL="admin@example.com" \
  -e SOVRABASE_ADMIN_PASSWORD="your-secure-password" \
  ghcr.io/ketsuna-org/sovrabase:latest
```

Server starts at `http://localhost:6070`. Open `http://localhost:6070/docs` for the full interactive API reference (Redoc).

## API Reference

Full OpenAPI 3.0 documentation is served at **`/docs`** (Redoc) and **`/docs/swagger.json`** (raw spec) on any running instance.

The swagger spec is generated from source annotations via [swaggo/swag](https://github.com/swaggo/swag) and lives in the `docs/` directory. Regenerate with:

```bash
make swagger
```

### Client generation

Use the OpenAPI spec to generate typed clients for any language:

```bash
# TypeScript / JavaScript
npx openapi-typescript-codegen --input http://localhost:6070/docs/swagger.json --output ./client

# Python
openapi-python-client generate --url http://localhost:6070/docs/swagger.json

# Go
oapi-codegen -package client http://localhost:6070/docs/swagger.json > client/client.go

# Dart / Flutter
dart run swagger_dart_code_generator -i http://localhost:6070/docs/swagger.json -o ./lib/client
```

### Quick endpoints

```
POST   /auth/v1/signup                # Create account
POST   /auth/v1/signin                # Login
POST   /auth/v1/refresh               # Refresh tokens
GET    /auth/v1/oauth/{provider}      # OAuth redirect
POST   /auth/v1/magic-link            # Passwordless login
POST   /auth/v1/mfa/setup             # Enable 2FA
POST   /api/v1/collections/{name}     # Insert document
GET    /api/v1/collections/{name}     # List documents
POST   /api/v1/collections/{name}/query  # Query with filter
GET    /api/v1/collections/{name}/{id}   # Get document
PUT    /api/v1/collections/{name}/{id}   # Update document
DELETE /api/v1/collections/{name}/{id}   # Delete document
POST   /api/v1/storage/{bucket}/upload   # Upload file
GET    /api/v1/storage/{bucket}/list     # List files
GET    /api/v1/storage/{bucket}/{path}   # Download file
POST   /api/v1/queues/send            # Enqueue message
POST   /api/v1/queues/receive         # Dequeue messages
POST   /api/v1/events                 # Ingest analytics
```

### docker-compose

Create a `docker-compose.yml`:

```yaml
services:
  sovrabase:
    image: ghcr.io/ketsuna-org/sovrabase:latest
    container_name: sovrabase
    restart: unless-stopped
    ports:
      - "6070:6070"
    volumes:
      - sovrabase-data:/data          # Persistent DB + storage
      - ./config.yaml:/data/config.yaml:ro  # Config file (optional)
    environment:
      SOVRABASE_JWT_SECRET: "change-me-in-production"
      SOVRABASE_ADMIN_EMAIL: "admin@example.com"
      SOVRABASE_ADMIN_PASSWORD: "admin1234"
      SOVRABASE_ENV: "production"
      # SMTP (optional — for email verification / password reset)
      # SOVRABASE_SMTP_HOST: "smtp.example.com"
      # SOVRABASE_SMTP_PORT: "587"
      # SOVRABASE_SMTP_USER: "noreply@example.com"
      # SOVRABASE_SMTP_PASSWORD: "..."
      # SOVRABASE_SMTP_SENDER: "Sovrabase <noreply@example.com>"
      # SOVRABASE_EMAIL_VERIFICATION: "true"
      # S3 storage (optional — offload files to S3/MinIO/R2)
      # SOVRABASE_S3_ENABLED: "true"
      # SOVRABASE_S3_ENDPOINT: "https://s3.example.com"
      # SOVRABASE_S3_ACCESS_KEY: "..."
      # SOVRABASE_S3_SECRET_KEY: "..."
      # SOVRABASE_S3_BUCKET_PREFIX: "sovrabase"

volumes:
  sovrabase-data:
```

```bash
docker-compose up -d
```

#### Configuration priority

Sovrabase resolves config in this order (highest wins):

1. **Environment variables** — `SOVRABASE_*` vars
2. **`config.yaml`** — placed in the data directory (`/data/config.yaml`)
3. **Hard-coded defaults** — works out of the box for local dev

If no `config.yaml` exists at startup, one is auto-created with defaults.

Example `config.yaml`:

```yaml
data_dir: /data
listen_addr: :6070
jwt_secret: "use-openssl-rand-hex-32"
storage_dir: /data/storage
allow_origins: '*'
admin_email: admin@example.com
admin_password: your-secure-password
session_duration: 24h
env: production
rate_limit_per_minute: 100
rate_limit_burst: 20
# SMTP (optional)
# smtp_host: smtp.example.com
# smtp_port: 587
# smtp_user: noreply@example.com
# smtp_password: "..."
# smtp_sender: "Sovrabase <noreply@example.com>"
# email_verification: true
# S3 storage (optional)
# s3_enabled: true
# s3_endpoint: https://s3.example.com
# s3_access_key: "..."
# s3_secret_key: "..."
# s3_bucket_prefix: sovrabase
# s3_use_ssl: true
```

Place it alongside your `docker-compose.yml` and mount it read-only:

#### Volumes explained

| Mount | Purpose | Required |
|---|---|---|
| `sovrabase-data:/data` | PebbleDB database, file storage, auto-generated config | **Yes** |
| `./config.yaml:/data/config.yaml:ro` | Pre-built config file | No (env vars work too) |

#### Production checklist

- [ ] Set a strong `SOVRABASE_JWT_SECRET` (at least 32 random bytes)
- [ ] Change `SOVRABASE_ADMIN_EMAIL` and `SOVRABASE_ADMIN_PASSWORD`
- [ ] Set `SOVRABASE_ENV=production` (disables debug endpoints, warns on weak secrets)
- [ ] Configure SMTP if you need email verification or password reset
- [ ] Use a reverse proxy (nginx/Caddy) with TLS in front of port 6070

### From source

```bash
git clone https://github.com/ketsuna-org/sovrabase.git
cd sovrabase

export SOVRABASE_JWT_SECRET="your-secret-key"
make dev
```

## Why Sovrabase?

| | Supabase | Sovrabase |
|---|---|---|
| **Local setup** | Docker, 3 GB RAM | Single binary, 30 MB RAM |
| **Binary size** | ~2 GB (containers) | ~40 MB |
| **Jurisdiction** | 🇺🇸 US (Cloud Act) | 🇪🇺 EU (GDPR) |
| **Database** | PostgreSQL | Pebble (LSM, embedded) |
| **Replication** | Postgres streaming | WebSocket log streaming |
| **License** | Apache 2.0 | AGPL v3 |

## Roadmap

- [x] Phase 1: Core monolith binary (MVP)
- [ ] Phase 2: High-availability replication
- [ ] Phase 3: Sovereign cloud SaaS
- [ ] Phase 4: Open-core launch

## License

GNU Affero General Public License v3.0
