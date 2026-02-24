ALTER TABLE sb_users ADD COLUMN IF NOT EXISTS account_type TEXT NOT NULL DEFAULT 'admin';
ALTER TABLE sb_users ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT true;

CREATE TABLE IF NOT EXISTS sb_roles (
  id UUID PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  description TEXT NOT NULL DEFAULT '',
  parent_role_id UUID NULL REFERENCES sb_roles(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS sb_scopes (
  id UUID PRIMARY KEY,
  scope_key TEXT NOT NULL UNIQUE,
  description TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS sb_user_roles (
  user_id UUID NOT NULL REFERENCES sb_users(id) ON DELETE CASCADE,
  role_id UUID NOT NULL REFERENCES sb_roles(id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (user_id, role_id)
);

CREATE TABLE IF NOT EXISTS sb_role_scopes (
  role_id UUID NOT NULL REFERENCES sb_roles(id) ON DELETE CASCADE,
  scope_id UUID NOT NULL REFERENCES sb_scopes(id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (role_id, scope_id)
);

INSERT INTO sb_roles (id, name, description, parent_role_id, created_at, updated_at)
VALUES ('00000000-0000-0000-0000-000000000001', 'admin', 'Default admin role', NULL, NOW(), NOW())
ON CONFLICT (name) DO NOTHING;

INSERT INTO sb_roles (id, name, description, parent_role_id, created_at, updated_at)
VALUES ('00000000-0000-0000-0000-000000000002', 'user', 'Default end user role', NULL, NOW(), NOW())
ON CONFLICT (name) DO NOTHING;

INSERT INTO sb_roles (id, name, description, parent_role_id, created_at, updated_at)
VALUES ('00000000-0000-0000-0000-000000000003', 'service', 'Default service role', NULL, NOW(), NOW())
ON CONFLICT (name) DO NOTHING;

UPDATE sb_users
SET account_type = CASE role
  WHEN 'admin' THEN 'admin'
  WHEN 'service' THEN 'service'
  ELSE 'end_user'
END,
    is_active = true;
