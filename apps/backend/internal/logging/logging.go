package logging

import (
	"log/slog"
	"os"
	"strings"
)

func NewLogger(appEnv string) *slog.Logger {
	level := slog.LevelInfo
	if strings.EqualFold(strings.TrimSpace(appEnv), "development") {
		level = slog.LevelDebug
	}

	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	}))
}
