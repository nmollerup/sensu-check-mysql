package main

import (
	"database/sql"
	"fmt"
	"os"

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
	Query      string
	Warning    int
	Critical   int
	CountValue bool
}

var (
	plugin = Config{
		PluginConfig: sensu.PluginConfig{
			Name:     "check-mysql-query-result-count",
			Short:    "MySQL query result count check",
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
		&sensu.PluginConfigOption[string]{
			Path:      "query",
			Argument:  "query",
			Shorthand: "q",
			Usage:     "SQL query to execute",
			Value:     &plugin.Query,
		},
		&sensu.PluginConfigOption[int]{
			Path:      "warning",
			Argument:  "warning",
			Shorthand: "w",
			Usage:     "Warning threshold for the result count",
			Value:     &plugin.Warning,
		},
		&sensu.PluginConfigOption[int]{
			Path:      "critical",
			Argument:  "critical",
			Shorthand: "c",
			Usage:     "Critical threshold for the result count",
			Value:     &plugin.Critical,
		},
		&sensu.PluginConfigOption[bool]{
			Path:     "count-value",
			Argument: "count-value",
			Usage:    "Read the integer value of the first column of the first row instead of counting rows (use for SELECT COUNT(*) queries)",
			Value:    &plugin.CountValue,
			Default:  false,
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
	if plugin.Query == "" {
		return sensu.CheckStateCritical, fmt.Errorf("--query is required")
	}
	if plugin.Warning == 0 && plugin.Critical == 0 {
		return sensu.CheckStateCritical, fmt.Errorf("at least one of --warning or --critical must be set")
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

	count, err := queryCount(db, plugin.Query, plugin.CountValue)
	if err != nil {
		return sensu.CheckStateCritical, fmt.Errorf("error executing query: %v", err)
	}

	return evaluateCount(count)
}

func queryCount(db *sql.DB, query string, countValue bool) (int, error) {
	rows, err := db.Query(query)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	if countValue {
		if !rows.Next() {
			return 0, fmt.Errorf("query returned no rows")
		}
		var count int
		if err := rows.Scan(&count); err != nil {
			return 0, fmt.Errorf("error reading value from first column: %v", err)
		}
		return count, rows.Err()
	}

	count := 0
	for rows.Next() {
		count++
	}
	return count, rows.Err()
}

func evaluateCount(count int) (int, error) {
	if plugin.Critical > 0 && count >= plugin.Critical {
		return sensu.CheckStateCritical, fmt.Errorf("result count %d is above the critical threshold (%d)", count, plugin.Critical)
	}
	if plugin.Warning > 0 && count >= plugin.Warning {
		return sensu.CheckStateWarning, fmt.Errorf("result count %d is above the warning threshold (%d)", count, plugin.Warning)
	}

	fmt.Printf("result count %d is below thresholds\n", count)
	return sensu.CheckStateOK, nil
}
