package main

import (
	"database/sql"
	"fmt"
	"os"

	"github.com/dariubs/percent"
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
	Percentage bool
}

var (
	plugin = Config{
		PluginConfig: sensu.PluginConfig{
			Name:     "check-mysql-connections",
			Short:    "Mysql connections check",
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
			Usage:     "Number of connections upon which we will issue a warning",
			Value:     &plugin.Warning,
			Default:   100,
		},
		&sensu.PluginConfigOption[int]{
			Path:      "critical",
			Argument:  "critical",
			Shorthand: "c",
			Usage:     "Number of connections upon which we will issue an alert",
			Value:     &plugin.Critical,
			Default:   128,
		},
		&sensu.PluginConfigOption[bool]{
			Path:     "percentage",
			Argument: "percentage",
			Usage:    "Use percentage of defined max connections instead of absolute value",
			Value:    &plugin.Percentage,
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

	var maxConnections int
	err = db.QueryRow("SELECT @@GLOBAL.max_connections").Scan(&maxConnections)
	if err != nil {
		return sensu.CheckStateCritical, fmt.Errorf("error fetching max connections: %v", err)
	}

	var usedConnections int
	err = db.QueryRow("SELECT VARIABLE_VALUE FROM performance_schema.global_status WHERE variable_name LIKE 'Threads_connected'").Scan(&usedConnections)
	if err != nil {
		return sensu.CheckStateCritical, fmt.Errorf("error fetching connections: %v", err)
	}

	if plugin.Percentage {
		percentage := percent.PercentOf(usedConnections, maxConnections)
		if percentage >= float64(plugin.Critical) {
			return sensu.CheckStateCritical, fmt.Errorf("max connections reached in MySQL: %d out of %d", usedConnections, maxConnections)
		}
		if percentage >= float64(plugin.Warning) {
			return sensu.CheckStateWarning, fmt.Errorf("max connections reached in MySQL: %d out of %d", usedConnections, maxConnections)
		}
	} else if !plugin.Percentage {
		if usedConnections >= plugin.Critical {
			return sensu.CheckStateCritical, fmt.Errorf("max connections reached in MySQL: %d out of %d", usedConnections, maxConnections)
		}
		if usedConnections >= plugin.Warning {
			return sensu.CheckStateWarning, fmt.Errorf("max connections reached in MySQL: %d out of %d", usedConnections, maxConnections)
		}
	}
	fmt.Printf("max connections is under limit in MySQL: %d out of %d\n", usedConnections, maxConnections)
	return sensu.CheckStateOK, nil
}
