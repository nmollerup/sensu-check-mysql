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
			// Reset plugin config
			plugin.Port = tt.port
			plugin.IniSection = tt.iniSection
			if tt.iniSection == "" {
				plugin.IniSection = "client"
			}

			// Handle ini file setup
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

			// Call checkArgs
			event := &corev2.Event{}
			code, err := checkArgs(event)

			// Verify results
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

func TestCheckArgsEdgeCases(t *testing.T) {
	// Test with empty ini file path (should pass)
	plugin.Port = 3306
	plugin.IniFile = ""
	event := &corev2.Event{}

	code, err := checkArgs(event)
	if code != sensu.CheckStateOK {
		t.Errorf("expected OK with no ini file, got code %d", code)
	}
	if err != nil {
		t.Errorf("expected no error with no ini file, got: %v", err)
	}
}

func TestExecuteCheckWithInvalidIniFile(t *testing.T) {
	// Setup with invalid ini file path
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

func TestExecuteCheckDSNGeneration(t *testing.T) {
	tests := []struct {
		name       string
		socket     string
		hostname   string
		port       uint
		database   string
		user       string
		password   string
		warning    int
		critical   int
		percentage bool
	}{
		{
			name:       "TCP connection with absolute thresholds",
			socket:     "",
			hostname:   "localhost",
			port:       3306,
			database:   "test",
			user:       "root",
			password:   "pass",
			warning:    100,
			critical:   128,
			percentage: false,
		},
		{
			name:       "Unix socket connection with percentage thresholds",
			socket:     "/var/run/mysqld/mysqld.sock",
			hostname:   "localhost",
			port:       3306,
			database:   "test",
			user:       "root",
			password:   "pass",
			warning:    80,
			critical:   90,
			percentage: true,
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
			plugin.Warning = tt.warning
			plugin.Critical = tt.critical
			plugin.Percentage = tt.percentage

			event := &corev2.Event{}

			// This will fail to connect but we're just testing that
			// the function runs without panicking and returns appropriate error
			code, err := executeCheck(event)

			// Should get critical code since MySQL won't be available
			if code != sensu.CheckStateCritical {
				t.Errorf("expected critical code for unavailable MySQL, got %d", code)
			}
			if err == nil {
				t.Error("expected error for unavailable MySQL connection")
			}
		})
	}
}
