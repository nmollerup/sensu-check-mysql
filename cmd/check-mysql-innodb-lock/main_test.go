package main

import (
	"database/sql"
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

func TestIsMySQL8Plus(t *testing.T) {
	tests := []struct {
		version  string
		expected bool
	}{
		{"8.0.32", true},
		{"8.4.0", true},
		{"9.0.0", true},
		{"5.7.44", false},
		{"5.6.51", false},
		{"10.6.12-MariaDB", false},
		{"10.11.0-MariaDB", false},
		{"8.0.32-MariaDB", false}, // MariaDB with 8.x version string
	}

	for _, tt := range tests {
		got := isMySQL8Plus(tt.version)
		if got != tt.expected {
			t.Errorf("isMySQL8Plus(%q) = %v, want %v", tt.version, got, tt.expected)
		}
	}
}

func TestEvaluateLocks(t *testing.T) {
	nullStr := func(s string) sql.NullString {
		return sql.NullString{String: s, Valid: s != ""}
	}

	tests := []struct {
		name         string
		locks        []lockInfo
		warning      int
		critical     int
		expectedCode int
		expectError  bool
	}{
		{
			name:         "OK: no locks",
			locks:        []lockInfo{},
			warning:      5,
			critical:     10,
			expectedCode: sensu.CheckStateOK,
			expectError:  false,
		},
		{
			name: "WARNING: lock below critical threshold",
			locks: []lockInfo{
				{
					blockingID: "42", requestingID: "43",
					blockingHost: "app1:12345", requestingHost: "app2:12346",
					lockTable: "`mydb`.`orders`", lockIndex: "PRIMARY",
					lockMode: "X", seconds: 7,
					blockingInfo: nullStr("UPDATE orders SET ..."), requestingInfo: nullStr("SELECT ..."),
				},
			},
			warning:      5,
			critical:     10,
			expectedCode: sensu.CheckStateWarning,
			expectError:  true,
		},
		{
			name: "CRITICAL: lock at critical threshold",
			locks: []lockInfo{
				{
					blockingID: "42", requestingID: "43",
					lockTable: "`mydb`.`orders`", lockIndex: "PRIMARY",
					lockMode: "X", seconds: 10,
				},
			},
			warning:      5,
			critical:     10,
			expectedCode: sensu.CheckStateCritical,
			expectError:  true,
		},
		{
			name: "CRITICAL: lock above critical threshold",
			locks: []lockInfo{
				{
					blockingID: "42", requestingID: "43",
					lockTable: "`mydb`.`orders`", lockIndex: "PRIMARY",
					lockMode: "X", seconds: 30,
				},
			},
			warning:      5,
			critical:     10,
			expectedCode: sensu.CheckStateCritical,
			expectError:  true,
		},
		{
			name: "CRITICAL: multiple locks, one exceeds critical",
			locks: []lockInfo{
				{blockingID: "1", requestingID: "2", lockTable: "t1", lockMode: "X", seconds: 7},
				{blockingID: "3", requestingID: "4", lockTable: "t2", lockMode: "S", seconds: 15},
			},
			warning:      5,
			critical:     10,
			expectedCode: sensu.CheckStateCritical,
			expectError:  true,
		},
		{
			name: "WARNING: multiple locks, none exceed critical",
			locks: []lockInfo{
				{blockingID: "1", requestingID: "2", lockTable: "t1", lockMode: "X", seconds: 6},
				{blockingID: "3", requestingID: "4", lockTable: "t2", lockMode: "S", seconds: 8},
			},
			warning:      5,
			critical:     10,
			expectedCode: sensu.CheckStateWarning,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugin.Warning = tt.warning
			plugin.Critical = tt.critical

			code, err := evaluateLocks(tt.locks)

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

func TestFormatLocks(t *testing.T) {
	locks := []lockInfo{
		{
			blockingID: "42", requestingID: "43",
			lockTable: "`mydb`.`orders`", lockIndex: "PRIMARY",
			lockMode: "X", seconds: 7,
		},
	}
	got := formatLocks(locks)
	expected := "[blocking_id=42 requesting_id=43 table=`mydb`.`orders` index=PRIMARY mode=X seconds=7]"
	if got != expected {
		t.Errorf("formatLocks() = %q, want %q", got, expected)
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
	plugin.Socket = ""
	plugin.Hostname = "localhost"
	plugin.Port = 3306
	plugin.Database = "test"
	plugin.User = "root"
	plugin.Password = "pass"
	plugin.IniFile = ""
	plugin.Warning = 5
	plugin.Critical = 10

	event := &corev2.Event{}
	code, err := executeCheck(event)

	if code != sensu.CheckStateCritical {
		t.Errorf("expected critical code for unavailable MySQL, got %d", code)
	}
	if err == nil {
		t.Error("expected error for unavailable MySQL connection")
	}
}
