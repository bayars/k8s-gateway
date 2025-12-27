package logger

import (
	"io"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

var Log *logrus.Logger

// InitLogger initializes the logging system
func InitLogger(logFilePath string, logLevel string) error {
	Log = logrus.New()

	// Set log level
	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		level = logrus.InfoLevel
	}
	Log.SetLevel(level)

	// Create log directory if it doesn't exist
	logDir := filepath.Dir(logFilePath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}

	// Open log file
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return err
	}

	// Write to both file and stdout
	multiWriter := io.MultiWriter(os.Stdout, logFile)
	Log.SetOutput(multiWriter)

	// Set formatter
	Log.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: "2006-01-02 15:04:05",
	})

	Log.Info("Logger initialized successfully")
	return nil
}
