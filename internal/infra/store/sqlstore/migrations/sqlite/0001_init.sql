CREATE TABLE IF NOT EXISTS sb_schema_migrations (
  version TEXT PRIMARY KEY,
  applied_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS sb_connections (
  id TEXT PRIMARY KEY,
  project_id TEXT NOT NULL,
  slug TEXT NOT NULL,
  display_name TEXT NOT NULL DEFAULT '',
  engine TEXT NOT NULL,
  encrypted_dsn TEXT NOT NULL,
  options_json TEXT NOT NULL DEFAULT '{}',
  managed INTEGER NOT NULL DEFAULT 0,
  managed_provider TEXT NULL,
  managed_resource_id TEXT NULL,
  status TEXT NOT NULL,
  last_error TEXT NULL,
  last_checked_at TEXT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  UNIQUE(project_id, slug)
);

CREATE INDEX IF NOT EXISTS idx_sb_connections_project_id ON sb_connections(project_id);
