package logger

import "testing"

func TestLoggers(t *testing.T) {
	Info("test")
	Warn("test")
	Error("test")
	Debug("test")
	// Fatal("test")
}
