package main

import (
	"os"

	"github.com/sirupsen/logrus"
)

func init() {
	logrus.SetFormatter(&logrus.TextFormatter{})
	logrus.SetOutput(os.Stdout)
	logrus.SetLevel(logrus.DebugLevel)
}

func InfoBold(message ...interface{}) {
	message = append([]interface{}{"\x1b[1m"}, message...)
	message = append(message, []interface{}{"\x1b[0m"}...)
	logrus.Info(message...)
}

func Debug(message ...interface{}) {
	logrus.Debug(message...)
}

func Info(message ...interface{}) {
	logrus.Info(message...)
}

func Warn(message ...interface{}) {
	logrus.Warn(message...)
}

func Error(message ...interface{}) {
	logrus.Error(message...)
}

func Fatal(message ...interface{}) {
	logrus.Fatal(message...)
}
