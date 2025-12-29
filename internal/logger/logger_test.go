package logger

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitLogger(t *testing.T) {
	// Create temp directory for log file
	tmpDir, err := os.MkdirTemp("", "logger-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	_ = tmpDir // Used in subtests

	tests := []struct {
		name     string
		logLevel string
		wantErr  bool
	}{
		{
			name:     "Valid info level",
			logLevel: "info",
			wantErr:  false,
		},
		{
			name:     "Valid debug level",
			logLevel: "debug",
			wantErr:  false,
		},
		{
			name:     "Valid warn level",
			logLevel: "warn",
			wantErr:  false,
		},
		{
			name:     "Invalid level defaults to info",
			logLevel: "invalid",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testLogPath := filepath.Join(tmpDir, tt.name+".log")
			err := InitLogger(testLogPath, tt.logLevel)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Verify log file was created
			if _, err := os.Stat(testLogPath); os.IsNotExist(err) {
				t.Errorf("Log file was not created at %s", testLogPath)
			}

			// Verify logger is initialized
			if Log == nil {
				t.Error("Logger was not initialized")
			}
		})
	}
}

func TestInitLoggerCreatesDirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "logger-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create path with non-existent subdirectory
	logPath := filepath.Join(tmpDir, "subdir", "nested", "test.log")

	err = InitLogger(logPath, "info")
	if err != nil {
		t.Errorf("Failed to initialize logger with nested directory: %v", err)
	}

	// Verify directory was created
	logDir := filepath.Dir(logPath)
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		t.Errorf("Log directory was not created at %s", logDir)
	}
}

func TestLoggerOutput(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "logger-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	logPath := filepath.Join(tmpDir, "output.log")
	err = InitLogger(logPath, "debug")
	if err != nil {
		t.Fatal(err)
	}

	// Write a log message
	Log.Info("Test message")
	Log.WithField("key", "value").Debug("Debug message")

	// Read log file and verify content
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}

	if len(content) == 0 {
		t.Error("Log file is empty")
	}
}
