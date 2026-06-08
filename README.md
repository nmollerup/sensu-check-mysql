[![Sensu Bonsai Asset](https://img.shields.io/badge/Bonsai-Download%20Me-brightgreen.svg?colorB=89C967&logo=sensu)](https://bonsai.sensu.io/assets/nmollerup/sensu-check-mysql)
[![goreleaser](https://github.com/nmollerup/sensu-check-mysql/actions/workflows/release.yml/badge.svg)](https://github.com/nmollerup/sensu-check-mysql/actions/workflows/release.yml) [![Go Test](https://github.com/nmollerup/sensu-check-mysql/actions/workflows/test.yml/badge.svg)](https://github.com/nmollerup/sensu-check-mysql/actions/workflows/test.yml)
# sensu-check-mysql

A collection of Sensu Go checks for MySQL and MariaDB, converted from the [sensu-plugins-mysql](https://github.com/sensu-plugins/sensu-plugins-mysql) Ruby plugin collection.

## Checks

| Binary | Description |
|--------|-------------|
| `check-mysql-alive` | Verifies the server is reachable and accepting connections |
| `check-mysql-connections` | Monitors `Threads_connected` against warning/critical thresholds (absolute or percentage of `max_connections`) |
| `check-mysql-replication` | Checks replication IO/SQL thread status and lag; auto-detects MySQL vs MariaDB |
| `check-mysql-threads` | Monitors `Threads_running` with both high and low warning/critical thresholds |
| `check-mysql-disk` | Checks total database size from `information_schema` against thresholds in MB |
| `check-mysql-innodb-lock` | Detects InnoDB lock waits exceeding configurable time thresholds |
| `check-mysql-query-result-count` | Runs an arbitrary SQL query and alerts based on the row count or a scalar count value |

## Common flags

All checks share these connection flags:

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--hostname` | | `localhost` | MySQL host |
| `--port` | | `3306` | MySQL port |
| `--socket` | `-s` | | Unix socket (overrides host/port) |
| `--user` | `-u` | | MySQL user |
| `--password` | `-p` | | MySQL password |
| `--ini` | `-i` | | Path to `my.cnf` file; credentials from the ini file override `--user`/`--password` |
| `--ini-section` | | `client` | Section to read from the ini file |
| `--database` | `-d` | `test` | Database to connect to |

## Usage

### check-mysql-alive

Connects to MySQL and returns OK if the ping succeeds.

```
check-mysql-alive --hostname db1.example.com --user sensu --password secret
```

### check-mysql-connections

Alerts when the number of connected threads exceeds thresholds. Use `--percentage` to compare against `max_connections` instead of an absolute count.

```
check-mysql-connections --warning 100 --critical 128
check-mysql-connections --warning 80 --critical 90 --percentage
```

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--warning` | `-w` | `100` | Warning threshold |
| `--critical` | `-c` | `128` | Critical threshold |
| `--percentage` | | `false` | Use percentage of `max_connections` |

### check-mysql-replication

Checks that the IO and SQL replication threads are running and that replication lag is within thresholds. Automatically uses the correct queries for MySQL (via `performance_schema`) and MariaDB (via `SHOW REPLICA STATUS`).

```
check-mysql-replication --warning 900 --critical 1800
```

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--warning` | `-w` | `900` | Warning threshold for replication lag (seconds) |
| `--critical` | `-c` | `1800` | Critical threshold for replication lag (seconds) |
| `--master-connection` | `-m` | | Replication master connection name |

### check-mysql-threads

Monitors `Threads_running`. Supports both high thresholds (too many active threads) and low thresholds (useful for detecting silent failures on replicas).

```
check-mysql-threads --warning 50 --critical 100
check-mysql-threads --warning 50 --critical 100 --warning-low 2 --critical-low 1
```

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--warning` | `-w` | `50` | High warning threshold |
| `--critical` | `-c` | `100` | High critical threshold |
| `--warning-low` | | `0` | Low warning threshold (0 = disabled) |
| `--critical-low` | | `0` | Low critical threshold (0 = disabled) |

### check-mysql-disk

Queries `information_schema.tables` for total database size and alerts when it exceeds thresholds (in MB). Use `--schema` to check a single database; omit to check all schemas.

```
check-mysql-disk --warning 5120 --critical 10240
check-mysql-disk --schema myapp --warning 1024 --critical 2048
```

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--warning` | `-w` | `5120` | Warning threshold in MB |
| `--critical` | `-c` | `10240` | Critical threshold in MB |
| `--schema` | `-S` | | Schema to measure (empty = all schemas) |

### check-mysql-innodb-lock

Detects InnoDB lock waits that have been blocking for longer than the warning threshold. Automatically uses `performance_schema.data_lock_waits` on MySQL 8.0+ and `information_schema.INNODB_LOCK_WAITS` on MySQL 5.x and MariaDB.

```
check-mysql-innodb-lock --warning 5 --critical 10
```

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--warning` | `-w` | `5` | Seconds a lock wait must exceed to be reported |
| `--critical` | `-c` | `10` | Seconds a lock wait must exceed to trigger critical |

### check-mysql-query-result-count

Runs an arbitrary SQL query and compares the result count against thresholds. By default counts the number of rows returned. Use `--count-value` for `SELECT COUNT(*)` queries to read the scalar value instead.

```
# Alert when more than 10 rows match a condition
check-mysql-query-result-count --query "SELECT id FROM orders WHERE status='failed'" \
  --warning 10 --critical 50

# Alert when a COUNT(*) query exceeds thresholds
check-mysql-query-result-count --query "SELECT COUNT(*) FROM jobs WHERE created_at < NOW() - INTERVAL 1 HOUR" \
  --count-value --warning 100 --critical 500
```

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--query` | `-q` | | SQL query to execute (required) |
| `--warning` | `-w` | | Warning threshold (required if `--critical` not set) |
| `--critical` | `-c` | | Critical threshold (required if `--warning` not set) |
| `--count-value` | | `false` | Read the integer value from the first column of the first row instead of counting rows |
