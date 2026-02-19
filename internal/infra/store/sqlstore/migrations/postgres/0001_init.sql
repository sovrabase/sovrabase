CREATE TABLE IF NOT EXISTS sb_schema_migrations (
  version TEXT PRIMARY KEY,
  applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS sb_connections (
  id UUID PRIMARY KEY,
  project_id TEXT NOT NULL,
  slug TEXT NOT NULL,
  display_name TEXT NOT NULL DEFAULT '',
  engine TEXT NOT NULL,
  encrypted_dsn TEXT NOT NULL,
  options_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  managed BOOLEAN NOT NULL DEFAULT false,
  managed_provider TEXT NULL,
  managed_resource_id TEXT NULL,
  status TEXT NOT NULL,
  last_error TEXT NULL,
  last_checked_at TIMESTAMPTZ NULL,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  UNIQUE(project_id, slug)
);

CREATE INDEX IF NOT EXISTS idx_sb_connections_project_id ON sb_connections(project_id);
