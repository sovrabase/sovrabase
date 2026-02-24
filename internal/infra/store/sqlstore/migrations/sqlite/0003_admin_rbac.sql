ALTER TABLE sb_users ADD COLUMN account_type TEXT NOT NULL DEFAULT 'admin';
ALTER TABLE sb_users ADD COLUMN is_active INTEGER NOT NULL DEFAULT 1;

CREATE TABLE IF NOT EXISTS sb_roles (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  description TEXT NOT NULL DEFAULT '',
  parent_role_id TEXT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  FOREIGN KEY (parent_role_id) REFERENCES sb_roles(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS sb_scopes (
  id TEXT PRIMARY KEY,
  scope_key TEXT NOT NULL UNIQUE,
  description TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS sb_user_roles (
  user_id TEXT NOT NULL,
  role_id TEXT NOT NULL,
  created_at TEXT NOT NULL,
  PRIMARY KEY (user_id, role_id),
  FOREIGN KEY (user_id) REFERENCES sb_users(id) ON DELETE CASCADE,
  FOREIGN KEY (role_id) REFERENCES sb_roles(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS sb_role_scopes (
  role_id TEXT NOT NULL,
  scope_id TEXT NOT NULL,
  created_at TEXT NOT NULL,
  PRIMARY KEY (role_id, scope_id),
  FOREIGN KEY (role_id) REFERENCES sb_roles(id) ON DELETE CASCADE,
  FOREIGN KEY (scope_id) REFERENCES sb_scopes(id) ON DELETE CASCADE
);

INSERT OR IGNORE INTO sb_roles (id, name, description, parent_role_id, created_at, updated_at)
VALUES ('00000000-0000-0000-0000-000000000001', 'admin', 'Default admin role', NULL, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);

INSERT OR IGNORE INTO sb_roles (id, name, description, parent_role_id, created_at, updated_at)
VALUES ('00000000-0000-0000-0000-000000000002', 'user', 'Default end user role', NULL, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);

INSERT OR IGNORE INTO sb_roles (id, name, description, parent_role_id, created_at, updated_at)
VALUES ('00000000-0000-0000-0000-000000000003', 'service', 'Default service role', NULL, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);

UPDATE sb_users
SET account_type = CASE role
  WHEN 'admin' THEN 'admin'
  WHEN 'service' THEN 'service'
  ELSE 'end_user'
END,
    is_active = 1;
