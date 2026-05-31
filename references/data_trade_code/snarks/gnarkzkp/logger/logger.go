package logger

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	nested "github.com/antonfisher/nested-logrus-formatter"
	"github.com/sirupsen/logrus"
)

var ConsoleLog *logrus.Logger
var FileLog *logrus.Logger

func init() {
	ConsoleLog = logrus.New()
	ConsoleLog.SetFormatter(&nested.Formatter{
		HideKeys:    true,
		FieldsOrder: []string{"component", "category"},
	})

	FileLog = logrus.New()
	FileLog.SetFormatter(&nested.Formatter{
		NoColors:        true,
		HideKeys:        true,
		FieldsOrder:     []string{"func", "category"},
		TimestampFormat: "15:04:05",
	})
	logFile := "abc.log"
	if dir, err := os.Getwd(); err == nil {
		logFile = filepath.Join(dir, logFile)
	}
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0755)
	if err == nil {
		FileLog.SetOutput(file)
	}

	ConsoleLog.SetLevel(logrus.InfoLevel)
	FileLog.SetLevel(logrus.DebugLevel)
}

func Debug(format string, args ...interface{}) {
	FileLog.Debugf(format, args...)
}

func Info(format string, args ...interface{}) {
	ConsoleLog.Infof(format, args...)
	FileLog.Infof(format, args...)
}

func Warn(format string, args ...interface{}) {
	ConsoleLog.Warnf(format, args...)
	FileLog.Warnf(format, args...)
}

func Error(format string, args ...interface{}) error {
	errorMsg := fmt.Sprintf(format, args...)
	ConsoleLog.Error(errorMsg)
	FileLog.Error(errorMsg)
	return errors.New(errorMsg)
}

func Fatal(format string, args ...interface{}) error {
	errorMsg := fmt.Sprintf(format, args...)
	ConsoleLog.Error(errorMsg)
	FileLog.Error(errorMsg)
	panic(errorMsg)
}
