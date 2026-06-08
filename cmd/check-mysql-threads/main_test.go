package main

import (
	"os"
	"path/filepath"
	"testing"

	corev2 "github.com/sensu/sensu-go/api/core/v2"
	"github.com/sensu/sensu-plugin-sdk/sensu"
)

func TestCheckArgs(t *testing.T) {
	tests := []struct {
		name         string
		port         uint
		iniFile      string
		iniContent   string
		iniSection   string
		expectedCode int
		expectError  bool
	}{
		{
			name:         "valid port",
			port:         3306,
			iniFile:      "",
			expectedCode: sensu.CheckStateOK,
			expectError:  false,
		},
		{
			name:         "port too low",
			port:         1,
			iniFile:      "",
			expectedCode: sensu.CheckStateCritical,
			expectError:  true,
		},
		{
			name:         "port too high",
			port:         65535,
			iniFile:      "",
			expectedCode: sensu.CheckStateCritical,
			expectError:  true,
		},
		{
			name:         "port at minimum valid",
			port:         2,
			iniFile:      "",
			expectedCode: sensu.CheckStateOK,
			expectError:  false,
		},
		{
			name:         "port at maximum valid",
			port:         65534,
			iniFile:      "",
			expectedCode: sensu.CheckStateOK,
			expectError:  false,
		},
		{
			name:         "nonexistent ini file",
			port:         3306,
			iniFile:      "/nonexistent/file.cnf",
			expectedCode: sensu.CheckStateCritical,
			expectError:  true,
		},
		{
			name:    "valid ini file with client section",
			port:    3306,
			iniFile: "temp",
			iniContent: `[client]
user = testuser
password = testpass
`,
			iniSection:   "client",
			expectedCode: sensu.CheckStateOK,
			expectError:  false,
		},
		{
			name:    "ini file with missing section",
			port:    3306,
			iniFile: "temp",
			iniContent: `[client]
user = testuser
`,
			iniSection:   "nonexistent",
			expectedCode: sensu.CheckStateCritical,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugin.Port = tt.port
			plugin.IniSection = tt.iniSection
			if tt.iniSection == "" {
				plugin.IniSection = "client"
			}

			if tt.iniFile == "temp" {
				tmpDir := t.TempDir()
				tmpFile := filepath.Join(tmpDir, "test.cnf")
				err := os.WriteFile(tmpFile, []byte(tt.iniContent), 0644)
				if err != nil {
					t.Fatalf("failed to create temp ini file: %v", err)
				}
				plugin.IniFile = tmpFile
			} else {
				plugin.IniFile = tt.iniFile
			}

			event := &corev2.Event{}
			code, err := checkArgs(event)

			if code != tt.expectedCode {
				t.Errorf("expected code %d, got %d", tt.expectedCode, code)
			}
			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestEvaluateThresholds(t *testing.T) {
	tests := []struct {
		name           string
		runningThreads int
		warning        int
		critical       int
		warnLow        int
		critLow        int
		expectedCode   int
		expectError    bool
	}{
		{
			name:           "OK: threads within range",
			runningThreads: 10,
			warning:        50,
			critical:       100,
			warnLow:        0,
			critLow:        0,
			expectedCode:   sensu.CheckStateOK,
			expectError:    false,
		},
		{
			name:           "WARNING: threads at warning threshold",
			runningThreads: 50,
			warning:        50,
			critical:       100,
			warnLow:        0,
			critLow:        0,
			expectedCode:   sensu.CheckStateWarning,
			expectError:    true,
		},
		{
			name:           "WARNING: threads above warning threshold",
			runningThreads: 75,
			warning:        50,
			critical:       100,
			warnLow:        0,
			critLow:        0,
			expectedCode:   sensu.CheckStateWarning,
			expectError:    true,
		},
		{
			name:           "CRITICAL: threads at critical threshold",
			runningThreads: 100,
			warning:        50,
			critical:       100,
			warnLow:        0,
			critLow:        0,
			expectedCode:   sensu.CheckStateCritical,
			expectError:    true,
		},
		{
			name:           "CRITICAL: threads above critical threshold",
			runningThreads: 200,
			warning:        50,
			critical:       100,
			warnLow:        0,
			critLow:        0,
			expectedCode:   sensu.CheckStateCritical,
			expectError:    true,
		},
		{
			name:           "WARNING-LOW: threads at warning-low threshold",
			runningThreads: 5,
			warning:        50,
			critical:       100,
			warnLow:        5,
			critLow:        2,
			expectedCode:   sensu.CheckStateWarning,
			expectError:    true,
		},
		{
			name:           "CRITICAL-LOW: threads at critical-low threshold",
			runningThreads: 2,
			warning:        50,
			critical:       100,
			warnLow:        5,
			critLow:        2,
			expectedCode:   sensu.CheckStateCritical,
			expectError:    true,
		},
		{
			name:           "CRITICAL-LOW: threads below critical-low threshold",
			runningThreads: 1,
			warning:        50,
			critical:       100,
			warnLow:        5,
			critLow:        2,
			expectedCode:   sensu.CheckStateCritical,
			expectError:    true,
		},
		{
			name:           "OK: low thresholds disabled, threads low",
			runningThreads: 1,
			warning:        50,
			critical:       100,
			warnLow:        0,
			critLow:        0,
			expectedCode:   sensu.CheckStateOK,
			expectError:    false,
		},
		{
			name:           "high check takes priority over low check",
			runningThreads: 200,
			warning:        50,
			critical:       100,
			warnLow:        5,
			critLow:        2,
			expectedCode:   sensu.CheckStateCritical,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugin.Warning = tt.warning
			plugin.Critical = tt.critical
			plugin.WarnLow = tt.warnLow
			plugin.CritLow = tt.critLow

			code, err := evaluateThresholds(tt.runningThreads)

			if code != tt.expectedCode {
				t.Errorf("expected code %d, got %d", tt.expectedCode, code)
			}
			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestExecuteCheckWithInvalidIniFile(t *testing.T) {
	plugin.IniFile = "/nonexistent/file.cnf"
	plugin.IniSection = "client"
	event := &corev2.Event{}

	code, err := executeCheck(event)

	if code != sensu.CheckStateCritical {
		t.Errorf("expected critical code, got %d", code)
	}
	if err == nil {
		t.Error("expected error for nonexistent ini file")
	}
}

func TestExecuteCheckConnectionFailure(t *testing.T) {
	tests := []struct {
		name     string
		socket   string
		hostname string
		port     uint
		database string
		user     string
		password string
	}{
		{
			name:     "TCP connection to unavailable host",
			socket:   "",
			hostname: "localhost",
			port:     3306,
			database: "test",
			user:     "root",
			password: "pass",
		},
		{
			name:     "Unix socket connection to nonexistent socket",
			socket:   "/nonexistent/mysqld.sock",
			hostname: "localhost",
			port:     3306,
			database: "test",
			user:     "root",
			password: "pass",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugin.Socket = tt.socket
			plugin.Hostname = tt.hostname
			plugin.Port = tt.port
			plugin.Database = tt.database
			plugin.User = tt.user
			plugin.Password = tt.password
			plugin.IniFile = ""
			plugin.Warning = 50
			plugin.Critical = 100
			plugin.WarnLow = 0
			plugin.CritLow = 0

			event := &corev2.Event{}
			code, err := executeCheck(event)

			if code != sensu.CheckStateCritical {
				t.Errorf("expected critical code for unavailable MySQL, got %d", code)
			}
			if err == nil {
				t.Error("expected error for unavailable MySQL connection")
			}
		})
	}
}
