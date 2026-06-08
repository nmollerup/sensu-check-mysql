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
		query        string
		warning      int
		critical     int
		iniFile      string
		iniContent   string
		iniSection   string
		expectedCode int
		expectError  bool
	}{
		{
			name:         "valid args",
			port:         3306,
			query:        "SELECT 1",
			warning:      10,
			critical:     20,
			expectedCode: sensu.CheckStateOK,
			expectError:  false,
		},
		{
			name:         "missing query",
			port:         3306,
			query:        "",
			warning:      10,
			critical:     20,
			expectedCode: sensu.CheckStateCritical,
			expectError:  true,
		},
		{
			name:         "missing both thresholds",
			port:         3306,
			query:        "SELECT 1",
			warning:      0,
			critical:     0,
			expectedCode: sensu.CheckStateCritical,
			expectError:  true,
		},
		{
			name:         "only warning set is valid",
			port:         3306,
			query:        "SELECT 1",
			warning:      10,
			critical:     0,
			expectedCode: sensu.CheckStateOK,
			expectError:  false,
		},
		{
			name:         "only critical set is valid",
			port:         3306,
			query:        "SELECT 1",
			warning:      0,
			critical:     20,
			expectedCode: sensu.CheckStateOK,
			expectError:  false,
		},
		{
			name:         "port too low",
			port:         1,
			query:        "SELECT 1",
			warning:      10,
			critical:     20,
			expectedCode: sensu.CheckStateCritical,
			expectError:  true,
		},
		{
			name:         "port too high",
			port:         65535,
			query:        "SELECT 1",
			warning:      10,
			critical:     20,
			expectedCode: sensu.CheckStateCritical,
			expectError:  true,
		},
		{
			name:         "nonexistent ini file",
			port:         3306,
			query:        "SELECT 1",
			warning:      10,
			critical:     20,
			iniFile:      "/nonexistent/file.cnf",
			expectedCode: sensu.CheckStateCritical,
			expectError:  true,
		},
		{
			name:    "valid ini file",
			port:    3306,
			query:   "SELECT 1",
			warning: 10,
			critical: 20,
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
			query:   "SELECT 1",
			warning: 10,
			critical: 20,
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
			plugin.Query = tt.query
			plugin.Warning = tt.warning
			plugin.Critical = tt.critical
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

func TestEvaluateCount(t *testing.T) {
	tests := []struct {
		name         string
		count        int
		warning      int
		critical     int
		expectedCode int
		expectError  bool
	}{
		{
			name:         "OK: count below warning",
			count:        5,
			warning:      10,
			critical:     20,
			expectedCode: sensu.CheckStateOK,
			expectError:  false,
		},
		{
			name:         "WARNING: count at warning threshold",
			count:        10,
			warning:      10,
			critical:     20,
			expectedCode: sensu.CheckStateWarning,
			expectError:  true,
		},
		{
			name:         "WARNING: count above warning, below critical",
			count:        15,
			warning:      10,
			critical:     20,
			expectedCode: sensu.CheckStateWarning,
			expectError:  true,
		},
		{
			name:         "CRITICAL: count at critical threshold",
			count:        20,
			warning:      10,
			critical:     20,
			expectedCode: sensu.CheckStateCritical,
			expectError:  true,
		},
		{
			name:         "CRITICAL: count above critical threshold",
			count:        50,
			warning:      10,
			critical:     20,
			expectedCode: sensu.CheckStateCritical,
			expectError:  true,
		},
		{
			name:         "OK: zero count",
			count:        0,
			warning:      10,
			critical:     20,
			expectedCode: sensu.CheckStateOK,
			expectError:  false,
		},
		{
			name:         "WARNING only: count triggers warning",
			count:        5,
			warning:      5,
			critical:     0,
			expectedCode: sensu.CheckStateWarning,
			expectError:  true,
		},
		{
			name:         "CRITICAL only: count triggers critical",
			count:        5,
			warning:      0,
			critical:     5,
			expectedCode: sensu.CheckStateCritical,
			expectError:  true,
		},
		{
			name:         "CRITICAL only: count below critical",
			count:        4,
			warning:      0,
			critical:     5,
			expectedCode: sensu.CheckStateOK,
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugin.Warning = tt.warning
			plugin.Critical = tt.critical

			code, err := evaluateCount(tt.count)

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
	plugin.Query = "SELECT 1"
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
	plugin.Socket = ""
	plugin.Hostname = "localhost"
	plugin.Port = 3306
	plugin.Database = "test"
	plugin.User = "root"
	plugin.Password = "pass"
	plugin.IniFile = ""
	plugin.Query = "SELECT 1"
	plugin.Warning = 10
	plugin.Critical = 20

	event := &corev2.Event{}
	code, err := executeCheck(event)

	if code != sensu.CheckStateCritical {
		t.Errorf("expected critical code for unavailable MySQL, got %d", code)
	}
	if err == nil {
		t.Error("expected error for unavailable MySQL connection")
	}
}
