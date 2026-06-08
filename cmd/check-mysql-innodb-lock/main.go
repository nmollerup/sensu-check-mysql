package main

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	corev2 "github.com/sensu/sensu-go/api/core/v2"
	"github.com/sensu/sensu-plugin-sdk/sensu"
	"gopkg.in/ini.v1"
)

type Config struct {
	sensu.PluginConfig
	User       string
	Password   string
	IniFile    string
	IniSection string
	Hostname   string
	Port       uint
	Socket     string
	Database   string
	Warning    int
	Critical   int
}

type lockInfo struct {
	blockingID     string
	requestingID   string
	blockingHost   string
	requestingHost string
	lockTable      string
	lockIndex      string
	lockMode       string
	seconds        int
	blockingInfo   sql.NullString
	requestingInfo sql.NullString
}

var (
	plugin = Config{
		PluginConfig: sensu.PluginConfig{
			Name:     "check-mysql-innodb-lock",
			Short:    "MySQL InnoDB lock wait check",
			Keyspace: "",
		},
	}

	options = []sensu.ConfigOption{
		&sensu.PluginConfigOption[string]{
			Path:      "User",
			Argument:  "user",
			Shorthand: "u",
			Usage:     "MySQL user to connect",
			Value:     &plugin.User,
		},
		&sensu.PluginConfigOption[string]{
			Path:      "Password",
			Argument:  "password",
			Shorthand: "p",
			Usage:     "Password for user",
			Value:     &plugin.Password,
		},
		&sensu.PluginConfigOption[string]{
			Path:      "inifile",
			Argument:  "ini",
			Shorthand: "i",
			Usage:     "Location of my.cnf ini file for access to MySQL",
			Value:     &plugin.IniFile,
		},
		&sensu.PluginConfigOption[uint]{
			Path:     "port",
			Argument: "port",
			Usage:    "Port to connect to",
			Value:    &plugin.Port,
			Default:  3306,
		},
		&sensu.PluginConfigOption[string]{
			Path:      "Socket",
			Argument:  "socket",
			Shorthand: "s",
			Usage:     "Socket to use",
			Value:     &plugin.Socket,
		},
		&sensu.PluginConfigOption[string]{
			Path:     "hostname",
			Argument: "hostname",
			Usage:    "Hostname to login to",
			Value:    &plugin.Hostname,
			Default:  "localhost",
		},
		&sensu.PluginConfigOption[string]{
			Path:      "database",
			Argument:  "database",
			Shorthand: "d",
			Usage:     "Database schema to connect to",
			Value:     &plugin.Database,
			Default:   "test",
		},
		&sensu.PluginConfigOption[string]{
			Path:     "ini-section",
			Argument: "ini-section",
			Usage:    "Section to use from my.cnf ini file",
			Value:    &plugin.IniSection,
			Default:  "client",
		},
		&sensu.PluginConfigOption[int]{
			Path:      "warning",
			Argument:  "warning",
			Shorthand: "w",
			Usage:     "Warning threshold: seconds a lock wait must exceed to trigger a warning",
			Value:     &plugin.Warning,
			Default:   5,
		},
		&sensu.PluginConfigOption[int]{
			Path:      "critical",
			Argument:  "critical",
			Shorthand: "c",
			Usage:     "Critical threshold: seconds a lock wait must exceed to trigger a critical alert",
			Value:     &plugin.Critical,
			Default:   10,
		},
	}
)

func main() {
	check := sensu.NewCheck(&plugin.PluginConfig, options, checkArgs, executeCheck, false)
	check.Execute()
}

func checkArgs(event *corev2.Event) (int, error) {
	if plugin.Port <= 1 || plugin.Port >= 65535 {
		return sensu.CheckStateCritical, fmt.Errorf("invalid port, should be a value between 1 and 65535")
	}
	if plugin.IniFile != "" {
		if _, err := os.Stat(plugin.IniFile); os.IsNotExist(err) {
			return sensu.CheckStateCritical, fmt.Errorf("unable to open the supplied config file %s", plugin.IniFile)
		}
		file, err := ini.Load(plugin.IniFile)
		if err != nil {
			return sensu.CheckStateCritical, fmt.Errorf("failed to read inifile")
		}
		if _, err := file.GetSection(plugin.IniSection); err != nil {
			return sensu.CheckStateCritical, fmt.Errorf("unable to read section %s from %s", plugin.IniSection, plugin.IniFile)
		}
	}

	return sensu.CheckStateOK, nil
}

