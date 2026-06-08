package main

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	corev2 "github.com/sensu/sensu-go/api/core/v2"
	"github.com/sensu/sensu-plugin-sdk/sensu"
	"gopkg.in/ini.v1"
)

type Config struct {
	sensu.PluginConfig
	User             string
	Password         string
	IniFile          string
	IniSection       string
	Hostname         string
	Port             uint
	Socket           string
	Database         string
	Warning          int
	Critical         int
	MasterConnection string
}

var (
	plugin = Config{
		PluginConfig: sensu.PluginConfig{
			Name:     "check-mysql-replication",
			Short:    "Mysql replication status check",
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
			Usage:     "Warning threshold for replication lag",
			Value:     &plugin.Warning,
			Default:   900,
		},
		&sensu.PluginConfigOption[int]{
			Path:      "critical",
			Argument:  "critical",
			Shorthand: "c",
			Usage:     "Critical threshold for replication lag",
			Value:     &plugin.Critical,
			Default:   1800,
		},
		&sensu.PluginConfigOption[string]{
			Path:      "master connection",
			Argument:  "master-connection",
			Shorthand: "m",
			Usage:     "Replication master connection name",
			Value:     &plugin.MasterConnection,
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
	defer func() { if cerr := db.Close(); cerr != nil { err = cerr } }()

	if err := db.Ping(); err != nil {
		return sensu.CheckStateCritical, fmt.Errorf("error connecting to MySQL: %v", err)
	}

	// Detect database type
	var version string
	err = db.QueryRow("SELECT VERSION()").Scan(&version)
	if err != nil {
		return sensu.CheckStateCritical, fmt.Errorf("error detecting database version: %v", err)
	}

	isMariaDB := strings.Contains(strings.ToLower(version), "mariadb")

	if isMariaDB {
		return checkMariaDBReplication(db)
	}
	return checkMySQLReplication(db)
}

func checkMySQLReplication(db *sql.DB) (int, error) {
	// get IO replica status
	var ioThreadStatus string
	err := db.QueryRow("SELECT SERVICE_STATE FROM performance_schema.replication_connection_status").Scan(&ioThreadStatus)
	if err != nil {
		if err == sql.ErrNoRows {
			return sensu.CheckStateCritical, fmt.Errorf("MySQL replication is not configured on this server")
		}
		return sensu.CheckStateCritical, fmt.Errorf("error fetching replica IO thread status: %v", err)
	}
	if ioThreadStatus != "ON" {
		return sensu.CheckStateCritical, fmt.Errorf("replica IO thread not running (status: %s)", ioThreadStatus)
	}

	// get SQL replica status
	var sqlThreadStatus string
	err = db.QueryRow("SELECT SERVICE_STATE FROM performance_schema.replication_applier_status_by_coordinator").Scan(&sqlThreadStatus)
	if err != nil {
		if err == sql.ErrNoRows {
			return sensu.CheckStateCritical, fmt.Errorf("MySQL replication is not configured on this server")
		}
		return sensu.CheckStateCritical, fmt.Errorf("error fetching replica SQL thread status: %v", err)
	}
	if sqlThreadStatus != "ON" {
		return sensu.CheckStateCritical, fmt.Errorf("replica SQL thread not running (status: %s)", sqlThreadStatus)
	}

	// TODO: Check replication lag against warning/critical thresholds

	fmt.Printf("Replication status OK: IO thread and SQL thread are running\n")
	return sensu.CheckStateOK, nil
}

func checkMariaDBReplication(db *sql.DB) (int, error) {
	type ReplicaStatus struct {
		SlaveIORunning  sql.NullString
		SlaveSQLRunning sql.NullString
		SecondsBehind   sql.NullInt64
		LastIOError     sql.NullString
		LastSQLError    sql.NullString
	}

	var status ReplicaStatus

	// Try SHOW REPLICA STATUS first (MariaDB 10.5+), fall back to SHOW SLAVE STATUS
	query := "SHOW REPLICA STATUS"
	rows, err := db.Query(query)
	if err != nil {
		// Try legacy command
		query = "SHOW SLAVE STATUS"
		rows, err = db.Query(query)
		if err != nil {
			return sensu.CheckStateCritical, fmt.Errorf("error executing %s: %v", query, err)
		}
	}
	defer func() { _ = rows.Close() }()

	if !rows.Next() {
		return sensu.CheckStateCritical, fmt.Errorf("replication is not configured on this server")
	}

	// Get column names to find the correct indices
	columns, err := rows.Columns()
	if err != nil {
		return sensu.CheckStateCritical, fmt.Errorf("error reading columns: %v", err)
	}

	// Create a slice to scan all columns
	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	if err := rows.Scan(valuePtrs...); err != nil {
		return sensu.CheckStateCritical, fmt.Errorf("error scanning replication status: %v", err)
	}

	// Map column names to values
	columnMap := make(map[string]interface{})
	for i, col := range columns {
		columnMap[col] = values[i]
	}

	// Extract the values we need
	if val, ok := columnMap["Slave_IO_Running"]; ok {
		if s, ok := val.([]byte); ok {
			status.SlaveIORunning = sql.NullString{String: string(s), Valid: true}
		}
	}
	if val, ok := columnMap["Slave_SQL_Running"]; ok {
		if s, ok := val.([]byte); ok {
			status.SlaveSQLRunning = sql.NullString{String: string(s), Valid: true}
		}
	}
	if val, ok := columnMap["Seconds_Behind_Master"]; ok {
		if val != nil {
			if i, ok := val.(int64); ok {
				status.SecondsBehind = sql.NullInt64{Int64: i, Valid: true}
			}
		}
	}
	if val, ok := columnMap["Last_IO_Error"]; ok {
		if s, ok := val.([]byte); ok {
			status.LastIOError = sql.NullString{String: string(s), Valid: true}
		}
	}
	if val, ok := columnMap["Last_SQL_Error"]; ok {
		if s, ok := val.([]byte); ok {
			status.LastSQLError = sql.NullString{String: string(s), Valid: true}
		}
	}

	// Check IO thread
	if !status.SlaveIORunning.Valid || (status.SlaveIORunning.String != "Yes" && status.SlaveIORunning.String != "Connecting") {
		errorMsg := "replica IO thread not running"
		if status.LastIOError.Valid && status.LastIOError.String != "" {
			errorMsg = fmt.Sprintf("%s: %s", errorMsg, status.LastIOError.String)
		}
		return sensu.CheckStateCritical, fmt.Errorf("%s", errorMsg)
	}

	// Check SQL thread
	if !status.SlaveSQLRunning.Valid || status.SlaveSQLRunning.String != "Yes" {
		errorMsg := "replica SQL thread not running"
		if status.LastSQLError.Valid && status.LastSQLError.String != "" {
			errorMsg = fmt.Sprintf("%s: %s", errorMsg, status.LastSQLError.String)
		}
		return sensu.CheckStateCritical, fmt.Errorf("%s", errorMsg)
	}

	// Check replication lag
	if status.SecondsBehind.Valid {
		secondsBehind := int(status.SecondsBehind.Int64)
		if plugin.Critical > 0 && secondsBehind >= plugin.Critical {
			return sensu.CheckStateCritical, fmt.Errorf("replication lag is %d seconds (critical threshold: %d)", secondsBehind, plugin.Critical)
		}
		if plugin.Warning > 0 && secondsBehind >= plugin.Warning {
			return sensu.CheckStateWarning, fmt.Errorf("replication lag is %d seconds (warning threshold: %d)", secondsBehind, plugin.Warning)
		}
		fmt.Printf("Replication status OK: IO and SQL threads running, lag: %d seconds\n", secondsBehind)
	} else {
		fmt.Printf("Replication status OK: IO and SQL threads running\n")
	}

	return sensu.CheckStateOK, nil
}
