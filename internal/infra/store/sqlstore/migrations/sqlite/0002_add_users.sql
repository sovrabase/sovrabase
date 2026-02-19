CREATE TABLE IF NOT EXISTS sb_users (
  id TEXT PRIMARY KEY,
  email TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  role TEXT NOT NULL,
  is_root INTEGER NOT NULL DEFAULT 1,
  last_login_at TEXT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_sb_users_single_root
  ON sb_users(is_root)
  WHERE is_root = 1;