func executeCheck(event *corev2.Event) (int, error) {
	var dataSourceName string

	var dbUser, dbPass string
	if plugin.IniFile != "" {
		iniFile, err := ini.Load(plugin.IniFile)
		if err != nil {
			return sensu.CheckStateCritical, fmt.Errorf("error parsing ini file: %v", err)
		}
		dbUser = iniFile.Section(plugin.IniSection).Key("user").String()
		dbPass = iniFile.Section(plugin.IniSection).Key("password").String()
	} else {
		dbUser = plugin.User
		dbPass = plugin.Password
	}

	if plugin.Socket != "" {
		dataSourceName = fmt.Sprintf("%s:%s@unix(%s)/%s", dbUser, dbPass, plugin.Socket, plugin.Database)
	} else {
		dataSourceName = fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", dbUser, dbPass, plugin.Hostname, plugin.Port, plugin.Database)
	}

	db, err := sql.Open("mysql", dataSourceName)
	if err != nil {
		return sensu.CheckStateCritical, fmt.Errorf("error connecting to MySQL: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	if err := db.Ping(); err != nil {
		return sensu.CheckStateCritical, fmt.Errorf("error pinging MySQL: %v", err)
	}

	var version string
	if err := db.QueryRow("SELECT VERSION()").Scan(&version); err != nil {
		return sensu.CheckStateCritical, fmt.Errorf("error detecting database version: %v", err)
	}

	usePerformanceSchema := isMySQL8Plus(version)

	locks, err := queryLocks(db, usePerformanceSchema)
	if err != nil {
		return sensu.CheckStateCritical, fmt.Errorf("error querying InnoDB locks: %v", err)
	}

	return evaluateLocks(locks)
}

// isMySQL8Plus returns true for MySQL 8.0+ (not MariaDB), which replaced
// information_schema.INNODB_LOCK_WAITS with performance_schema.data_lock_waits.
func isMySQL8Plus(version string) bool {
	if strings.Contains(strings.ToLower(version), "mariadb") {
		return false
	}
	parts := strings.SplitN(version, ".", 2)
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return false
	}
	return major >= 8
}

func queryLocks(db *sql.DB, usePerformanceSchema bool) ([]lockInfo, error) {
	var query string
	if usePerformanceSchema {
		// MySQL 8.0+: INNODB_LOCK_WAITS and INNODB_LOCKS moved to performance_schema
		query = `
			SELECT
				t_b.trx_mysql_thread_id,
				t_w.trx_mysql_thread_id,
				p_b.HOST,
				p_w.HOST,
				CONCAT(dl.OBJECT_SCHEMA, '.', dl.OBJECT_NAME),
				COALESCE(dl.INDEX_NAME, ''),
				dl.LOCK_MODE,
				p_w.TIME,
				p_b.INFO,
				p_w.INFO
			FROM performance_schema.data_lock_waits w
			JOIN performance_schema.data_locks dl ON w.REQUESTING_ENGINE_LOCK_ID = dl.ENGINE_LOCK_ID
			JOIN information_schema.INNODB_TRX t_b ON w.BLOCKING_ENGINE_TRANSACTION_ID = t_b.trx_id
			JOIN information_schema.INNODB_TRX t_w ON w.REQUESTING_ENGINE_TRANSACTION_ID = t_w.trx_id
			JOIN information_schema.PROCESSLIST p_b ON t_b.trx_mysql_thread_id = p_b.ID
			JOIN information_schema.PROCESSLIST p_w ON t_w.trx_mysql_thread_id = p_w.ID
			WHERE p_w.TIME > ?
			ORDER BY t_w.trx_mysql_thread_id, t_b.trx_mysql_thread_id`
	} else {
		// MySQL 5.x / MariaDB
		query = `
			SELECT
				t_b.trx_mysql_thread_id,
				t_w.trx_mysql_thread_id,
				p_b.HOST,
				p_w.HOST,
				l.lock_table,
				COALESCE(l.lock_index, ''),
				l.lock_mode,
				p_w.TIME,
				p_b.INFO,
				p_w.INFO
			FROM information_schema.INNODB_LOCK_WAITS w
			JOIN information_schema.INNODB_LOCKS l ON w.blocking_lock_id = l.lock_id
			JOIN information_schema.INNODB_TRX t_b ON w.blocking_trx_id = t_b.trx_id
			JOIN information_schema.INNODB_TRX t_w ON w.requesting_trx_id = t_w.trx_id
			JOIN information_schema.PROCESSLIST p_b ON t_b.trx_mysql_thread_id = p_b.ID
			JOIN information_schema.PROCESSLIST p_w ON t_w.trx_mysql_thread_id = p_w.ID
			WHERE p_w.TIME > ?
			ORDER BY t_w.trx_mysql_thread_id, t_b.trx_mysql_thread_id`
	}

	rows, err := db.Query(query, plugin.Warning)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var locks []lockInfo
	for rows.Next() {
		var l lockInfo
		if err := rows.Scan(
			&l.blockingID,
			&l.requestingID,
			&l.blockingHost,
			&l.requestingHost,
			&l.lockTable,
			&l.lockIndex,
			&l.lockMode,
			&l.seconds,
			&l.blockingInfo,
			&l.requestingInfo,
		); err != nil {
			return nil, err
		}
		locks = append(locks, l)
	}
	return locks, rows.Err()
}

func evaluateLocks(locks []lockInfo) (int, error) {
	if len(locks) == 0 {
		fmt.Println("InnoDB lock check OK: no blocking locks detected")
		return sensu.CheckStateOK, nil
	}

	isCritical := false
	for _, l := range locks {
		if l.seconds >= plugin.Critical {
			isCritical = true
			break
		}
	}

	msg := formatLocks(locks)
	if isCritical {
		return sensu.CheckStateCritical, fmt.Errorf("detected InnoDB locks: %s", msg)
	}
	return sensu.CheckStateWarning, fmt.Errorf("detected InnoDB locks: %s", msg)
}

func formatLocks(locks []lockInfo) string {
	parts := make([]string, 0, len(locks))
	for _, l := range locks {
		parts = append(parts, fmt.Sprintf(
			"[blocking_id=%s requesting_id=%s table=%s index=%s mode=%s seconds=%d]",
			l.blockingID, l.requestingID, l.lockTable, l.lockIndex, l.lockMode, l.seconds,
		))
	}
	return strings.Join(parts, " ")
}
