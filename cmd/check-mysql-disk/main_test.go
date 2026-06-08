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

func TestEvaluateSize(t *testing.T) {
	const (
		mb = 1024 * 1024
		gb = 1024 * mb
	)

	tests := []struct {
		name         string
		sizeBytes    int64
		warningMB    int
		criticalMB   int
		expectedCode int
		expectError  bool
	}{
		{
			name:         "OK: size well under warning",
			sizeBytes:    int64(1 * gb),
			warningMB:    5120,
			criticalMB:   10240,
			expectedCode: sensu.CheckStateOK,
			expectError:  false,
		},
		{
			name:         "WARNING: size at warning threshold",
			sizeBytes:    int64(5120 * mb),
			warningMB:    5120,
			criticalMB:   10240,
			expectedCode: sensu.CheckStateWarning,
			expectError:  true,
		},
		{
			name:         "WARNING: size above warning, below critical",
			sizeBytes:    int64(7 * gb),
			warningMB:    5120,
			criticalMB:   10240,
			expectedCode: sensu.CheckStateWarning,
			expectError:  true,
		},
		{
			name:         "CRITICAL: size at critical threshold",
			sizeBytes:    int64(10240 * mb),
			warningMB:    5120,
			criticalMB:   10240,
			expectedCode: sensu.CheckStateCritical,
			expectError:  true,
		},
		{
			name:         "CRITICAL: size above critical threshold",
			sizeBytes:    int64(20 * gb),
			warningMB:    5120,
			criticalMB:   10240,
			expectedCode: sensu.CheckStateCritical,
			expectError:  true,
		},
		{
			name:         "OK: zero byte database",
			sizeBytes:    0,
			warningMB:    5120,
			criticalMB:   10240,
			expectedCode: sensu.CheckStateOK,
			expectError:  false,
		},
		{
			name:         "OK: size just below warning (sub-MB)",
			sizeBytes:    int64(5120*mb) - 1,
			warningMB:    5120,
			criticalMB:   10240,
			expectedCode: sensu.CheckStateOK,
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugin.Warning = tt.warningMB
			plugin.Critical = tt.criticalMB

			code, err := evaluateSize(tt.sizeBytes)

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

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1023, "1023 B"},
		{1024, "1.00 KB"},
		{1536, "1.50 KB"},
		{1024 * 1024, "1.00 MB"},
		{int64(1.5 * 1024 * 1024), "1.50 MB"},
		{1024 * 1024 * 1024, "1.00 GB"},
		{int64(2.5 * 1024 * 1024 * 1024), "2.50 GB"},
	}

	for _, tt := range tests {
		got := formatBytes(tt.input)
		if got != tt.expected {
			t.Errorf("formatBytes(%d) = %q, want %q", tt.input, got, tt.expected)
		}
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
	}{
		{
			name:     "TCP connection to unavailable host",
			socket:   "",
			hostname: "localhost",
			port:     3306,
		},
		{
			name:     "Unix socket connection to nonexistent socket",
			socket:   "/nonexistent/mysqld.sock",
			hostname: "localhost",
			port:     3306,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugin.Socket = tt.socket
			plugin.Hostname = tt.hostname
			plugin.Port = tt.port
			plugin.Database = "test"
			plugin.User = "root"
			plugin.Password = "pass"
			plugin.IniFile = ""
			plugin.Warning = 5120
			plugin.Critical = 10240

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
