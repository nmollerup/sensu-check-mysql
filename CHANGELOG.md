# Changelog

All notable changes to this project will be documented in this file.

## [0.1.0] - 2026-06-08

### Added
- `check-mysql-replication`: checks replication IO/SQL thread status and lag with auto-detection of MySQL vs MariaDB
- `check-mysql-threads`: monitors `Threads_running` with configurable high and low warning/critical thresholds
- `check-mysql-disk`: checks total database size from `information_schema` against configurable MB thresholds; optionally scoped to a single schema
- `check-mysql-innodb-lock`: detects InnoDB lock waits exceeding configurable time thresholds; auto-selects `performance_schema.data_lock_waits` on MySQL 8.0+ and `information_schema.INNODB_LOCK_WAITS` on MySQL 5.x and MariaDB
- `check-mysql-query-result-count`: runs an arbitrary SQL query and alerts based on row count or scalar value; replaces both `check-mysql-query-result-count` and `check-mysql-select-count` from the Ruby plugin collection

### Changed
- Updated README with documentation for all checks, flag tables, and usage examples

### Fixed
- Resolved `errcheck` lint errors on deferred `rows.Close()` calls

## [0.0.4] - 2025-12-12

- Dependency updates
