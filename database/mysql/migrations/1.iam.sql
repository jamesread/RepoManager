-- +migrate Up

CREATE TABLE user_accounts (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  username VARCHAR(255) NOT NULL,
  password_hash TEXT NOT NULL,
  created_by VARCHAR(64) NOT NULL DEFAULT 'admin-created',
  UNIQUE KEY idx_user_accounts_username (username)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE sessions (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  sid VARCHAR(255) NOT NULL,
  user_account_id BIGINT NOT NULL,
  impersonator_user_id BIGINT NULL,
  UNIQUE KEY idx_sessions_sid (sid),
  KEY idx_sessions_user (user_account_id),
  CONSTRAINT fk_sessions_user FOREIGN KEY (user_account_id) REFERENCES user_accounts(id) ON DELETE CASCADE,
  CONSTRAINT fk_sessions_impersonator FOREIGN KEY (impersonator_user_id) REFERENCES user_accounts(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE api_keys (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  user_account_id BIGINT NOT NULL,
  name VARCHAR(255) NOT NULL DEFAULT '',
  key_value VARCHAR(255) NOT NULL,
  read_only TINYINT NOT NULL DEFAULT 0,
  last_used_at DATETIME NULL,
  UNIQUE KEY idx_api_keys_key_value (key_value),
  KEY idx_api_keys_user (user_account_id),
  CONSTRAINT fk_api_keys_user FOREIGN KEY (user_account_id) REFERENCES user_accounts(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE user_groups (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  name VARCHAR(255) NOT NULL,
  UNIQUE KEY idx_user_groups_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE user_group_memberships (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  user_account_id BIGINT NOT NULL,
  user_group_id BIGINT NOT NULL,
  UNIQUE KEY idx_user_group_membership (user_account_id, user_group_id),
  KEY idx_ugm_group (user_group_id),
  CONSTRAINT fk_ugm_user FOREIGN KEY (user_account_id) REFERENCES user_accounts(id) ON DELETE CASCADE,
  CONSTRAINT fk_ugm_group FOREIGN KEY (user_group_id) REFERENCES user_groups(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE rbac_permissions (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  name VARCHAR(255) NOT NULL,
  description TEXT NOT NULL,
  UNIQUE KEY idx_rbac_permissions_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE rbac_roles (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  name VARCHAR(255) NOT NULL,
  description TEXT NOT NULL,
  UNIQUE KEY idx_rbac_roles_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE rbac_role_permissions (
  role_id BIGINT NOT NULL,
  permission_id BIGINT NOT NULL,
  PRIMARY KEY (role_id, permission_id),
  KEY idx_rrp_perm (permission_id),
  CONSTRAINT fk_rrp_role FOREIGN KEY (role_id) REFERENCES rbac_roles(id) ON DELETE CASCADE,
  CONSTRAINT fk_rrp_perm FOREIGN KEY (permission_id) REFERENCES rbac_permissions(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE rbac_group_roles (
  user_group_id BIGINT NOT NULL,
  role_id BIGINT NOT NULL,
  PRIMARY KEY (user_group_id, role_id),
  KEY idx_rgr_role (role_id),
  CONSTRAINT fk_rgr_group FOREIGN KEY (user_group_id) REFERENCES user_groups(id) ON DELETE CASCADE,
  CONSTRAINT fk_rgr_role FOREIGN KEY (role_id) REFERENCES rbac_roles(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT INTO rbac_permissions (name, description) VALUES
('app.access', 'Use the application (non-admin APIs)'),
('users.view', 'List and view user accounts'),
('users.create', 'Create user accounts'),
('users.delete', 'Delete user accounts'),
('users.reset-password', 'Reset passwords for other users'),
('usergroups.view', 'List user groups and view membership'),
('usergroups.manage', 'Create and delete user groups; manage membership'),
('rbac.view', 'View roles and permissions'),
('rbac.manage', 'Create, update, and delete roles; assign roles to groups'),
('system.settings', 'View and modify system settings'),
('system.logs', 'View logs, job status, and run maintenance'),
('system.impersonate', 'Impersonate other users');

INSERT INTO rbac_roles (name, description) VALUES
('superuser', 'All permissions (system role)'),
('member', 'Standard application access');

INSERT INTO rbac_role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM rbac_roles r CROSS JOIN rbac_permissions p WHERE r.name = 'superuser';

INSERT INTO rbac_role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM rbac_roles r JOIN rbac_permissions p ON p.name = 'app.access' WHERE r.name = 'member';

INSERT INTO user_groups (name) VALUES ('Everyone'), ('Administrators');

INSERT INTO rbac_group_roles (user_group_id, role_id)
SELECT g.id, r.id FROM user_groups g CROSS JOIN rbac_roles r
WHERE g.name = 'Everyone' AND r.name = 'member';

INSERT INTO rbac_group_roles (user_group_id, role_id)
SELECT g.id, r.id FROM user_groups g CROSS JOIN rbac_roles r
WHERE g.name = 'Administrators' AND r.name = 'superuser';

-- +migrate Down

DROP TABLE IF EXISTS rbac_group_roles;
DROP TABLE IF EXISTS rbac_role_permissions;
DROP TABLE IF EXISTS rbac_roles;
DROP TABLE IF EXISTS rbac_permissions;
DROP TABLE IF EXISTS user_group_memberships;
DROP TABLE IF EXISTS user_groups;
DROP TABLE IF EXISTS api_keys;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS user_accounts;
