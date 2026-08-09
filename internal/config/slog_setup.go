package config

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

// LogLevel 日志级别
type LogLevel int

var logLevelConvertMap = map[LogLevel]slog.Level{
	LogLevelDebug: slog.LevelDebug,
	LogLevelInfo:  slog.LevelInfo,
	LogLevelWarn:  slog.LevelWarn,
	LogLevelError: slog.LevelError,
}

const (
	LogLevelDebug LogLevel = 1
	LogLevelInfo  LogLevel = 2
	LogLevelWarn  LogLevel = 3
	LogLevelError LogLevel = 4
)

// LogFormat 日志格式
type LogFormat string

const (
	LogFormatText LogFormat = "text" // 文本格式
	LogFormatJSON LogFormat = "json" // JSON 格式
)

// LevelFileHandler 根据日志级别输出到不同文件
type LevelFileHandler struct {
	handlers map[slog.Level]*slog.TextHandler
	files    map[slog.Level]*os.File
	minLevel slog.Level
}

// NewLevelFileHandler 创建按级别分文件的 handler
func NewLevelFileHandler(dir string, opts *slog.HandlerOptions) (*LevelFileHandler, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	levels := map[slog.Level]string{
		slog.LevelDebug: "debug.log",
		slog.LevelInfo:  "info.log",
		slog.LevelWarn:  "warn.log",
		slog.LevelError: "error.log",
	}

	handlers := make(map[slog.Level]*slog.TextHandler)
	files := make(map[slog.Level]*os.File)
	for level, filename := range levels {
		f, err := os.OpenFile(
			filepath.Join(dir, filename),
			os.O_CREATE|os.O_WRONLY|os.O_APPEND,
			0644,
		)
		if err != nil {
			return nil, err
		}
		handlers[level] = slog.NewTextHandler(f, opts)
		files[level] = f
	}

	var minLevel slog.Level
	if l, ok := opts.Level.(slog.Level); ok {
		minLevel = l
	} else {
		minLevel = slog.LevelInfo
	}

	return &LevelFileHandler{
		handlers: handlers,
		files:    files,
		minLevel: minLevel,
	}, nil
}

func (h *LevelFileHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.minLevel
}

func (h *LevelFileHandler) Handle(_ context.Context, record slog.Record) error {
	if handler, ok := h.handlers[record.Level]; ok {
		if err := handler.Handle(context.Background(), record); err != nil {
			return err
		}
	}
	if record.Level == slog.LevelWarn {
		if handler, ok := h.handlers[slog.LevelError]; ok {
			if err := handler.Handle(context.Background(), record); err != nil {
				return err
			}
		}
	}
	return nil
}

func (h *LevelFileHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make(map[slog.Level]*slog.TextHandler)
	for level, handler := range h.handlers {
		handlers[level] = handler.WithAttrs(attrs).(*slog.TextHandler)
	}
	return &LevelFileHandler{handlers: handlers, files: h.files, minLevel: h.minLevel}
}

func (h *LevelFileHandler) WithGroup(name string) slog.Handler {
	handlers := make(map[slog.Level]*slog.TextHandler)
	for level, handler := range h.handlers {
		handlers[level] = handler.WithGroup(name).(*slog.TextHandler)
	}
	return &LevelFileHandler{handlers: handlers, files: h.files, minLevel: h.minLevel}
}

// Close closes all open file descriptors
func (h *LevelFileHandler) Close() error {
	var firstErr error
	for _, f := range h.files {
		if err := f.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// InitLog 初始化 slog 日志
func InitLog(cfg *LogConfig) error {
	if cfg == nil {
		cfg = &LogConfig{
			Level:     2,
			Format:    "text",
			LogDir:    "logs",
			AddSource: true,
			Console:   true,
		}
	}

	opts := &slog.HandlerOptions{
		AddSource: cfg.AddSource,
		Level:     logLevelConvertMap[LogLevel(cfg.Level)],
	}

	var handlers []slog.Handler

	if cfg.Console {
		var handler slog.Handler
		if LogFormat(cfg.Format) == LogFormatJSON {
			handler = slog.NewJSONHandler(os.Stdout, opts)
		} else {
			handler = slog.NewTextHandler(os.Stdout, opts)
		}
		handlers = append(handlers, handler)
	}

	if cfg.LogDir != "" {
		fileHandler, err := NewLevelFileHandler(cfg.LogDir, opts)
		if err != nil {
			return errors.WithStack(err)
		}
		handlers = append(handlers, fileHandler)
	}
	var sg *slog.Logger
	if len(handlers) == 0 {
		sg = slog.New(slog.NewTextHandler(os.Stdout, opts))
	} else if len(handlers) == 1 {
		sg = slog.New(handlers[0])
	} else {
		sg = slog.New(slog.NewMultiHandler(handlers...))
	}
	slog.SetDefault(sg)
	return nil
}
