CREATE TABLE IF NOT EXISTS sb_users (
  id UUID PRIMARY KEY,
  email TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  role TEXT NOT NULL,
  is_root BOOLEAN NOT NULL DEFAULT true,
  last_login_at TIMESTAMPTZ NULL,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_sb_users_single_root
  ON sb_users(is_root)
  WHERE is_root = true;
