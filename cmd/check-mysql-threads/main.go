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
	Warning    int
	Critical   int
	WarnLow    int
	CritLow    int
}

var (
	plugin = Config{
		PluginConfig: sensu.PluginConfig{
			Name:     "check-mysql-threads",
			Short:    "MySQL running threads check",
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
			Usage:     "Warning threshold for running threads (high)",
			Value:     &plugin.Warning,
			Default:   50,
		},
		&sensu.PluginConfigOption[int]{
			Path:      "critical",
			Argument:  "critical",
			Shorthand: "c",
			Usage:     "Critical threshold for running threads (high)",
			Value:     &plugin.Critical,
			Default:   100,
		},
		&sensu.PluginConfigOption[int]{
			Path:     "warning-low",
			Argument: "warning-low",
			Usage:    "Warning threshold for running threads (low); 0 disables low checks",
			Value:    &plugin.WarnLow,
			Default:  0,
		},
		&sensu.PluginConfigOption[int]{
			Path:     "critical-low",
			Argument: "critical-low",
			Usage:    "Critical threshold for running threads (low); 0 disables low checks",
			Value:    &plugin.CritLow,
			Default:  0,
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

	var runningThreads int
	err = db.QueryRow("SELECT VARIABLE_VALUE FROM performance_schema.global_status WHERE variable_name = 'Threads_running'").Scan(&runningThreads)
	if err != nil {
		return sensu.CheckStateCritical, fmt.Errorf("error fetching running threads: %v", err)
	}

	return evaluateThresholds(runningThreads)
}

func evaluateThresholds(runningThreads int) (int, error) {
	if runningThreads >= plugin.Critical {
		return sensu.CheckStateCritical, fmt.Errorf("running threads too high: %d (critical threshold: %d)", runningThreads, plugin.Critical)
	}
	if runningThreads >= plugin.Warning {
		return sensu.CheckStateWarning, fmt.Errorf("running threads high: %d (warning threshold: %d)", runningThreads, plugin.Warning)
	}
	if plugin.CritLow > 0 && runningThreads <= plugin.CritLow {
		return sensu.CheckStateCritical, fmt.Errorf("running threads too low: %d (critical-low threshold: %d)", runningThreads, plugin.CritLow)
	}
	if plugin.WarnLow > 0 && runningThreads <= plugin.WarnLow {
		return sensu.CheckStateWarning, fmt.Errorf("running threads low: %d (warning-low threshold: %d)", runningThreads, plugin.WarnLow)
	}

	fmt.Printf("running threads is OK: %d\n", runningThreads)
	return sensu.CheckStateOK, nil
}
