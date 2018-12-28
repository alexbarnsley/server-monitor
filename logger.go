package main

import (
	"bytes"
	"os"

	"github.com/sirupsen/logrus"
)

type OutputSplitter struct{}

var textFormatter = &logrus.TextFormatter{}

func (splitter *OutputSplitter) Write(p []byte) (n int, err error) {
	if bytes.Contains(p, []byte("[31mERRO")) {
		return os.Stderr.Write(p)
	}
	return os.Stdout.Write(p)
}

func init() {
	logrus.SetFormatter(&logrus.TextFormatter{
		ForceColors:     true,
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})
	logrus.SetOutput(&OutputSplitter{})
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
