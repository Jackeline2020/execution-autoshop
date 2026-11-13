// Package logger fornece o log estruturado (JSON) usado fora do ciclo das
// requisições HTTP, como no processamento das mensagens da Saga.
package logger

import (
	"log/slog"
	"os"
)

var base = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

func Info(msg string, attrs ...any)  { base.Info(msg, attrs...) }
func Warn(msg string, attrs ...any)  { base.Warn(msg, attrs...) }
func Error(msg string, attrs ...any) { base.Error(msg, attrs...) }
