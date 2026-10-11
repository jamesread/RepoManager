-- +migrate Up

CREATE TABLE cvars (
  cvar_key VARCHAR(255) NOT NULL PRIMARY KEY,
  cvar_value_int INT NULL,
  cvar_value_string TEXT NULL,
  cvar_main_type VARCHAR(32) NOT NULL,
  cvar_title VARCHAR(255) NOT NULL DEFAULT '',
  cvar_description TEXT NOT NULL,
  cvar_category VARCHAR(255) NOT NULL DEFAULT '',
  cvar_ordinal INT NOT NULL DEFAULT 0
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +migrate Down

DROP TABLE IF EXISTS cvars;
