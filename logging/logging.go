package logging

import (
	"fmt"
	"log/slog"
	"os"
	"time"
)

// NewLogger creates a new logger that writes to a file.
func NewLogger(env string) *slog.Logger {
	logDir := "logs"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		panic(err)
	}

	logFile := fmt.Sprintf("%s/%s-%s.log", logDir, time.Now().Format("2006-01-02-15-04-05"), env)

	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}

	return slog.New(slog.NewJSONHandler(file, nil))
}
