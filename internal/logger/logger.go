package logger

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
)

type contextKey struct{}

var (
	mu     sync.RWMutex
	global = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
)

// Init configures the global logger.
// format: text|json (default text), level: debug|info|warn|error (default info)
func Init(format, level string) {
	handlerOpts := &slog.HandlerOptions{Level: parseLevel(level)}

	var handler slog.Handler
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "json":
		handler = slog.NewJSONHandler(os.Stderr, handlerOpts)
	default:
		handler = slog.NewTextHandler(os.Stderr, handlerOpts)
	}
	SetLogger(slog.New(handler))
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func SetLogger(l *slog.Logger) {
	if l == nil {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	global = l
}

func getLogger() *slog.Logger {
	mu.RLock()
	defer mu.RUnlock()
	return global
}

func FromContext(ctx context.Context) *slog.Logger {
	if ctx == nil {
		return getLogger()
	}
	if l, ok := ctx.Value(contextKey{}).(*slog.Logger); ok && l != nil {
		return l
	}
	return getLogger()
}

func WithContext(ctx context.Context, args ...any) context.Context {
	return context.WithValue(ctx, contextKey{}, FromContext(ctx).With(args...))
}

func Debug(msg string, args ...any) { getLogger().Debug(msg, args...) }
func Info(msg string, args ...any)  { getLogger().Info(msg, args...) }
func Warn(msg string, args ...any)  { getLogger().Warn(msg, args...) }
func Error(msg string, args ...any) { getLogger().Error(msg, args...) }

// Compatibility helpers for incremental migration from stdlib log.
// Fatal/Fatalf intentionally call os.Exit(1), which skips deferred cleanup.
func Print(v ...any)                 { Info(fmt.Sprint(v...)) }
func Println(v ...any)               { Info(strings.TrimSuffix(fmt.Sprintln(v...), "\n")) }
func Printf(format string, v ...any) { Info(fmt.Sprintf(format, v...)) }
func Fatal(v ...any)                 { Error(fmt.Sprint(v...)); os.Exit(1) }
func Fatalf(format string, v ...any) { Error(fmt.Sprintf(format, v...)); os.Exit(1) }
