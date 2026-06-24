# Sovrabase

**The sovereign European BaaS — a lightweight, high-performance alternative to Supabase.**

A single Go binary that provides:
- 📦 **JSON Document Database** (Pebble LSM engine)
- 🔐 **Authentication** (JWT, OAuth with Google & GitHub)
- 📁 **File Storage** (local driver, S3-ready interface)

All in ~40 MB, consuming ~30 MB RAM at idle.

---

## Quick Start

```bash
# Clone
git clone https://github.com/ketsuna-org/sovrabase.git
cd sovrabase

# Configure (optional — defaults work for local dev)
export SOVRABASE_JWT_SECRET="your-secret-key"
export SOVRABASE_LISTEN_ADDR=":6070"

# Build & Run
make dev
```

Server starts at `http://localhost:6070`.

## API Overview

### Auth
```
POST /auth/v1/signup          # Create account
POST /auth/v1/signin          # Login
POST /auth/v1/refresh         # Refresh tokens
GET  /auth/v1/oauth/:provider # OAuth redirect
```

### Database (protected)
```
POST   /api/v1/collections/:name         # Insert document
GET    /api/v1/collections/:name         # List all documents
POST   /api/v1/collections/:name/query   # Query with filter
GET    /api/v1/collections/:name/:id     # Get document
PUT    /api/v1/collections/:name/:id     # Update document
DELETE /api/v1/collections/:name/:id     # Delete document
```

### Storage (protected)
```
POST   /api/v1/storage/:bucket/upload    # Upload file
GET    /api/v1/storage/:bucket/list      # List files
GET    /api/v1/storage/:bucket/:path     # Download file
DELETE /api/v1/storage/:bucket/:path     # Delete file
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
