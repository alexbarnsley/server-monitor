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
