-- +migrate Up

CREATE TABLE project_views (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  user_account_id BIGINT NOT NULL,
  name VARCHAR(128) NOT NULL,
  state_json LONGTEXT NOT NULL,
  is_default TINYINT(1) NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  CONSTRAINT fk_project_views_user FOREIGN KEY (user_account_id) REFERENCES user_accounts(id) ON DELETE CASCADE,
  UNIQUE KEY idx_project_views_user_name (user_account_id, name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE INDEX idx_project_views_user_default ON project_views(user_account_id, is_default);

-- +migrate Down

DROP TABLE IF EXISTS project_views;
