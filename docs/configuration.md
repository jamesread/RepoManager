# Configuration

The web service reads `config.yaml` from `-configdir` (default development: `var/config/`).

| Key | Description |
|-----|-------------|
| `listen_port` | HTTP port (overridden by `$PORT`) |
| `site_title` | Shown in Init / header |
| `database_driver` | Optional: `sqlite3` or `mysql`. Inferred from `database_url` when omitted (`file:` → SQLite). |
| `database_url` | Database DSN for IAM and settings cvars |
| `auth` | httpauthshim configuration block |

Environment overrides use the `REPOMANAGER_` prefix (for example `REPOMANAGER_DATABASE_URL`).

### SQLite (default)

```yaml
database_url: file:tmp/repomanager.db?_foreign_keys=on
```

### MySQL

```yaml
database_driver: mysql
database_url: repomanager:secret@tcp(127.0.0.1:3306)/repomanager?parseTime=true
```

Create the database and user first, then apply migrations:

```bash
make migrate DB_DRIVER=mysql
```

Apply database migrations before starting:

```bash
make migrate
```

The **System Control Panel → Database** page (requires `system.logs`) shows connection status, driver, host, schema version, and applied migrations.
