package sensucheckmysql

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseIni(t *testing.T) {
	tests := []struct {
		name           string
		iniContent     string
		section        string
		expectedUser   string
		expectedPass   string
		expectedSocket string
		expectedHost   string
		expectError    bool
	}{
		{
			name: "valid client section",
			iniContent: `[client]
user = testuser
password = testpass
socket = /var/run/mysqld/mysqld.sock
host = localhost
`,
			section:        "client",
			expectedUser:   "testuser",
			expectedPass:   "testpass",
			expectedSocket: "/var/run/mysqld/mysqld.sock",
			expectedHost:   "localhost",
			expectError:    false,
		},
		{
			name: "custom section",
			iniContent: `[client]
user = defaultuser
password = defaultpass

[custom]
user = customuser
password = custompass
host = db.example.com
`,
			section:        "custom",
			expectedUser:   "customuser",
			expectedPass:   "custompass",
			expectedSocket: "",
			expectedHost:   "db.example.com",
			expectError:    false,
		},
		{
			name: "missing keys return empty strings",
			iniContent: `[minimal]
user = minimaluser
`,
			section:        "minimal",
			expectedUser:   "minimaluser",
			expectedPass:   "",
			expectedSocket: "",
			expectedHost:   "",
			expectError:    false,
		},
		{
			name: "nonexistent section returns empty strings",
			iniContent: `[client]
user = testuser
`,
			section:        "nonexistent",
			expectedUser:   "",
			expectedPass:   "",
			expectedSocket: "",
			expectedHost:   "",
			expectError:    false,
		},
		{
			name:        "invalid ini file",
			iniContent:  "not a valid ini file [[[[",
			section:     "client",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary ini file
			tmpDir := t.TempDir()
			iniFile := filepath.Join(tmpDir, "test.cnf")
			err := os.WriteFile(iniFile, []byte(tt.iniContent), 0644)
			if err != nil {
				t.Fatalf("failed to create test ini file: %v", err)
			}

			// Call ParseIni
			user, pass, socket, host, err := ParseIni(iniFile, tt.section)

			// Check error expectation
			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// Check returned values (only if no error expected)
			if !tt.expectError {
				if user != tt.expectedUser {
					t.Errorf("user: got %q, want %q", user, tt.expectedUser)
				}
				if pass != tt.expectedPass {
					t.Errorf("password: got %q, want %q", pass, tt.expectedPass)
				}
				if socket != tt.expectedSocket {
					t.Errorf("socket: got %q, want %q", socket, tt.expectedSocket)
				}
				if host != tt.expectedHost {
					t.Errorf("host: got %q, want %q", host, tt.expectedHost)
				}
			}
		})
	}
}

func TestParseIniFileNotFound(t *testing.T) {
	user, pass, socket, host, err := ParseIni("/nonexistent/file.cnf", "client")
	
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
	if user != "" || pass != "" || socket != "" || host != "" {
		t.Error("expected empty strings when file not found")
	}
}
