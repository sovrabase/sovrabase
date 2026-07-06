# Sovrabase HA Deployment (Phase 2)

This guide describes deploying Sovrabase in **active/passive** high-availability mode:

- **1 Master** — accepts all writes
- **1 Heir** — standby, promotes to Master on failure
- **N Readers** — serve reads and WebSocket realtime

No Redis or NATS is required. State is synchronized via the built-in **WAL WebSocket replication** stream.

## Requirements

| Requirement | Reason |
|-------------|--------|
| **S3-compatible storage** (production) | Local `projects/*/storage/` is not replicated across nodes |
| **Separate `/data` volume per node** | Pebble requires one writer per data directory — do not use NFS for `/data` |
| **Reverse proxy** | Route writes to Master, reads + WebSocket to Readers |

## Node roles

Set via environment variables or `config.yaml`:

```yaml
role: master          # master | heir | reader
node_id: sov-master-1
repl_addr: ":9090"    # WebSocket WAL stream listen address
peers:
  - "master.internal:9090"   # heirs/readers point to master
lease_ttl: 5s
```

| Role | Writes | Reads | WAL stream |
|------|--------|-------|------------|
| `master` | Yes | Yes | Publishes |
| `heir` | No (until promotion) | Yes | Consumes + standby |
| `reader` | No | Yes | Consumes |

## Storage (S3)

In production with `SOVRABASE_ENV=production`, HA mode **requires S3**:

```yaml
s3_enabled: true
s3_endpoint: "https://s3.fr-par.scw.cloud"
s3_access_key: "..."
s3_secret_key: "..."
s3_bucket_prefix: "myapp"
s3_use_ssl: true
```

## Reverse proxy example (nginx)

```nginx
# Writes → Master only
upstream sovrabase_write {
    server master:6070;
}

# Reads + WebSocket → Readers (ip_hash optional for WS stickiness)
upstream sovrabase_read {
    ip_hash;
    server reader1:6070;
    server reader2:6070;
}

server {
    listen 443 ssl;
    server_name api.example.com;

    location / {
        # Route mutations to master
        if ($request_method ~ ^(POST|PUT|PATCH|DELETE)$) {
            proxy_pass http://sovrabase_write;
            break;
        }
        proxy_pass http://sovrabase_read;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
```

## Health and observability

`GET /health` returns replication status when HA is enabled:

```json
{
  "status": "ok",
  "replication": {
    "role": "reader",
    "current_lsn": 42,
    "last_applied_lsn": 40,
    "lag_lsn": 2,
    "master_epoch": 1
  }
}
```

Prometheus metric: `sovrabase_replication_lag_lsn`

Read-only nodes return **503** for mutations with headers:

- `X-Sovrabase-Role: reader`
- `X-Sovrabase-Write-Leader: master.internal:6070`

## Read-your-writes

Readers may lag behind the Master by a few LSN entries. Clients requiring immediate consistency after a write should target the Master (or wait and retry).

## Realtime

WebSocket clients can connect to **any** node (Master or Reader). Events are published via the WAL apply path — no external pub/sub required.

JWT validation uses the **per-project secret** (`X-Project-Key`), not the global admin JWT secret.

## Failover behavior

1. Master stops heartbeating / WAL stream dies
2. Heir detects failure after `3 × lease_ttl`
3. Heir promotes, increments `master_epoch`, starts WAL stream
4. Background jobs (cron, functions, backups) restart on new Master
5. Readers reconnect to new Master's WAL stream

## Docker Compose (3 nodes)

See [`deploy/docker-compose.ha.yml`](../deploy/docker-compose.ha.yml).

```bash
docker compose -f deploy/docker-compose.ha.yml up -d
```

## Out of scope (Phase 2)

- Horizontal multi-writer scaling (requires external pub/sub)
- Sharding by `repl_group`
- Local storage replication
- Managed cloud (Phase 3)
