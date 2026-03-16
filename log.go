package minato

import "log/slog"

// Logger interface defines the methods required for custom loggin
// within Minato framework.
type Logger interface {
	Info(msg string, args ...any)
	Error(msg string, args ...any)
}

// defaultLogger provides a fallback implementation using the standard log/slog package.
type defaultLogger struct{}

func (l *defaultLogger) Info(msg string, args ...any) {
	slog.Info(msg, args...)
}

func (l *defaultLogger) Error(msg string, args ...any) {
	slog.Error(msg, args...)
}
