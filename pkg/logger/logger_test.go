package logger

import "testing"

func TestLogFunctionsDoNotPanic(t *testing.T) {
	Info("info", "k", "v")
	Warn("warn", "k", "v")
	Error("error", "k", "v")
}
